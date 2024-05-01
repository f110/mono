package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/bazelbuild/buildtools/build"
	"github.com/google/go-github/v49/github"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/cli"
)

var (
	ignoreKustomizeVersion = []string{
		"v5.2.0", // darwin/arm64 is not distributed
		"v5.1.1", // darwin/arm64 is not distributed
		"v5.1.0", // darwin/arm64 is not distributed
	}
)

const (
	KustomizeRepositoryOwner = "kubernetes-sigs"
	KustomizeRepositoryName  = "kustomize"
)

type release struct {
	Version string
	Assets  []*asset
}

type asset struct {
	OS     string
	Arch   string
	URL    string
	SHA256 string
}

var ignoreVersions map[string]map[string]struct{}

func init() {
	ignoreVersions = make(map[string]map[string]struct{})

	ignoreVersions["kustomize"] = make(map[string]struct{})
	for _, v := range ignoreKustomizeVersion {
		ignoreVersions["kustomize"][v] = struct{}{}
	}
}

func getRelease(ctx context.Context, gClient *github.Client, ver string) (*release, error) {
	rel, _, err := gClient.Repositories.GetReleaseByTag(
		ctx,
		KustomizeRepositoryOwner,
		KustomizeRepositoryName,
		fmt.Sprintf("kustomize/%s", ver),
	)
	if err != nil {
		return nil, err
	}

	assets := make(map[string]*asset)
	checksums := make(map[string]string)
	foundChecksums := false
	for _, v := range rel.Assets {
		if v.GetName() == "checksums.txt" {
			foundChecksums = true
			checksums, err = getChecksum(ctx, v.GetBrowserDownloadURL())
			if err != nil {
				return nil, err
			}
			continue
		}
		s := strings.Split(v.GetName(), "_")
		if s[3] != "amd64.tar.gz" && s[3] != "arm64.tar.gz" {
			continue
		}
		a := strings.Split(s[3], ".")
		arch := a[0]
		assets[v.GetName()] = &asset{
			OS:   s[2],
			Arch: arch,
			URL:  v.GetBrowserDownloadURL(),
		}
	}
	if !foundChecksums {
		return nil, xerrors.Define("checksums.txt is not found").WithStack()
	}
	newRelease := &release{Version: ver, Assets: make([]*asset, 0)}
	for _, v := range assets {
		u, err := url.Parse(v.URL)
		if err != nil {
			return nil, err
		}
		filename := filepath.Base(u.Path)
		if checksum, ok := checksums[filename]; !ok {
			return nil, xerrors.Definef("unknown filename: %s", filename).WithStack()
		} else {
			v.SHA256 = checksum
		}

		newRelease.Assets = append(newRelease.Assets, v)
	}

	return newRelease, nil
}

func getChecksum(ctx context.Context, url string) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	contents, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	sums := make(map[string]string)
	s := bufio.NewScanner(bytes.NewReader(contents))
	for s.Scan() {
		line := s.Text()
		s := strings.Split(line, "  ")
		sums[s[1]] = s[0]
	}

	return sums, nil
}

func releaseToKeyValueExpr(release *release) *build.KeyValueExpr {
	sort.Slice(release.Assets, func(i, j int) bool {
		return release.Assets[i].OS < release.Assets[j].OS
	})
	assets := make(map[string][]*asset)
	var osNames []string
	for _, v := range release.Assets {
		if _, ok := assets[v.OS]; !ok {
			osNames = append(osNames, v.OS)
		}
		assets[v.OS] = append(assets[v.OS], v)
	}
	sort.Strings(osNames)

	files := make([]*build.KeyValueExpr, 0)
	for _, osName := range osNames {
		v := assets[osName]
		sort.Slice(v, func(i, j int) bool {
			return v[i].Arch < v[j].Arch
		})

		osFiles := &build.DictExpr{}
		for _, a := range v {
			osFiles.List = append(osFiles.List, &build.KeyValueExpr{
				Key: &build.StringExpr{Value: a.Arch},
				Value: &build.TupleExpr{
					List: []build.Expr{
						&build.StringExpr{Value: a.URL},
						&build.StringExpr{Value: a.SHA256},
					},
				},
			})
		}

		files = append(files, &build.KeyValueExpr{
			Key:   &build.StringExpr{Value: osName},
			Value: osFiles,
		})
	}

	kv := &build.KeyValueExpr{
		Key: &build.StringExpr{Value: release.Version},
		Value: &build.DictExpr{
			List: files,
		},
	}

	return kv
}

type kustomizeAssets struct {
	assetsFile string
	overwrite  bool
}

func (k *kustomizeAssets) Flags(fs *cli.FlagSet) {
	fs.String("assets-file", "File path of assets.bzl").Var(&k.assetsFile)
	fs.Bool("overwrite", "Overwrite").Var(&k.overwrite)
}

func (k *kustomizeAssets) update(ctx context.Context) error {
	buf, err := os.ReadFile(k.assetsFile)
	if err != nil {
		return err
	}
	f, err := build.Parse(filepath.Base(k.assetsFile), buf)
	if err != nil {
		return err
	}
	if len(f.Stmt) != 1 {
		return xerrors.Define("the file has to include dict assign only").WithStack()
	}
	a, ok := f.Stmt[0].(*build.AssignExpr)
	if !ok {
		return xerrors.Definef("statement is not assign: %s", reflect.TypeOf(f.Stmt[0]).String()).WithStack()
	}
	dict, ok := a.RHS.(*build.DictExpr)
	if !ok {
		return xerrors.Definef("RHS is not dict: %s", reflect.TypeOf(a.RHS).String()).WithStack()
	}
	exists := make(map[string]*build.KeyValueExpr)
	for _, v := range dict.List {
		key, ok := v.Key.(*build.StringExpr)
		if !ok {
			continue
		}
		exists[key.Value] = v
	}

	gClient := github.NewClient(nil)
	rel, _, err := gClient.Repositories.ListReleases(ctx, KustomizeRepositoryOwner, KustomizeRepositoryName, &github.ListOptions{})
	if err != nil {
		return xerrors.WithStack(err)
	}
	vers := make([]string, 0)
	for _, v := range rel {
		if !strings.HasPrefix(v.GetName(), "kustomize/") {
			continue
		}
		ver := strings.TrimPrefix(v.GetName(), "kustomize/")
		if _, ok := exists[ver]; ok {
			log.Printf("%s is already exists", ver)
			continue
		}
		if _, ok := ignoreVersions["kustomize"][ver]; ok {
			log.Printf("%s is ignored version", ver)
			continue
		}
		vers = append(vers, ver)
	}
	if len(vers) == 0 {
		log.Print("No need to update the asset file")
		return nil
	}

	for _, v := range vers {
		log.Printf("Get %s", v)
		rel, err := getRelease(ctx, gClient, v)
		if err != nil {
			return err
		}
		dict.List = append(dict.List, releaseToKeyValueExpr(rel))
		sort.Slice(dict.List, func(i, j int) bool {
			left := semver.MustParse(dict.List[i].Key.(*build.StringExpr).Value)
			right := semver.MustParse(dict.List[j].Key.(*build.StringExpr).Value)
			return left.LessThan(right)
		})
	}
	out := build.FormatString(f)
	fmt.Print(out)

	if k.overwrite {
		if err := os.WriteFile(k.assetsFile, []byte(out), 0644); err != nil {
			return err
		}
	}
	return nil
}

func updateAssets(args []string) error {
	cmd := &cli.Command{
		Use: "update-assets",
	}

	kustomize := &kustomizeAssets{}
	kustomizeCmd := &cli.Command{
		Use: "kustomize",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return kustomize.update(ctx)
		},
	}
	kustomize.Flags(kustomizeCmd.Flags())
	cmd.AddCommand(kustomizeCmd)

	return cmd.Execute(args)
}

func main() {
	if err := updateAssets(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
