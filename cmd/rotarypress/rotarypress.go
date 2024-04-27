package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/blang/semver/v4"
	"github.com/google/go-github/v49/github"
	"go.f110.dev/xerrors"
	"go.starlark.net/syntax"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/githubutil"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/storage"
)

type RotaryPress struct {
	*fsm.FSM

	conf                []*ruleOnGithub
	bazelRelease        []*bazel
	client              *storage.S3
	httpClient          *http.Client
	githubClientFactory *githubutil.GitHubClientFactory

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
	bazel           bool
}

const (
	stateInit fsm.State = iota
	stateFetch
	stateFetchBazel
	stateStore
	stateFinish
)

func NewRotaryPress() *RotaryPress {
	r := &RotaryPress{httpClient: http.DefaultClient, githubClientFactory: githubutil.NewGitHubClientFactory("", false)}
	r.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateInit:       r.init,
			stateFetch:      r.fetch,
			stateFetchBazel: r.fetchBazel,
			stateStore:      r.store,
			stateFinish:     r.finish,
		},
		stateInit,
		stateFinish,
	)
	return r
}

func (r *RotaryPress) SetFlags(fs *cli.FlagSet) {
	fs.String("rules-macro-file", "Macro file path").Shorthand("c").Var(&r.macroFile)
	fs.Bool("dry-run", "Do not download and upload artifact files").Var(&r.dryRun)
	fs.String("endpoint", "").Var(&r.endpoint).Required()
	fs.String("bucket", "The bucket name").Var(&r.bucket).Required()
	fs.String("region", "").Var(&r.region)
	fs.String("access-key", "").Var(&r.accessKey).Required()
	fs.String("secret-access-key", "").Var(&r.secretAccessKey).Required()
	fs.String("ca-file", "File path that contains CA certificate").Var(&r.caFile)
	fs.String("prefix", "").Var(&r.prefix)
	fs.Bool("bazel", "Check bazel release").Var(&r.bazel)
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
	if err := r.githubClientFactory.Init(); err != nil {
		fsm.Error(err)
	}
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
	if r.bazel {
		return fsm.Next(stateFetchBazel)
	}
	return fsm.Next(stateStore)
}

func (r *RotaryPress) fetchBazel(ctx context.Context) (fsm.State, error) {
	releases, _, err := r.githubClientFactory.REST.Repositories.ListReleases(ctx, "bazelbuild", "bazel", &github.ListOptions{PerPage: 100})
	if err != nil {
		return fsm.Error(err)
	}
	env := []string{"linux-x86_64", "darwin-arm64"}
	minimumVer := semver.MustParse("6.0.0")
	for _, release := range releases {
		if release.GetPrerelease() {
			continue
		}

		v, err := semver.New(release.GetName())
		if err != nil {
			logger.Log.Warn("Failed to parse the version string as semver", logger.Error(err))
			continue
		}
		if v.LT(minimumVer) {
			continue
		}
		releasePair := make([]*bazel, 0)
		for _, e := range env {
			b := &bazel{ReleaseID: release.GetID(), Version: release.GetName(), Env: e}
			if r.client.ExistObject(ctx, b.storePath(r.prefix)) {
				continue
			}
			releasePair = append(releasePair, b)
		}

		a, _, err := r.githubClientFactory.REST.Repositories.ListReleaseAssets(ctx, "bazelbuild", "bazel", release.GetID(), &github.ListOptions{PerPage: 30})
		if err != nil {
			logger.Log.Warn("Failed to fetch the list of asset", logger.Error(err))
			continue
		}
		assets := make(map[string]*github.ReleaseAsset)
		for _, asset := range a {
			assets[asset.GetName()] = asset
		}
		for _, v := range releasePair {
			if asset, ok := assets[v.filename()]; ok {
				v.URL = asset.GetBrowserDownloadURL()
				r.bazelRelease = append(r.bazelRelease, v)
			}
		}
	}

	for _, release := range r.bazelRelease {
		if r.dryRun {
			logger.Log.Info("Skip download", zap.String("Version", release.Version), zap.String("Env", release.Env))
			f, err := os.CreateTemp("", "")
			if err != nil {
				return fsm.Error(err)
			}
			release.downloadedFile = f
		} else {
			err := release.Fetch(ctx, r.httpClient)
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
			logger.Log.Info("Upload rules", zap.String("path", r.storePath(v)))
			if err := r.client.PutReader(ctx, r.storePath(v), v.downloadedFile); err != nil {
				return fsm.Error(err)
			}
		}
	}

	for _, b := range r.bazelRelease {
		if b.downloadedFile == nil {
			continue
		}

		if r.dryRun {
			logger.Log.Info("Skip upload bazel", zap.String("Version", b.Version), zap.String("Env", b.Env), zap.String("path", b.storePath(r.prefix)))
		} else {
			logger.Log.Info("Upload bazel", zap.String("path", b.storePath(r.prefix)))
			if err := r.client.PutReader(ctx, b.storePath(r.prefix), b.downloadedFile); err != nil {
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

	for _, v := range r.bazelRelease {
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
	if r.macroFile == "" {
		return nil, nil
	}
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
		return nil, xerrors.Definef("got status: %d", res.StatusCode).WithStack()
	}

	h := sha256.New()
	reader := io.TeeReader(res.Body, h)
	f, err := os.CreateTemp("", "")
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	r.downloadedFile = f
	if _, err := io.Copy(f, reader); err != nil {
		return nil, xerrors.WithStack(err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, xerrors.WithStack(err)
	}
	calculatedHash := hex.EncodeToString(h.Sum(nil)[:])
	if r.SHA256 != "" && r.SHA256 != calculatedHash {
		os.Remove(f.Name())
		r.downloadedFile = nil
		return nil, xerrors.New("file hash is mismatched")
	}

	return f, nil
}

type bazel struct {
	ReleaseID int64
	Version   string
	Env       string
	URL       string

	downloadedFile *os.File
}

func (b *bazel) storePath(prefix string) string {
	return fmt.Sprintf("%s/releases.bazel.build/", prefix) + b.filename()
}

func (b *bazel) filename() string {
	return fmt.Sprintf("bazel-%s-%s", b.Version, b.Env)
}

func (b *bazel) Fetch(ctx context.Context, httpClient *http.Client) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.URL+".sha256", nil)
	if err != nil {
		return xerrors.WithStack(err)
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return xerrors.WithStack(err)
	}
	expectedHash, err := io.ReadAll(res.Body)
	if err != nil {
		return xerrors.WithStack(err)
	}
	if err := res.Body.Close(); err != nil {
		return xerrors.WithStack(err)
	}
	i := bytes.IndexByte(expectedHash, ' ')
	if i < 1 {
		return xerrors.New("the hash file is unexpected format")
	}
	expectedHash = expectedHash[:i]

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, b.URL, nil)
	if err != nil {
		return xerrors.WithStack(err)
	}
	res, err = httpClient.Do(req)
	if err != nil {
		return xerrors.WithStack(err)
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
	default:
		return xerrors.Definef("got status: %d", res.StatusCode).WithStack()
	}

	h := sha256.New()
	reader := io.TeeReader(res.Body, h)
	f, err := os.CreateTemp("", "")
	if err != nil {
		return xerrors.WithStack(err)
	}
	if _, err := io.Copy(f, reader); err != nil {
		return xerrors.WithStack(err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return xerrors.WithStack(err)
	}
	calculatedHash := hex.EncodeToString(h.Sum(nil))[:]
	if string(expectedHash) != calculatedHash {
		os.Remove(f.Name())
		return xerrors.Define("file hash is mismatched").WithStack()
	}
	b.downloadedFile = f

	return nil
}
