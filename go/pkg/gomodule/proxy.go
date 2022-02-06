package gomodule

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver"
	"github.com/google/go-github/v32/github"
	"go.uber.org/zap"
	"golang.org/x/xerrors"

	"go.f110.dev/mono/go/pkg/githubutil"
	"go.f110.dev/mono/go/pkg/logger"
)

const (
	moduleProxyUserAgent = "gomodule-proxy/v0.1 github.com/f110/gomodule-proxy"
)

type ModuleProxy struct {
	conf Config

	fetcher *ModuleFetcher
	ghProxy *GitHubProxy
	cache   *ModuleCache

	mu              sync.Mutex
	confLookupCache map[string]*ModuleSetting
}

func NewModuleProxy(conf Config, moduleDir string, cache *ModuleCache, ghClient *github.Client, tokenProvider *githubutil.TokenProvider, caBundle []byte) *ModuleProxy {
	return &ModuleProxy{
		conf:            conf,
		fetcher:         NewModuleFetcher(moduleDir, cache, tokenProvider, caBundle),
		ghProxy:         NewGitHubProxy(cache, ghClient),
		cache:           cache,
		confLookupCache: make(map[string]*ModuleSetting),
	}
}

func (m *ModuleProxy) GetConfig(module string) *ModuleSetting {
	m.mu.Lock()
	if v, ok := m.confLookupCache[module]; ok {
		m.mu.Unlock()
		return v
	}
	m.mu.Unlock()

	for _, v := range m.conf {
		if v.match.MatchString(module) {
			m.mu.Lock()
			m.confLookupCache[module] = v
			m.mu.Unlock()
			return v
		}
	}

	return nil
}

func (m *ModuleProxy) IsProxy(module string) bool {
	if v := m.GetConfig(module); v != nil {
		return true
	}

	return false
}

func (m *ModuleProxy) IsUpstream(module string) bool {
	return !m.IsProxy(module)
}

type Info struct {
	Version string
	Time    time.Time
}

func (m *ModuleProxy) Versions(ctx context.Context, module string) ([]string, error) {
	moduleRoot, err := m.fetcher.Get(ctx, module, m.GetConfig(module))
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	mod := moduleRoot.FindModule(module)
	if mod == nil {
		return nil, xerrors.Errorf("%s is not found", module)
	}

	var versions []string
	for _, v := range mod.Versions {
		versions = append(versions, v.Semver)
	}
	return versions, nil
}

func (m *ModuleProxy) GetInfo(ctx context.Context, module, version string) (Info, error) {
	moduleRoot, err := m.fetcher.Get(ctx, module, m.GetConfig(module))
	if err != nil {
		return Info{}, xerrors.Errorf(": %w", err)
	}

	mod := moduleRoot.FindModule(module)
	if mod == nil {
		return Info{}, xerrors.Errorf("%s is not found", module)
	}
	for _, v := range mod.Versions {
		if version == v.Semver {
			return Info{Version: v.Semver, Time: v.Time}, nil
		}
	}

	if moduleRoot.IsGitHub {
		if IsSemanticVersion(version) {
			i, err := m.ghProxy.GetInfo(ctx, moduleRoot, module, version)
			if err != nil {
				return i, xerrors.Errorf(": %w", err)
			}
			return i, nil
		} else {
			// Pseudo-version
			pseudoVersion, err := ParsePseudoVersion(version)
			if err != nil {
				return Info{}, xerrors.Errorf(": %w", err)
			}
			i, err := m.ghProxy.GetInfoRevision(ctx, moduleRoot, module, pseudoVersion)
			if err != nil {
				return i, xerrors.Errorf(": %w", err)
			}
			return i, nil
		}
	}

	return Info{}, xerrors.Errorf("%s is not found in %s", version, module)
}

func (m *ModuleProxy) GetLatestVersion(ctx context.Context, module string) (Info, error) {
	moduleRoot, err := m.fetcher.Get(ctx, module, m.GetConfig(module))
	if err != nil {
		return Info{}, xerrors.Errorf(": %w", err)
	}

	mod := moduleRoot.FindModule(module)
	if mod == nil {
		return Info{}, xerrors.Errorf("%s is not found", module)
	}

	modVer := mod.Versions[len(mod.Versions)-1]
	return Info{Version: modVer.Version, Time: modVer.Time}, nil
}

func (m *ModuleProxy) GetGoMod(ctx context.Context, module, version string) (string, error) {
	moduleRoot, err := m.fetcher.Get(ctx, module, m.GetConfig(module))
	if err != nil {
		return "", xerrors.Errorf(": %w", err)
	}

	mod := moduleRoot.FindModule(module)
	if mod == nil {
		return "", xerrors.Errorf("%s is not found", version)
	}

	goMod, err := mod.ModuleFile(version)
	if err == nil {
		return string(goMod), nil
	}
	if moduleRoot.IsGitHub {
		if IsSemanticVersion(version) {
			modFile, err := m.ghProxy.GetGoMod(ctx, moduleRoot, mod, version)
			if err != nil {
				return "", xerrors.Errorf(": %w", err)
			}
			return modFile, nil
		} else {
			// Pseudo-version
			pseudoVersion, err := ParsePseudoVersion(version)
			if err != nil {
				return "", xerrors.Errorf(": %w", err)
			}
			modFile, err := m.ghProxy.GetGoModRevision(ctx, moduleRoot, mod, pseudoVersion)
			if err != nil {
				return "", xerrors.Errorf(": %w", err)
			}
			return modFile, nil
		}
	}

	return "", xerrors.Errorf("%s is not found", version)
}

func (m *ModuleProxy) GetZip(ctx context.Context, w io.Writer, module, version string) error {
	moduleRoot, err := m.fetcher.Get(ctx, module, m.GetConfig(module))
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	err = moduleRoot.Archive(ctx, w, module, version)
	if err == nil {
		return nil
	}
	if moduleRoot.IsGitHub {
		if IsSemanticVersion(version) {
			if err := m.ghProxy.Archive(ctx, w, moduleRoot, module, version); err != nil {
				return xerrors.Errorf(": %w", err)
			}
		} else {
			// Pseudo-version
			if err := m.ghProxy.ArchiveRevision(ctx, w, moduleRoot, module, version); err != nil {
				return xerrors.Errorf(": %w", err)
			}
		}
		return nil
	}

	return xerrors.Errorf("%s is not found", version)
}

func (m *ModuleProxy) CachedModuleRoots() ([]*ModuleRoot, error) {
	moduleRoots, err := m.cache.CachedModuleRoots()
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	return moduleRoots, nil
}

func (m *ModuleProxy) InvalidateCache(module string) error {
	if err := m.cache.Invalidate(module); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (m *ModuleProxy) FlushAllCache() error {
	if err := m.cache.FlushAll(); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

type httpTransport struct {
	http.RoundTripper
}

var _ http.RoundTripper = &httpTransport{}

func (tr *httpTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", moduleProxyUserAgent)

	return http.DefaultTransport.RoundTrip(req)
}

type GitHubProxy struct {
	cache        *ModuleCache
	githubClient *github.Client
}

func NewGitHubProxy(cache *ModuleCache, ghClient *github.Client) *GitHubProxy {
	return &GitHubProxy{
		cache:        cache,
		githubClient: ghClient,
	}
}

func (g *GitHubProxy) GetInfo(ctx context.Context, moduleRoot *ModuleRoot, module, version string) (Info, error) {
	if len(version) > 11 {
		t, err := g.cache.GetModInfo(module, version[:12])
		if err == nil {
			logger.Log.Debug("The mod info was found in cache", zap.String("module", module), zap.String("version", version))
			return Info{Version: fmt.Sprintf("v0.0.0-%s-%s", t.Format("20060102150405"), version[:12])}, nil
		}
	}
	logger.Log.Debug("Get commit information from GitHub API", zap.String("url", moduleRoot.RepositoryURL))
	u, err := url.Parse(moduleRoot.RepositoryURL)
	if err != nil {
		return Info{}, xerrors.Errorf(": %w", err)
	}
	s := strings.Split(u.Path, "/")
	owner, repo := s[1], s[2]
	commit, _, err := g.githubClient.Repositories.GetCommit(ctx, owner, repo, version)
	if err != nil {
		return Info{}, xerrors.Errorf(": %w", err)
	}

	t := commit.Commit.Author.GetDate()
	if err := g.cache.SetModInfo(module, commit.GetSHA(), t); err != nil {
		logger.Log.Warn("Failed set cache", zap.Error(err))
	}
	return Info{Version: fmt.Sprintf("v0.0.0-%s-%s", t.Format("20060102150405"), commit.GetSHA()[:12]), Time: t}, nil
}

func (g *GitHubProxy) GetInfoRevision(ctx context.Context, moduleRoot *ModuleRoot, module string, pseudoVersion *PseudoVersion) (Info, error) {
	if len(pseudoVersion.Revision) > 11 {
		t, err := g.cache.GetModInfo(module, pseudoVersion.Revision)
		if err == nil {
			logger.Log.Debug("The mod info was found in cache", zap.String("module", module), zap.String("revision", pseudoVersion.Revision))
			return Info{Version: fmt.Sprintf("v0.0.0-%s-%s", t.Format("20060102150504"), pseudoVersion.Revision)}, nil
		}
	}
	logger.Log.Debug("Get commit information of pseudo-version from GitHub API")
	u, err := url.Parse(moduleRoot.RepositoryURL)
	if err != nil {
		return Info{}, xerrors.Errorf(": %w", err)
	}
	s := strings.Split(u.Path, "/")
	owner, repo := s[1], s[2]
	commit, _, err := g.githubClient.Repositories.GetCommit(ctx, owner, repo, pseudoVersion.Revision)
	if err != nil {
		return Info{}, xerrors.Errorf(": %w", err)
	}

	t := commit.Commit.Committer.GetDate()
	if err := g.cache.SetModInfo(module, commit.GetSHA(), t); err != nil {
		logger.Log.Warn("Failed set cache", zap.String("module", module), zap.String("revision", pseudoVersion.Revision), zap.Error(err))
	}
	return Info{Version: fmt.Sprintf("v0.0.0-%s-%s", t.Format("20060102150405"), commit.GetSHA()[:12]), Time: t}, nil
}

func (g *GitHubProxy) GetGoMod(ctx context.Context, moduleRoot *ModuleRoot, module *Module, version string) (string, error) {
	if len(version) > 11 {
		modFile, err := g.cache.GetModFile(module.Path, version[:12])
		if err == nil {
			logger.Log.Debug("The module file was found in cache",
				zap.String("module", module.Path),
				zap.String("version", version[:12]),
			)
			return string(modFile), nil
		}
	}
	logger.Log.Debug("Get the module file from GitHub API", zap.String("url", moduleRoot.RepositoryURL))
	u, err := url.Parse(moduleRoot.RepositoryURL)
	if err != nil {
		return "", xerrors.Errorf(": %w", err)
	}
	s := strings.Split(u.Path, "/")
	owner, repo := s[1], s[2]
	contents, _, _, err := g.githubClient.Repositories.GetContents(
		ctx,
		owner,
		repo,
		module.ModFilePath,
		&github.RepositoryContentGetOptions{
			Ref: version,
		},
	)
	if err != nil {
		return "", xerrors.Errorf(": %w", err)
	}
	if contents == nil {
		return "", xerrors.Errorf("%s is not found", version)
	}
	buf, err := contents.GetContent()
	if err != nil {
		return "", xerrors.Errorf(": %w", err)
	}
	if err := g.cache.SetModFile(module.Path, version, []byte(buf)); err != nil {
		logger.Log.Warn("Failed set the module fie", zap.Error(err))
	}
	return buf, nil
}

func (g *GitHubProxy) GetGoModRevision(ctx context.Context, moduleRoot *ModuleRoot, module *Module, pseudoVersion *PseudoVersion) (string, error) {
	if len(pseudoVersion.Revision) > 11 {
		modFile, err := g.cache.GetModFile(module.Path, pseudoVersion.Revision)
		if err == nil {
			logger.Log.Debug("The module file was found in cache",
				zap.String("module", module.Path),
				zap.String("version", pseudoVersion.Revision),
			)
			return string(modFile), nil
		}
	}
	logger.Log.Debug("Get the module file of pseudo-version from GitHub API", zap.String("url", moduleRoot.RepositoryURL))
	u, err := url.Parse(moduleRoot.RepositoryURL)
	if err != nil {
		return "", xerrors.Errorf(": %w", err)
	}
	s := strings.Split(u.Path, "/")
	owner, repo := s[1], s[2]
	contents, _, _, err := g.githubClient.Repositories.GetContents(
		ctx,
		owner,
		repo,
		module.ModFilePath,
		&github.RepositoryContentGetOptions{
			Ref: pseudoVersion.Revision,
		},
	)
	if err != nil {
		return "", xerrors.Errorf(": %w", err)
	}
	if contents == nil {
		return "", xerrors.Errorf("%s is not found", pseudoVersion)
	}
	buf, err := contents.GetContent()
	if err != nil {
		return "", xerrors.Errorf(": %w", err)
	}

	if err := g.cache.SetModFile(module.Path, pseudoVersion.Revision, []byte(buf)); err != nil {
		logger.Log.Warn("Failed set the module fie", zap.Error(err))
	}
	return buf, nil
}

func (g *GitHubProxy) Archive(ctx context.Context, w io.Writer, moduleRoot *ModuleRoot, module, version string) error {
	if len(version) > 11 {
		if err := g.cache.Archive(ctx, module, version[:12], w); err == nil {
			logger.Log.Debug("An archive file of module was found in cache",
				zap.String("module", module),
				zap.String("version", version[:12]),
			)
			return nil
		}
	}

	mod := moduleRoot.FindModule(module)
	if mod == nil {
		return xerrors.Errorf("%s module is not found", module)
	}

	logger.Log.Debug("Make the archive file through GitHub API", zap.String("url", moduleRoot.RepositoryURL))
	u, err := url.Parse(moduleRoot.RepositoryURL)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	s := strings.Split(u.Path, "/")
	owner, repo := s[1], s[2]
	commit, _, err := g.githubClient.Repositories.GetCommit(ctx, owner, repo, version)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	archiver, err := NewModuleArchiveFromGitHub(g.githubClient, moduleRoot, module, version, commit)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	buf := new(bytes.Buffer)
	if err := archiver.Pack(ctx, buf); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	data := buf.Bytes()
	if err := g.cache.SaveArchive(ctx, module, commit.GetSHA()[:12], data); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if _, err := io.Copy(w, bytes.NewReader(data)); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (g *GitHubProxy) ArchiveRevision(ctx context.Context, w io.Writer, moduleRoot *ModuleRoot, module, version string) error {
	mod := moduleRoot.FindModule(module)
	if mod == nil {
		return xerrors.Errorf("%s module is not found", module)
	}

	logger.Log.Debug("Make the archive file for pseudo-version through GitHub API", zap.String("url", moduleRoot.RepositoryURL))
	pseudoVersion, err := ParsePseudoVersion(version)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	u, err := url.Parse(moduleRoot.RepositoryURL)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	s := strings.Split(u.Path, "/")
	owner, repo := s[1], s[2]
	commit, _, err := g.githubClient.Repositories.GetCommit(ctx, owner, repo, pseudoVersion.Revision)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	archiver, err := NewModuleArchiveFromGitHub(g.githubClient, moduleRoot, module, version, commit)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	buf := new(bytes.Buffer)
	if err := archiver.Pack(ctx, buf); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	data := buf.Bytes()
	if err := g.cache.SaveArchive(ctx, module, commit.GetSHA()[:12], data); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if _, err := io.Copy(w, bytes.NewReader(data)); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

type ModuleArchive struct {
	ModuleRoot *ModuleRoot
	Module     *Module
	Version    string
	Revision   string

	ghClient *github.Client
}

func NewModuleArchiveFromGitHub(ghClient *github.Client, moduleRoot *ModuleRoot, module, version string, commit *github.RepositoryCommit) (*ModuleArchive, error) {
	mod := moduleRoot.FindModule(module)
	if mod == nil {
		return nil, xerrors.Errorf("%s module is not found", module)
	}

	return &ModuleArchive{ModuleRoot: moduleRoot, Module: mod, Version: version, Revision: commit.GetSHA(), ghClient: ghClient}, nil
}

func (a *ModuleArchive) Pack(ctx context.Context, w io.Writer) error {
	logger.Log.Debug("Make the archive file through GitHub API", zap.String("url", a.ModuleRoot.RepositoryURL))
	u, err := url.Parse(a.ModuleRoot.RepositoryURL)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	s := strings.Split(u.Path, "/")
	owner, repo := s[1], s[2]
	archiveUrl, _, err := a.ghClient.Repositories.GetArchiveLink(
		ctx,
		owner,
		repo,
		github.Tarball,
		&github.RepositoryContentGetOptions{
			Ref: a.Revision,
		},
		true,
	)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, archiveUrl.String(), nil)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	tmpFile, err := os.CreateTemp("", "")
	if err != nil {
		res.Body.Close()
		return xerrors.Errorf(": %w", err)
	}
	defer os.Remove(tmpFile.Name())
	_, err = io.Copy(tmpFile, res.Body)
	res.Body.Close()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	archiveFile, err := os.Open(tmpFile.Name())
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	fr, err := gzip.NewReader(archiveFile)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	excludeDirs := make(map[string]struct{})
	for _, v := range a.ModuleRoot.Modules {
		if v.Path == a.Module.Path {
			continue
		}
		excludeDirs[filepath.Dir(v.ModFilePath)+"/"] = struct{}{}
	}
	goModFileDir := filepath.Dir(a.Module.ModFilePath)
	modDir := a.Module.Path + "@" + a.Version
	foundLicenseFile := false
	licenseFiles := make(map[string]*bytes.Buffer)

	zipWriter := zip.NewWriter(w)
	archiveFileReader := tar.NewReader(fr)
Walk:
	for {
		header, err := archiveFileReader.Next()
		if err == io.EOF {
			break
		}
		if header.Typeflag != tar.TypeReg {
			if _, err := io.Copy(io.Discard, archiveFileReader); err != nil {
				return xerrors.Errorf(": %w", err)
			}
			continue Walk
		}
		s := strings.Split(header.Name, "/")
		if len(s) < 2 {
			if _, err := io.Copy(io.Discard, archiveFileReader); err != nil {
				return xerrors.Errorf(": %w", err)
			}
			continue Walk
		}
		path := strings.Join(s[1:], "/")

		if !foundLicenseFile && filepath.Base(path) == "LICENSE" {
			buf := new(bytes.Buffer)
			if _, err := io.Copy(buf, archiveFileReader); err != nil {
				return xerrors.Errorf(": %w", err)
			}
			licenseFiles[path] = buf
		}

		// Skip a file it is located under exclude directories
		for k := range excludeDirs {
			if strings.HasPrefix(path, k) {
				if _, err := io.Copy(io.Discard, archiveFileReader); err != nil {
					return xerrors.Errorf(": %w", err)
				}
				continue Walk
			}
		}

		if goModFileDir != "." && !strings.HasPrefix(path, goModFileDir) {
			if _, err := io.Copy(io.Discard, archiveFileReader); err != nil {
				return xerrors.Errorf(": %w", err)
			}
			continue Walk
		}

		if filepath.Join(goModFileDir, "LICENSE") == path {
			foundLicenseFile = true
		}

		p := strings.TrimPrefix(path, goModFileDir)
		fileWriter, err := zipWriter.Create(filepath.Join(modDir, p))
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		if _, err := io.Copy(fileWriter, archiveFileReader); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	if !foundLicenseFile {
		d := goModFileDir
		for {
			buf, ok := licenseFiles[filepath.Join(d, "LICENSE")]
			if !ok {
				if d == "." {
					break
				}
				d = filepath.Dir(d)
				continue
			}

			fileWriter, err := zipWriter.Create(filepath.Join(modDir, "LICENSE"))
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
			if _, err := io.Copy(fileWriter, buf); err != nil {
				return xerrors.Errorf(": %w", err)
			}
			break
		}
	}
	if err := zipWriter.Close(); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

type PseudoVersion struct {
	BaseVersion string
	Timestamp   string
	Revision    string
}

func ParsePseudoVersion(version string) (*PseudoVersion, error) {
	s := strings.Split(version, "-")
	if len(s) != 3 {
		return nil, xerrors.New("invalid pseudo-version format")
	}
	ver, ts, rev := s[0], s[1], s[2]
	if strings.Contains(ts, "0.") {
		t := strings.Split(ts, "0.")
		ts = t[len(t)-1]
		if t[0] != "" {
			pre := strings.Join(t[:len(t)-1], "0.")
			ver = fmt.Sprintf("%s-%s", ver, pre[:len(pre)-1])
		}
	}
	_, err := time.Parse("20060102150405", ts)
	if err != nil {
		return nil, xerrors.Errorf("invalid timestamp in pseudo-version: %w", err)
	}
	if len(rev) < 12 {
		return nil, xerrors.Errorf("invalid revision: revision is shorter")
	} else if len(rev) > 12 {
		return nil, xerrors.Errorf("invalid revision: revision is longer")
	}

	return &PseudoVersion{BaseVersion: ver, Timestamp: ts, Revision: rev}, nil
}

func (p *PseudoVersion) String() string {
	return fmt.Sprintf("%s-%s-%s", p.BaseVersion, p.Timestamp, p.Revision)
}

func IsSemanticVersion(version string) bool {
	if len(version) == 0 {
		return false
	}

	ver, err := semver.Parse(version[1:])
	if err != nil {
		return false
	}
	if len(ver.Pre) != 0 {
		if ver.Pre[0].VersionStr == "pre" && len(ver.Pre) == 1 {
			return true
		}
		return false
	}

	return true
}
