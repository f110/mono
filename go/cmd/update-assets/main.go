package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/bazelbuild/buildtools/build"
	"github.com/google/go-github/v73/github"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/enumerable"
	"go.f110.dev/mono/go/logger"
)

var (
	ignoreKustomizeVersion = []string{
		"v5.2.0", // darwin/arm64 is not distributed
		"v5.1.1", // darwin/arm64 is not distributed
		"v5.1.0", // darwin/arm64 is not distributed
	}
	minimumKindVersion = semver.MustParse("0.20.0")
)

const (
	KustomizeRepositoryOwner = "kubernetes-sigs"
	KustomizeRepositoryName  = "kustomize"
	KindRepositoryOwner      = "kubernetes-sigs"
	KindRepositoryName       = "kind"
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

func getChecksum(ctx context.Context, url string) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	logger.Log.Info("Get", logger.String("url", url))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
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

		osFiles := &build.DictExpr{ForceMultiLine: true}
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
			List:           files,
			ForceMultiLine: true,
		},
	}

	return kv
}

type kustomizeAssets struct {
	assetsFile string
	overwrite  bool
}

func (k *kustomizeAssets) Flags(fs *cli.FlagSet) {
	fs.String("assets-file", "File path of assets.bzl").Var(&k.assetsFile).Required()
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
			logger.Log.Info("Already exists", zap.String("version", ver))
			continue
		}
		if _, ok := ignoreVersions["kustomize"][ver]; ok {
			logger.Log.Info("Ignored version", zap.String("version", ver))
			continue
		}
		vers = append(vers, ver)
	}
	if len(vers) == 0 {
		logger.Log.Info("No need to update the asset file")
		return nil
	}

	for _, v := range vers {
		logger.Log.Debug("Get", zap.String("version", v))
		rel, err := k.getRelease(ctx, gClient, v)
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

func (k *kustomizeAssets) getRelease(ctx context.Context, gClient *github.Client, ver string) (*release, error) {
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

type kindAssets struct {
	assetsFile string
	overwrite  bool
}

func (k *kindAssets) Flags(fs *cli.FlagSet) {
	fs.String("assets-file", "File path of assets.bzl").Var(&k.assetsFile).Required()
	fs.Bool("overwrite", "Overwrite").Var(&k.overwrite)
}

func (k *kindAssets) update(ctx context.Context) error {
	buf, err := os.ReadFile(k.assetsFile)
	if err != nil {
		return xerrors.WithStack(err)
	}
	f, err := build.Parse(filepath.Base(k.assetsFile), buf)
	if err != nil {
		return xerrors.WithStack(err)
	}
	if len(f.Stmt) != 1 {
		return xerrors.Definef("the file has to include dict assign only").WithStack()
	}

	a, ok := f.Stmt[0].(*build.AssignExpr)
	if !ok {
		return xerrors.Definef("statement is not assign: %T", f.Stmt[0]).WithStack()
	}
	dict, ok := a.RHS.(*build.DictExpr)
	if !ok {
		return xerrors.Definef("RHS is not dict: %T", a.RHS).WithStack()
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
	rel, _, err := gClient.Repositories.ListReleases(ctx, KindRepositoryOwner, KindRepositoryName, &github.ListOptions{})
	if err != nil {
		return xerrors.WithStack(err)
	}
	vers := make([]string, 0)
	for _, v := range rel {
		ver := strings.TrimPrefix(v.GetTagName(), "v")
		sVer, err := semver.NewVersion(ver)
		if err != nil {
			continue
		}
		if sVer.LessThan(minimumKindVersion) {
			continue
		}

		if _, ok := exists[ver]; ok {
			logger.Log.Info("Already exists", zap.String("version", ver))
			continue
		}
		vers = append(vers, ver)
	}
	if len(vers) == 0 {
		logger.Log.Info("No need to update the asset file")
		return nil
	}

	for _, v := range vers {
		rel, err := k.getRelease(ctx, gClient, v)
		if err != nil {
			return err
		}
		dict.List = append(dict.List, releaseToKeyValueExpr(rel))
		sort.Slice(dict.List, func(i, j int) bool {
			left := semver.MustParse(dict.List[i].Key.(*build.StringExpr).Value)
			right := semver.MustParse(dict.List[j].Key.(*build.StringExpr).Value)
			return left.GreaterThan(right)
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

func (k *kindAssets) getRelease(ctx context.Context, gClient *github.Client, ver string) (*release, error) {
	rel, _, err := gClient.Repositories.GetReleaseByTag(ctx, KindRepositoryOwner, KindRepositoryName, "v"+ver)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	newRelease := &release{Version: ver, Assets: make([]*asset, 0)}
	for _, v := range rel.Assets {
		if strings.HasSuffix(v.GetName(), ".sha256sum") {
			continue
		}
		s := strings.Split(v.GetName(), "-")
		if s[1] != "darwin" && s[1] != "linux" {
			continue
		}
		checksums, err := getChecksum(ctx, v.GetBrowserDownloadURL()+".sha256sum")
		if err != nil {
			continue
		}

		a := &asset{
			OS:     s[1],
			Arch:   s[2],
			URL:    v.GetBrowserDownloadURL(),
			SHA256: checksums[v.GetName()],
		}
		newRelease.Assets = append(newRelease.Assets, a)
	}
	return newRelease, nil
}

type vaultAssets struct {
	assetsFile string
	overwrite  bool
}

func (h *vaultAssets) Flags(fs *cli.FlagSet) {
	fs.String("assets-file", "File path of assets.bzl").Var(&h.assetsFile).Required()
	fs.Bool("overwrite", "Overwrite").Var(&h.overwrite)
}

type hashicorpRelease struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Builds       []*hashicorpBuild `json:"builds"`
	IsPreRelease bool              `json:"is_prerelease"`
	ChecksumURL  string            `json:"url_shasums"`
	Created      time.Time         `json:"timestamp_created"`

	semver *semver.Version
}

type hashicorpBuild struct {
	Arch string `json:"arch"`
	OS   string `json:"os"`
	URL  string `json:"url"`
}

func (h *vaultAssets) update(ctx context.Context) error {
	buf, err := os.ReadFile(h.assetsFile)
	if err != nil {
		return xerrors.WithStack(err)
	}
	f, err := build.Parse(filepath.Base(h.assetsFile), buf)
	if err != nil {
		return xerrors.WithStack(err)
	}
	if len(f.Stmt) != 1 {
		return xerrors.Definef("the file has to include dict assign only").WithStack()
	}

	a, ok := f.Stmt[0].(*build.AssignExpr)
	if !ok {
		return xerrors.Definef("statement is not assign: %T", f.Stmt[0]).WithStack()
	}
	dict, ok := a.RHS.(*build.DictExpr)
	if !ok {
		return xerrors.Definef("RHS is not dict: %T", a.RHS).WithStack()
	}
	exists := make(map[string]*build.KeyValueExpr)
	for _, v := range dict.List {
		key, ok := v.Key.(*build.StringExpr)
		if !ok {
			continue
		}
		exists[key.Value] = v
	}

	var releases []*hashicorpRelease
	q := url.Values{}
	q.Set("license_class", "oss")
	q.Set("limit", "20")
	for {
		logger.Log.Info("Req", logger.String("path", "/v1/releases/vault?%s"+q.Encode()))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.releases.hashicorp.com/v1/releases/vault?"+q.Encode(), nil)
		if err != nil {
			return xerrors.WithStack(err)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return xerrors.WithStack(err)
		}
		var got []*hashicorpRelease
		if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
			return xerrors.WithStack(err)
		}
		if len(got) == 0 {
			break
		}
		stable := enumerable.FindAll(got, func(v *hashicorpRelease) bool { return !v.IsPreRelease })
		candidates := enumerable.Each(
			enumerable.FindAll(stable,
				func(h *hashicorpRelease) bool { _, ok := exists[h.Version]; return !ok }),
			func(h *hashicorpRelease) { h.semver = semver.MustParse(h.Version) },
		)
		releases = append(releases, candidates...)
		if len(candidates) != len(stable) {
			break
		}
		q.Set("after", candidates[len(candidates)-1].Created.Format(time.RFC3339))
	}
	slices.SortFunc(releases, func(a, b *hashicorpRelease) int { return a.semver.Compare(b.semver) })

	for _, v := range releases {
		logger.Log.Debug("Get", logger.String("version", v.Version))
		rel, err := h.getRelease(ctx, v)
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
	if h.overwrite {
		if err := os.WriteFile(h.assetsFile, []byte(out), 0644); err != nil {
			return xerrors.WithStack(err)
		}
	}
	return nil
}

func (h *vaultAssets) getRelease(ctx context.Context, rel *hashicorpRelease) (*release, error) {
	checksums, err := getChecksum(ctx, rel.ChecksumURL)
	if err != nil {
		return nil, err
	}
	newRelease := &release{Version: rel.Version, Assets: make([]*asset, 0)}
	assets := make(map[string]*hashicorpBuild)
	for _, b := range rel.Builds {
		assets[fmt.Sprintf("%s/%s", b.OS, b.Arch)] = b
	}
	for _, osName := range []string{"darwin", "linux"} {
		for _, archName := range []string{"amd64", "arm64"} {
			if b, ok := assets[fmt.Sprintf("%s/%s", osName, archName)]; ok {
				u, err := url.Parse(b.URL)
				if err != nil {
					return nil, xerrors.WithStack(err)
				}
				newRelease.Assets = append(newRelease.Assets, &asset{
					OS:     osName,
					Arch:   archName,
					URL:    b.URL,
					SHA256: checksums[path.Base(u.Path)],
				})
			}
		}
	}
	return newRelease, nil
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

	kind := &kindAssets{}
	kindCmd := &cli.Command{
		Use: "kind",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return kind.update(ctx)
		},
	}
	kind.Flags(kindCmd.Flags())
	cmd.AddCommand(kindCmd)

	vault := &vaultAssets{}
	vaultCmd := &cli.Command{
		Use: "vault",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return vault.update(ctx)
		},
	}
	vault.Flags(vaultCmd.Flags())
	cmd.AddCommand(vaultCmd)

	return cmd.Execute(args)
}

func main() {
	if err := updateAssets(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
