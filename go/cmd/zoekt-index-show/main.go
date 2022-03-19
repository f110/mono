package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/zoekt/query"
	"github.com/google/zoekt/shards"
	"github.com/spf13/pflag"
	"golang.org/x/xerrors"
)

func zoektIndexShow(args []string) error {
	indexDir := ""
	fs := pflag.NewFlagSet("zoekt-index-show", pflag.ContinueOnError)
	fs.StringVar(&indexDir, "index", indexDir, "Index directory")
	if err := fs.Parse(args); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	searcher, err := shards.NewDirectorySearcher(indexDir)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	repos, err := searcher.List(context.Background(), &query.Const{Value: true}, nil)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	for _, v := range repos.Repos {
		fmt.Printf("Name: %s\n", v.Repository.Name)
		branches := make([]string, 0)
		for _, b := range v.Repository.Branches {
			branches = append(branches, b.Name)
		}
		fmt.Printf("Branches: %s\n", strings.Join(branches, ", "))
		fmt.Printf("Documents: %d Total Index size: %d bytes\n", v.Stats.Documents, v.Stats.IndexBytes)
	}

	return nil
}

func main() {
	if err := zoektIndexShow(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
