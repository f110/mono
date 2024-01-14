package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"go.f110.dev/xerrors"
	"go.starlark.net/syntax"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/storage"
)

type RotaryPress struct {
	*fsm.FSM

	conf       []*ruleOnGithub
	client     *storage.S3
	httpClient *http.Client

	// Flags
	macroFile       string
	dryRun          bool
	endpoint        string
	region          string
	accessKey       string
	secretAccessKey string
	bucket          string
	caFile          string
	prefix          string
}

const (
	stateInit fsm.State = iota
	stateFetch
	stateStore
	stateFinish
)

func NewRotaryPress() *RotaryPress {
	r := &RotaryPress{httpClient: http.DefaultClient}
	r.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateInit:   r.init,
			stateFetch:  r.fetch,
			stateStore:  r.store,
			stateFinish: r.finish,
		},
		stateInit,
		stateFinish,
	)
	return r
}

func (r *RotaryPress) SetFlags(fs *cli.FlagSet) {
	fs.String("rules-macro-file", "Macro file path").Shorthand("c").Var(&r.macroFile).Required()
	fs.Bool("dry-run", "Do not download and upload artifact files").Var(&r.dryRun)
	fs.String("endpoint", "").Var(&r.endpoint).Required()
	fs.String("bucket", "The bucket name").Var(&r.bucket).Required()
	fs.String("region", "").Var(&r.region)
	fs.String("access-key", "").Var(&r.accessKey).Required()
	fs.String("secret-access-key", "").Var(&r.secretAccessKey).Required()
	fs.String("ca-file", "File path that contains CA certificate").Var(&r.caFile)
	fs.String("prefix", "").Var(&r.prefix)
}

func (r *RotaryPress) init(_ context.Context) (fsm.State, error) {
	if conf, err := r.readMacroFile(); err != nil {
		return fsm.Error(err)
	} else {
		r.conf = conf
	}
	opt := storage.NewS3OptionToExternal(r.endpoint, r.region, r.accessKey, r.secretAccessKey)
	opt.PathStyle = true
	opt.CACertFile = r.caFile
	r.client = storage.NewS3(r.bucket, opt)
	return fsm.Next(stateFetch)
}

func (r *RotaryPress) fetch(ctx context.Context) (fsm.State, error) {
	for _, v := range r.conf {
		if r.client.ExistObject(ctx, r.storePath(v)) {
			continue
		}

		if r.dryRun {
			logger.Log.Info("Skip download", zap.String("repository", v.Repository), zap.String("version", v.Version))
			f, err := os.CreateTemp("", "")
			if err != nil {
				return fsm.Error(err)
			}
			v.downloadedFile = f
		} else {
			_, err := v.Fetch(ctx, r.httpClient)
			if err != nil {
				return fsm.Error(err)
			}
		}
	}
	return fsm.Next(stateStore)
}

func (r *RotaryPress) store(ctx context.Context) (fsm.State, error) {
	for _, v := range r.conf {
		if v.downloadedFile == nil {
			continue
		}

		if r.dryRun {
			logger.Log.Info("Skip upload", zap.String("repository", v.Repository), zap.String("version", v.Version), zap.String("path", r.storePath(v)))
		} else {
			if err := r.client.PutReader(ctx, r.storePath(v), v.downloadedFile); err != nil {
				return fsm.Error(err)
			}
		}
	}
	return fsm.Next(stateFinish)
}

func (r *RotaryPress) finish(_ context.Context) (fsm.State, error) {
	for _, v := range r.conf {
		if v.downloadedFile != nil {
			os.Remove(v.downloadedFile.Name())
		}
	}

	return fsm.Finish()
}

func (r *RotaryPress) storePath(d *ruleOnGithub) string {
	if d.IsTagArchive {
		// e.g. bucket = mirror / prefix = None
		//   mirror/github.com/bazelbuild/rules_python/archive/refs/tags/0.9.0.tar.gz
		return path.Join(r.prefix, "github.com", d.Repository, "archive/refs/tags", fmt.Sprintf("%s.%s", d.Version, d.ArchiveFormat))
	}
	// e.g. bucket = mirror / prefix = None
	//   mirror/github.com/bazelbuild/rules_go/releases/download/v0.42.0/rules_go-v0.42.0.zip
	return path.Join(r.prefix, "github.com", d.Repository, "releases/download", d.Version, fmt.Sprintf("%s-%s.%s", d.Name, d.Version, d.ArchiveFormat))
}

func (r *RotaryPress) readMacroFile() ([]*ruleOnGithub, error) {
	macroFile, err := os.Open(r.macroFile)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	f, err := syntax.Parse(filepath.Base(r.macroFile), macroFile, 0)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return r.findRuleOnGithub(f.Stmts), nil
}

func (*RotaryPress) findRuleOnGithub(stmts []syntax.Stmt) []*ruleOnGithub {
	var results []*ruleOnGithub
	for _, v := range stmts {
		if _, ok := v.(*syntax.AssignStmt); !ok {
			continue
		}
		assign := v.(*syntax.AssignStmt)
		rh, ok := assign.RHS.(*syntax.DictExpr)
		if !ok {
			continue
		}
		for _, e := range rh.List {
			entry, ok := e.(*syntax.DictEntry)
			if !ok {
				continue
			}
			call, ok := entry.Value.(*syntax.CallExpr)
			if !ok {
				continue
			}
			funcName, ok := call.Fn.(*syntax.Ident)
			if !ok {
				continue
			}
			if funcName.Name != "rule_on_github" {
				continue
			}

			name := call.Args[0].(*syntax.Literal).Value.(string)
			repoName := call.Args[1].(*syntax.Literal).Value.(string)
			ver := call.Args[2].(*syntax.Literal).Value.(string)
			sha256 := call.Args[3].(*syntax.Literal).Value.(string)
			res := &ruleOnGithub{Name: name, Repository: repoName, Version: ver, SHA256: sha256, ArchiveFormat: "tar.gz"}
			for _, a := range call.Args[4:] {
				b, ok := a.(*syntax.BinaryExpr)
				if !ok {
					continue
				}
				kw := b.X.(*syntax.Ident)
				val := b.Y.(*syntax.Literal)
				switch kw.Name {
				case "type":
					if val.Value.(string) == "tag" {
						res.IsTagArchive = true
					}
				case "archive":
					res.ArchiveFormat = val.Value.(string)
				}
			}
			results = append(results, res)
		}
	}

	return results
}

type ruleOnGithub struct {
	Name          string
	Repository    string
	Version       string
	SHA256        string
	ArchiveFormat string
	IsTagArchive  bool

	downloadedFile *os.File
}

func (r *ruleOnGithub) Fetch(ctx context.Context, client *http.Client) (*os.File, error) {
	var u string
	if r.IsTagArchive {
		u = fmt.Sprintf("https://github.com/%s/archive/refs/tags/%s.%s", r.Repository, r.Version, r.ArchiveFormat)
	} else {
		u = fmt.Sprintf("https://github.com/%s/releases/download/%s/%s-%s.%s", r.Repository, r.Version, r.Name, r.Version, r.ArchiveFormat)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
	default:
		return nil, xerrors.Newf("got status: %d", res.StatusCode)
	}

	f, err := os.CreateTemp("", "")
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	r.downloadedFile = f
	if _, err := io.Copy(f, res.Body); err != nil {
		return nil, xerrors.WithStack(err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, xerrors.WithStack(err)
	}

	return f, nil
}
