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

	"github.com/blang/semver/v4"
	"github.com/google/go-github/v73/github"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/githubutil"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/storage"
)

type RotaryPress struct {
	*fsm.FSM

	bazelRelease        []*bazel
	client              *storage.S3
	httpClient          *http.Client
	githubClientFactory *githubutil.GitHubClientFactory

	// Flags
	dryRun              bool
	endpoint            string
	region              string
	accessKey           string
	secretAccessKey     string
	secretAccessKeyFile string
	bucket              string
	caFile              string
	prefix              string
}

const (
	stateInit fsm.State = iota
	stateFetchBazel
	stateStore
	stateFinish
)

func NewRotaryPress() *RotaryPress {
	r := &RotaryPress{httpClient: http.DefaultClient, githubClientFactory: githubutil.NewGitHubClientFactory("", false)}
	r.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateInit:       r.init,
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
	fs.Bool("dry-run", "Do not download and upload artifact files").Var(&r.dryRun)
	fs.String("endpoint", "").Var(&r.endpoint).Required()
	fs.String("bucket", "The bucket name").Var(&r.bucket).Required()
	fs.String("region", "").Var(&r.region)
	fs.String("access-key", "").Var(&r.accessKey).Required()
	fs.String("secret-access-key", "").Var(&r.secretAccessKey)
	fs.String("secret-access-key-file", "").Var(&r.secretAccessKeyFile)
	fs.String("ca-file", "File path that contains CA certificate").Var(&r.caFile)
	fs.String("prefix", "").Var(&r.prefix)
}

func (r *RotaryPress) init(_ context.Context) (fsm.State, error) {
	if r.secretAccessKey == "" && r.secretAccessKeyFile == "" {
		return fsm.Error(xerrors.New("--secret-access-key and --secret-access-key-file must be set"))
	}

	if r.secretAccessKeyFile != "" {
		buf, err := os.ReadFile(r.secretAccessKeyFile)
		if err != nil {
			return fsm.Error(err)
		}
		r.secretAccessKey = string(bytes.TrimSuffix(buf, []byte("\n")))
	}
	opt := storage.NewS3OptionToExternal(r.endpoint, r.region, r.accessKey, r.secretAccessKey)
	opt.PathStyle = true
	opt.CACertFile = r.caFile
	r.client = storage.NewS3(r.bucket, opt)
	if err := r.githubClientFactory.Init(); err != nil {
		fsm.Error(err)
	}
	return fsm.Next(stateFetchBazel)
}

func (r *RotaryPress) fetchBazel(ctx context.Context) (fsm.State, error) {
	releases, _, err := r.githubClientFactory.REST.Repositories.ListReleases(ctx, "bazelbuild", "bazel", &github.ListOptions{PerPage: 100})
	if err != nil {
		return fsm.Error(err)
	}
	env := []string{"linux-x86_64", "darwin-arm64"}
	minimumVer := semver.MustParse("7.0.0")
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
			logger.Log.Info("Skip download", zap.String("Version", release.Version), zap.String("Env", release.Env), zap.String("url", release.URL))
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
	for _, v := range r.bazelRelease {
		if v.downloadedFile != nil {
			os.Remove(v.downloadedFile.Name())
		}
	}

	return fsm.Finish()
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
	logger.Log.Debug("Download hash file", zap.String("url", req.URL.String()))
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
	logger.Log.Debug("Download binary file", zap.String("url", req.URL.String()))
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
	logger.Log.Debug("Verify file", zap.String("hash", calculatedHash), zap.String("expected_hash", string(expectedHash)))
	if string(expectedHash) != calculatedHash {
		os.Remove(f.Name())
		return xerrors.Define("file hash is mismatched").WithStack()
	}
	b.downloadedFile = f

	return nil
}
