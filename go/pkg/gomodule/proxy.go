package gomodule

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v32/github"
	"go.uber.org/zap"
	"golang.org/x/mod/module"
	modzip "golang.org/x/mod/zip"
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

func (m *ModuleProxy) GetInfo(ctx context.Context, moduleName, version string) (Info, error) {
	moduleRoot, err := m.fetcher.Get(ctx, moduleName, m.GetConfig(moduleName))
	if err != nil {
		return Info{}, xerrors.Errorf(": %w", err)
	}

	mod := moduleRoot.FindModule(moduleName)
	if mod == nil {
		return Info{}, xerrors.Errorf("%s is not found", moduleName)
	}
	for _, v := range mod.Versions {
		if version == v.Semver {
			return Info{Version: v.Semver, Time: v.Time}, nil
		}
	}

	if moduleRoot.IsGitHub {
		if module.IsPseudoVersion(version) {
			pseudoVersion, err := ParsePseudoVersion(version)
			if err != nil {
				return Info{}, xerrors.Errorf(": %w", err)
			}
			i, err := m.ghProxy.GetInfoRevision(ctx, moduleRoot, moduleName, pseudoVersion)
			if err != nil {
				return i, xerrors.Errorf(": %w", err)
			}
			return i, nil
		} else {
			i, err := m.ghProxy.GetInfo(ctx, moduleRoot, moduleName, version)
			if err != nil {
				return i, xerrors.Errorf(": %w", err)
			}
			return i, nil
		}
	}

	return Info{}, xerrors.Errorf("%s is not found in %s", version, moduleName)
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
	if len(mod.Versions) > 0 {
		modVer := mod.Versions[len(mod.Versions)-1]
		return Info{Version: modVer.Version, Time: modVer.Time}, nil
	}

	moduleVer, err := mod.LatestVersion(ctx)
	if err != nil {
		return Info{}, xerrors.Errorf(": %w", err)
	}
	return Info{Version: moduleVer.Version, Time: moduleVer.Time}, nil
}

func (m *ModuleProxy) GetGoMod(ctx context.Context, moduleName, version string) (string, error) {
	moduleRoot, err := m.fetcher.Get(ctx, moduleName, m.GetConfig(moduleName))
	if err != nil {
		return "", xerrors.Errorf(": %w", err)
	}

	goMod := moduleRoot.FindModule(moduleName)
	if goMod == nil {
		return "", xerrors.Errorf("%s is not found", version)
	}

	goModFile, err := goMod.ModuleFile(version)
	if err == nil {
		return string(goModFile), nil
	}
	if moduleRoot.IsGitHub {
		if module.IsPseudoVersion(version) {
			pseudoVersion, err := ParsePseudoVersion(version)
			if err != nil {
				return "", xerrors.Errorf(": %w", err)
			}
			modFile, err := m.ghProxy.GetGoModRevision(ctx, moduleRoot, goMod, pseudoVersion)
			if err != nil {
				return "", xerrors.Errorf(": %w", err)
			}
			return modFile, nil
		} else {
			modFile, err := m.ghProxy.GetGoMod(ctx, moduleRoot, goMod, version)
			if err != nil {
				return "", xerrors.Errorf(": %w", err)
			}
			return modFile, nil
		}
	}

	return "", xerrors.Errorf("%s is not found", version)
}

func (m *ModuleProxy) GetZip(ctx context.Context, w io.Writer, moduleName, version string) error {
	moduleRoot, err := m.fetcher.Get(ctx, moduleName, m.GetConfig(moduleName))
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	err = moduleRoot.Archive(ctx, w, moduleName, version)
	if err == nil {
		return nil
	}
	if moduleRoot.IsGitHub {
		if module.IsPseudoVersion(version) {
			if err := m.ghProxy.ArchiveRevision(ctx, w, moduleRoot, moduleName, version); err != nil {
				return xerrors.Errorf(": %w", err)
			}
		} else {
			if err := m.ghProxy.Archive(ctx, w, moduleRoot, moduleName, version); err != nil {
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

func (g *GitHubProxy) Archive(ctx context.Context, w io.Writer, moduleRoot *ModuleRoot, moduleName, version string) error {
	if err := g.cache.Archive(ctx, moduleName, version, w); err == nil {
		logger.Log.Debug("An archive file of module was found in cache",
			zap.String("module", moduleName),
			zap.String("version", version),
		)
		return nil
	}

	mod := moduleRoot.FindModule(moduleName)
	if mod == nil {
		return xerrors.Errorf("%s module is not found", moduleName)
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

	archiver, err := NewModuleArchiveFromGitHub(g.githubClient, moduleRoot, moduleName, version, commit)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	buf := new(bytes.Buffer)
	if err := archiver.Pack(ctx, buf); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	data := buf.Bytes()
	if err := g.cache.SaveArchive(ctx, moduleName, version, data); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	if _, err := io.Copy(w, bytes.NewReader(data)); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (g *GitHubProxy) ArchiveRevision(ctx context.Context, w io.Writer, moduleRoot *ModuleRoot, moduleName, version string) error {
	mod := moduleRoot.FindModule(moduleName)
	if mod == nil {
		return xerrors.Errorf("%s module is not found", moduleName)
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
	if err := g.cache.Archive(ctx, moduleName, commit.GetSHA()[:12], w); err == nil {
		logger.Log.Debug("An archive file of module was found in cache",
			zap.String("module", moduleName),
			zap.String("revision", commit.GetSHA()[:12]),
		)
		return nil
	}

	archiver, err := NewModuleArchiveFromGitHub(g.githubClient, moduleRoot, moduleName, version, commit)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	buf := new(bytes.Buffer)
	if err := archiver.Pack(ctx, buf); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	data := buf.Bytes()
	if err := g.cache.SaveArchive(ctx, moduleName, commit.GetSHA()[:12], data); err != nil {
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
	logger.Log.Debug("Pack the archive file through GitHub API", zap.String("url", a.ModuleRoot.RepositoryURL))
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
		github.Zipball,
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

	fr, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	var files []modzip.File
	for _, v := range fr.File {
		if v.Mode().IsDir() {
			continue
		}
		files = append(files, newModFile(v))
	}
	if err := modzip.Create(w, module.Version{Path: a.Module.Path, Version: a.Version}, files); err != nil {
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

type modFile struct {
	f *zip.File
}

func newModFile(f *zip.File) *modFile {
	return &modFile{f: f}
}

func (f *modFile) Path() string {
	s := strings.Split(f.f.Name, "/")[1:]
	if s[len(s)-1] == "" {
		s = s[:len(s)-1]
	}
	return strings.Join(s, "/")
}

func (f *modFile) Lstat() (os.FileInfo, error) {
	return f.f.FileInfo(), nil
}

func (f *modFile) Open() (io.ReadCloser, error) {
	return f.f.Open()
}
