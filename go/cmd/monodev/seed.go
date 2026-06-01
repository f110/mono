package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.f110.dev/xerrors"
	"gopkg.in/yaml.v3"

	"go.f110.dev/mono/go/logger/slogger"
)

type mysqlSeedDirectory struct {
	Name     string
	Dir      string
	MySQL    component
	Database component
}

var _ component = &mysqlSeedDirectory{}

func (c *mysqlSeedDirectory) GetName() string {
	return c.Name
}

func (c *mysqlSeedDirectory) GetType() componentType {
	return componentTypeOneshot
}

func (c *mysqlSeedDirectory) GetDeps() []component {
	return []component{c.MySQL, c.Database}
}

func (c *mysqlSeedDirectory) Run(ctx context.Context) {
	if c.MySQL.GetName() != "mysqld" {
		slogger.Log.Error("MySQL is not mysqld")
		return
	}
	dbComponent, ok := c.Database.(*mysqlDatabase)
	if !ok {
		slogger.Log.Error("Database field is not mysqlDatabase")
		return
	}

	dir := c.Dir
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(os.Getenv("BUILD_WORKING_DIRECTORY"), dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		slogger.Log.Error("Failed to read seed directory", slogger.E(err), slog.String("dir", dir))
		return
	}

	port := c.MySQL.(*mysqlComponent).Port.Number
	db, err := sql.Open("mysql", fmt.Sprintf("root@tcp(127.0.0.1:%d)/%s?parseTime=true", port, dbComponent.Name))
	if err != nil {
		slogger.Log.Error("Failed to connect to mysql", slogger.E(err))
		return
	}
	defer db.Close()

	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		files = append(files, name)
	}
	sort.Strings(files)

	for _, name := range files {
		table := strings.TrimSuffix(strings.TrimSuffix(name, ".yaml"), ".yml")
		path := filepath.Join(dir, name)
		f, err := os.Open(path)
		if err != nil {
			slogger.Log.Error("Failed to open seed file", slogger.E(err), slog.String("path", path))
			return
		}
		rows, err := parseSeed(f)
		f.Close()
		if err != nil {
			slogger.Log.Error("Failed to read seed file", slogger.E(err), slog.String("path", path))
			return
		}
		if len(rows) == 0 {
			continue
		}
		if err := upsertRows(ctx, db, table, rows); err != nil {
			slogger.Log.Error("Failed to upsert seed", slogger.E(err), slog.String("table", table))
			return
		}
		slogger.Log.Info("Seeded table", slog.String("table", table), slog.Int("rows", len(rows)))
	}
}

func parseSeed(r io.Reader) ([]map[string]any, error) {
	dec := yaml.NewDecoder(r)
	var rows []map[string]any
	for {
		var doc any
		err := dec.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		switch v := doc.(type) {
		case []any:
			for _, item := range v {
				m, ok := item.(map[string]any)
				if !ok {
					return nil, xerrors.Definef("seed item is not a mapping: %T", item).WithStack()
				}
				rows = append(rows, m)
			}
		case map[string]any:
			rows = append(rows, v)
		case nil:
		default:
			return nil, xerrors.Definef("unsupported seed document: %T", doc).WithStack()
		}
	}
	return rows, nil
}

func buildUpsertQuery(table string, rows []map[string]any) (string, []string) {
	columnSet := make(map[string]struct{})
	for _, r := range rows {
		for k := range r {
			columnSet[k] = struct{}{}
		}
	}
	columns := make([]string, 0, len(columnSet))
	for k := range columnSet {
		columns = append(columns, k)
	}
	sort.Strings(columns)

	placeholders := strings.Repeat("?, ", len(columns))
	placeholders = placeholders[:len(placeholders)-2]
	cols := make([]string, len(columns))
	updates := make([]string, len(columns))
	for i, c := range columns {
		cols[i] = "`" + c + "`"
		updates[i] = fmt.Sprintf("`%s` = VALUES(`%s`)", c, c)
	}
	query := fmt.Sprintf(
		"INSERT INTO `%s` (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
		table,
		strings.Join(cols, ", "),
		placeholders,
		strings.Join(updates, ", "),
	)
	return query, columns
}

func upsertRows(ctx context.Context, db *sql.DB, table string, rows []map[string]any) error {
	query, columns := buildUpsertQuery(table, rows)

	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return xerrors.WithMessage(err, "prepare upsert")
	}
	defer stmt.Close()

	for _, r := range rows {
		args := make([]any, len(columns))
		for i, c := range columns {
			args[i] = r[c]
		}
		if _, err := stmt.ExecContext(ctx, args...); err != nil {
			return xerrors.WithMessagef(err, "upsert row in %s", table)
		}
	}
	return nil
}
