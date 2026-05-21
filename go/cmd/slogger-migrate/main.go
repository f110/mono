// Command slogger-migrate rewrites Go code from the zap-based
// go.f110.dev/mono/go/logger package to the slog-based
// go.f110.dev/mono/go/logger/slogger package.
//
// Run `make update-deps` after this tool to update BUILD.bazel files.
package main

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.f110.dev/xerrors"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/imports"

	"go.f110.dev/mono/go/cli"
)

const (
	loggerPkg  = "go.f110.dev/mono/go/logger"
	sloggerPkg = "go.f110.dev/mono/go/logger/slogger"
	zapPkg     = "go.uber.org/zap"
	slogPkg    = "log/slog"
	fmtPkg     = "fmt"
)

// preservedLoggerFn lists logger.X identifiers that have no slogger
// equivalent. If a file references only these, the logger import is kept.
var preservedLoggerFn = map[string]bool{
	"SetLogLevel":          true,
	"Flags":                true,
	"OutputStderr":         true,
	"OverrideKlog":         true,
	"HijackStandardLogrus": true,
	"HijackLogrus":         true,
	"NewNamedWriter":       true,
	"NamedWriter":          true,
	"LogLevel":             true,
	"LogEncoding":          true,
	"Output":               true,
	"XError":               true,
}

type migrator struct {
	dryRun  bool
	updated int
}

func (m *migrator) Flags(fs *cli.FlagSet) {
	fs.Bool("dry-run", "Print files that would change without writing them").Var(&m.dryRun)
}

func (m *migrator) Run(_ context.Context, _ *cli.Command, args []string) error {
	if len(args) == 0 {
		return xerrors.New("requires at least one path argument")
	}
	for _, p := range args {
		if err := m.walk(p); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "\n%d file(s) updated\n", m.updated)
	return nil
}

func (m *migrator) walk(root string) error {
	info, err := os.Stat(root)
	if err != nil {
		return xerrors.WithStack(err)
	}
	if !info.IsDir() {
		return m.migrateFile(root)
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			n := d.Name()
			if n == "vendor" || n == "node_modules" || n == "testdata" {
				return fs.SkipDir
			}
			if strings.HasPrefix(n, ".") && n != "." && n != ".." {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		return m.migrateFile(path)
	})
}

func (m *migrator) migrateFile(path string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return xerrors.WithStack(err)
	}
	out, changed, err := migrateSource(path, src)
	if err != nil {
		return xerrors.WithMessagef(err, "migrate %s", path)
	}
	if !changed {
		return nil
	}
	if m.dryRun {
		fmt.Printf("would update: %s\n", path)
		m.updated++
		return nil
	}
	if err := os.WriteFile(path, out, 0644); err != nil {
		return xerrors.WithStack(err)
	}
	fmt.Printf("updated: %s\n", path)
	m.updated++
	return nil
}

// migrateSource performs the AST rewrite on src and returns the formatted
// output. The changed flag is true when any rewrite happened. filename is
// only used in error messages and as a token.FileSet entry.
func migrateSource(filename string, src []byte) ([]byte, bool, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return nil, false, xerrors.WithStack(err)
	}

	loggerName, zapName := importedNames(file)
	if loggerName == "" && zapName == "" {
		return src, false, nil
	}

	r := &rewriter{loggerName: loggerName, zapName: zapName}
	astutil.Apply(file, r.pre, r.post)
	if !r.changed {
		return src, false, nil
	}

	uses := scanUses(file)
	if uses["slogger"] {
		astutil.AddImport(fset, file, sloggerPkg)
	}
	if uses["slog"] {
		astutil.AddImport(fset, file, slogPkg)
	}
	if r.addedFmt && !alreadyImports(file, fmtPkg) {
		astutil.AddImport(fset, file, fmtPkg)
	}
	if loggerName != "" && !uses[loggerName] {
		astutil.DeleteImport(fset, file, loggerPkg)
	}
	if zapName != "" && !uses[zapName] {
		astutil.DeleteImport(fset, file, zapPkg)
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return nil, false, xerrors.WithStack(err)
	}
	// goimports-style post-process: collapse single-imports back to short
	// form when possible and group stdlib / external / local sections.
	out, err := imports.Process(filename, buf.Bytes(), &imports.Options{
		Comments:   true,
		TabIndent:  true,
		TabWidth:   8,
		FormatOnly: true,
	})
	if err != nil {
		return nil, false, xerrors.WithStack(err)
	}
	return out, true, nil
}

func importedNames(file *ast.File) (loggerName, zapName string) {
	for _, imp := range file.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		alias := ""
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		switch p {
		case loggerPkg:
			if alias != "" {
				loggerName = alias
			} else {
				loggerName = "logger"
			}
		case zapPkg:
			if alias != "" {
				zapName = alias
			} else {
				zapName = "zap"
			}
		}
	}
	return loggerName, zapName
}

func alreadyImports(file *ast.File, path string) bool {
	for _, imp := range file.Imports {
		if p, err := strconv.Unquote(imp.Path.Value); err == nil && p == path {
			return true
		}
	}
	return false
}

func scanUses(file *ast.File) map[string]bool {
	uses := make(map[string]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		uses[id.Name] = true
		return true
	})
	return uses
}

type rewriter struct {
	loggerName string
	zapName    string
	changed    bool
	addedFmt   bool
}

func (r *rewriter) pre(c *astutil.Cursor) bool {
	call, ok := c.Node().(*ast.CallExpr)
	if !ok {
		return true
	}
	if len(call.Args) == 0 {
		return true
	}
	// If a sibling argument already attaches the error (logger.Error /
	// zap.Error), drop logger.StackTrace(err) outright. Otherwise rewrite it
	// as slogger.E(err) so the error info isn't lost.
	hasError := false
	for _, arg := range call.Args {
		if r.isLoggerCallNamed(arg, "Error") || r.isZapCallNamed(arg, "Error") {
			hasError = true
			break
		}
	}
	newArgs := make([]ast.Expr, 0, len(call.Args))
	mutated := false
	for _, arg := range call.Args {
		if r.isLoggerCallNamed(arg, "StackTrace") {
			if hasError {
				mutated = true
				continue
			}
			// Convert to slogger.E(err).
			stackCall := arg.(*ast.CallExpr)
			stackCall.Fun = &ast.SelectorExpr{
				X:   &ast.Ident{Name: "slogger"},
				Sel: &ast.Ident{Name: "E"},
			}
			newArgs = append(newArgs, stackCall)
			hasError = true
			mutated = true
			continue
		}
		newArgs = append(newArgs, arg)
	}
	if mutated {
		call.Args = newArgs
		r.changed = true
	}
	return true
}

func (r *rewriter) isZapCallNamed(expr ast.Expr, fnName string) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return id.Name == r.zapName && sel.Sel.Name == fnName
}

func (r *rewriter) post(c *astutil.Cursor) bool {
	switch n := c.Node().(type) {
	case *ast.CallExpr:
		r.rewriteCall(n)
	case *ast.SelectorExpr:
		r.rewriteSelector(n)
	}
	return true
}

// rewriteCall handles transformations that need to inspect or modify
// the call's argument list (e.g. wrapping an arg in a cast).
func (r *rewriter) rewriteCall(call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}
	switch id.Name {
	case r.loggerName:
		switch sel.Sel.Name {
		case "Error":
			id.Name = "slogger"
			sel.Sel.Name = "E"
			r.changed = true
		case "Stringf":
			if len(call.Args) >= 2 {
				key := call.Args[0]
				sprintf := &ast.CallExpr{
					Fun: &ast.SelectorExpr{
						X:   &ast.Ident{Name: "fmt"},
						Sel: &ast.Ident{Name: "Sprintf"},
					},
					Args: call.Args[1:],
				}
				id.Name = "slog"
				sel.Sel.Name = "String"
				call.Args = []ast.Expr{key, sprintf}
				r.addedFmt = true
				r.changed = true
			}
		}
	case r.zapName:
		switch sel.Sel.Name {
		case "Error":
			id.Name = "slogger"
			sel.Sel.Name = "E"
			r.changed = true
		case "Int32":
			if len(call.Args) == 2 {
				call.Args[1] = &ast.CallExpr{
					Fun:  &ast.Ident{Name: "int"},
					Args: []ast.Expr{call.Args[1]},
				}
				id.Name = "slog"
				sel.Sel.Name = "Int"
				r.changed = true
			}
		case "Uint":
			if len(call.Args) == 2 {
				call.Args[1] = &ast.CallExpr{
					Fun:  &ast.Ident{Name: "uint64"},
					Args: []ast.Expr{call.Args[1]},
				}
				id.Name = "slog"
				sel.Sel.Name = "Uint64"
				r.changed = true
			}
		}
	}
}

// rewriteSelector handles pure rename transforms. Cases that need arg-list
// edits (Error, Stringf, Int32, Uint) are skipped here and handled by
// rewriteCall.
func (r *rewriter) rewriteSelector(sel *ast.SelectorExpr) {
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}
	switch id.Name {
	case r.loggerName:
		name := sel.Sel.Name
		if name == "Error" || name == "Stringf" || preservedLoggerFn[name] {
			return
		}
		switch name {
		case "Log", "Init", "Enabled", "Verbose", "KubernetesObject",
			"StandardLogger", "NewBufferLogger":
			id.Name = "slogger"
			r.changed = true
		case "String":
			id.Name = "slog"
			r.changed = true
		}
	case r.zapName:
		name := sel.Sel.Name
		if name == "Error" || name == "Int32" || name == "Uint" {
			return
		}
		switch name {
		case "String", "Int", "Int64", "Uint64", "Bool", "Duration", "Time", "Any":
			id.Name = "slog"
			r.changed = true
		case "Strings", "Array":
			id.Name = "slog"
			sel.Sel.Name = "Any"
			r.changed = true
		}
	}
}

func (r *rewriter) isLoggerCallNamed(expr ast.Expr, fnName string) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return id.Name == r.loggerName && sel.Sel.Name == fnName
}

func main() {
	m := &migrator{}
	cmd := &cli.Command{
		Use:   "slogger-migrate <path>...",
		Short: "Migrate code from go/logger (zap) to go/logger/slogger (slog).",
		Run:   m.Run,
	}
	m.Flags(cmd.Flags())
	if err := cmd.Execute(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
