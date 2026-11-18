package jujutsu

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"go.f110.dev/xerrors"
)

type Alias struct {
	Name string
	// Args is the arguments of jj which the alias runs.
	Args []string
}

const aliasTemplate = `"{\"name\":" ++ json(name) ++ ",\"value\":" ++ json(value) ++ "}\n"`

// Aliases returns the aliases configured by the user. The default aliases of jj are not included.
func (r *Repository) Aliases(ctx context.Context) ([]*Alias, error) {
	out, err := run(ctx, r.root, "config", "list", "--ignore-working-copy", "-T", aliasTemplate, "aliases")
	if err != nil {
		return nil, err
	}
	return parseAliases(out)
}

func parseAliases(out []byte) ([]*Alias, error) {
	var aliases []*Alias
	for line := range strings.Lines(string(out)) {
		var v struct {
			Name  string          `json:"name"`
			Value json.RawMessage `json:"value"`
		}
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			return nil, xerrors.WithMessagef(err, "failed to parse the alias: %s", line)
		}
		var args []string
		// The alias which is not the list of the arguments (e.g. the table of the subcommands) is ignored.
		if err := json.Unmarshal(v.Value, &args); err != nil {
			continue
		}
		aliases = append(aliases, &Alias{Name: strings.TrimPrefix(v.Name, "aliases."), Args: args})
	}
	return aliases, nil
}

// RunAlias runs the alias configured by the user and returns the output of stdout and stderr.
func (r *Repository) RunAlias(ctx context.Context, name string) (string, error) {
	aliases, err := r.Aliases(ctx)
	if err != nil {
		return "", err
	}
	if !slices.ContainsFunc(aliases, func(a *Alias) bool { return a.Name == name }) || strings.HasPrefix(name, "-") {
		return "", xerrors.Newf("unknown alias: %s", name)
	}
	stdout, stderr, err := runWithStderr(ctx, r.root, name)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(stdout) + string(stderr)), nil
}
