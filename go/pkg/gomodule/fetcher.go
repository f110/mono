package gomodule

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	gogitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"go.uber.org/zap"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
	"golang.org/x/tools/go/vcs"
	"golang.org/x/xerrors"

	"go.f110.dev/mono/go/pkg/logger"
)

type ModuleRoot struct {
	RootPath      string
	RepositoryURL string
	Modules       []*Module
	IsGitHub      bool

	dir   string
	vcs   *VCS
	cache *ModuleCache
}

type Module struct {
	Path        string
	Versions    []*ModuleVersion
	Root        string
	ModFilePath string

	dir   string
	vcs   *VCS
	cache *ModuleCache
}

type ModuleVersion struct {
	Version string
	Semver  string
	Time    time.Time
}

type ModuleFetcher struct {
	cache *ModuleCache

	baseDir        string
	appId          int64
	installationId int64
	privateKeyFile string
}

func NewModuleFetcher(baseDir string, cache *ModuleCache, githubAppId, githubInstallationId int64, privateKeyFile string) *ModuleFetcher {
	return &ModuleFetcher{
		baseDir:        baseDir,
		appId:          githubAppId,
		installationId: githubInstallationId,
		privateKeyFile: privateKeyFile,
		cache:          cache,
	}
}

func (f *ModuleFetcher) Get(ctx context.Context, importPath string) (*ModuleRoot, error) {
	var repoRoot *vcs.RepoRoot
	if root, u, err := f.cache.GetRepoRoot(importPath); err == nil {
		logger.Log.Debug("RepoRoot was found in cache",
			zap.String("importPath", importPath),
			zap.String("RepoRoot", root),
			zap.String("url", u),
		)
		repoRoot = &vcs.RepoRoot{Root: root, Repo: u}
	}
	if repoRoot == nil {
		logger.Log.Debug("Not found RepoRoot in cache", zap.String("importPath", importPath))
		r, err := vcs.RepoRootForImportPath(importPath, false)
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		repoRoot = r
		if err := f.cache.SetRepoRoot(importPath, r.Root, r.Repo); err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
	}

	vcsRepo, err := NewVCS("git", repoRoot.Repo, f.appId, f.installationId, f.privateKeyFile)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	var moduleRoot *ModuleRoot
	if mr, err := f.cache.GetModuleRoot(repoRoot.Root, f.baseDir, vcsRepo); err == nil {
		logger.Log.Debug("Found ModuleRoot in cache", zap.String("repoRoot", repoRoot.Root))
		moduleRoot = mr
	}
	if moduleRoot == nil {
		logger.Log.Debug("Not found ModuleRoot in cache", zap.String("repoRoot", repoRoot.Root))
		dir := filepath.Join(f.baseDir, repoRoot.Root)
		moduleRoot, err = NewModuleRoot(ctx, repoRoot.Root, vcsRepo, f.cache, dir)
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}

		if err := moduleRoot.SetCache(); err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
	}

	return moduleRoot, nil
}

func NewModuleRoot(ctx context.Context, rootPath string, vcsRepo *VCS, cache *ModuleCache, dir string) (*ModuleRoot, error) {
	moduleRoot := &ModuleRoot{
		RootPath:      rootPath,
		RepositoryURL: vcsRepo.URL,
		IsGitHub:      strings.Contains(vcsRepo.URL, "github.com"),
		dir:           dir,
		vcs:           vcsRepo,
		cache:         cache,
	}
	modules, err := moduleRoot.findModules(ctx)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	moduleRoot.Modules = modules
	if err := moduleRoot.findVersions(); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	return moduleRoot, nil
}

func NewModuleRootFromCache(rootPath string, modules []*Module, cache *ModuleCache, vcs *VCS, dir string) *ModuleRoot {
	for _, v := range modules {
		v.dir = dir
		v.vcs = vcs
		v.cache = cache
	}
	return &ModuleRoot{
		RootPath:      rootPath,
		RepositoryURL: vcs.URL,
		Modules:       modules,
		IsGitHub:      strings.Contains(vcs.URL, "github.com"),
		dir:           dir,
		vcs:           vcs,
		cache:         cache,
	}
}

func (m *ModuleRoot) SetCache() error {
	if err := m.cache.SetModuleRoot(m); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (m *ModuleRoot) FindModule(module string) *Module {
	for _, mod := range m.Modules {
		if mod.Path == module {
			return mod
		}
	}

	return nil
}

func (m *ModuleRoot) Archive(ctx context.Context, w io.Writer, module, version string) error {
	mod := m.FindModule(module)
	if mod == nil {
		return xerrors.Errorf("%s is not found", module)
	}
	isTag := false
	versionTag := ""
	for _, v := range mod.Versions {
		if version == v.Semver {
			isTag = true
			versionTag = v.Version
			break
		}
	}
	excludeDirs := make(map[string]struct{})
	for _, v := range m.Modules {
		if v == mod {
			continue
		}
		excludeDirs[filepath.Dir(v.ModFilePath)+"/"] = struct{}{}
	}

	if isTag {
		if err := m.cache.Archive(ctx, module, version, w); err == nil {
			logger.Log.Debug("Use cache", zap.String("mod", module), zap.String("ver", version))
			return nil
		} else if err != CacheMiss {
			return xerrors.Errorf(": %w", err)
		}

		if err := m.vcs.Sync(ctx, m.dir); err != nil {
			return xerrors.Errorf(": %w", err)
		}
		tagRef, err := m.vcs.gitRepo.Tag(versionTag)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		tag, err := m.vcs.gitRepo.TagObject(tagRef.Hash())
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		tree, err := tag.Tree()
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}

		buf := new(bytes.Buffer)
		zipWriter := zip.NewWriter(buf)
		modDir := mod.Path + "@" + version
		goModFileDir := filepath.Dir(mod.ModFilePath)
		foundLicenseFile := false
		walker := object.NewTreeWalker(tree, true, make(map[plumbing.Hash]bool))
	Walk:
		for {
			name, te, err := walker.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}

			if te.Mode&filemode.Dir == filemode.Dir {
				continue Walk
			}
			for k := range excludeDirs {
				if strings.HasPrefix(name, k) {
					continue Walk
				}
			}
			if goModFileDir != "." && !strings.HasPrefix(name, goModFileDir) {
				continue Walk
			}

			if filepath.Join(filepath.Dir(mod.ModFilePath), "LICENSE") == name {
				foundLicenseFile = true
			}

			p := strings.TrimPrefix(name, filepath.Dir(mod.ModFilePath))
			fileWriter, err := zipWriter.Create(filepath.Join(modDir, p))
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
			blob, err := m.vcs.gitRepo.BlobObject(te.Hash)
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
			fileReader, err := blob.Reader()
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
			_, err = io.Copy(fileWriter, fileReader)
			if err != nil {
				return xerrors.Errorf(": %w", err)
			}
			if err := fileReader.Close(); err != nil {
				return xerrors.Errorf(": %w", err)
			}
		}

		// Find and pack LICENSE file
		if !foundLicenseFile {
			d := goModFileDir
			for {
				if _, err := tree.File(filepath.Join(d, "LICENSE")); err == object.ErrFileNotFound {
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
				f, err := tree.File(filepath.Join(d, "LICENSE"))
				if err != nil {
					return xerrors.Errorf(": %w", err)
				}
				fileReader, err := f.Reader()
				if err != nil {
					return xerrors.Errorf(": %w", err)
				}
				_, err = io.Copy(fileWriter, fileReader)
				if err != nil {
					return xerrors.Errorf(": %w", err)
				}
				if err := fileReader.Close(); err != nil {
					return xerrors.Errorf(": %w", err)
				}
				break
			}
		}

		if err := zipWriter.Close(); err != nil {
			return xerrors.Errorf(": %w", err)
		}
		data := buf.Bytes()
		if _, err := io.Copy(w, bytes.NewReader(data)); err != nil {
			return xerrors.Errorf(": %w", err)
		}
		if err := m.cache.SaveArchive(ctx, module, version, data); err != nil {
			return xerrors.Errorf(": %w", err)
		}
		return nil
	}

	return xerrors.New("specified commit is not support")
}

func (m *ModuleRoot) findModules(ctx context.Context) ([]*Module, error) {
	if err := m.vcs.Sync(ctx, m.dir); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	ref, err := m.vcs.gitRepo.Head()
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	commit, err := m.vcs.gitRepo.CommitObject(ref.Hash())
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	walker := object.NewTreeWalker(tree, true, make(map[plumbing.Hash]bool))
	var mods []*Module
	for {
		name, te, err := walker.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		// Skip directory
		if te.Mode&filemode.Dir == filemode.Dir {
			continue
		}

		if filepath.Base(name) != "go.mod" {
			continue
		}
		blob, err := m.vcs.gitRepo.BlobObject(te.Hash)
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		r, err := blob.Reader()
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		buf, err := io.ReadAll(r)
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		if err := r.Close(); err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		modFile, err := modfile.Parse(te.Name, buf, nil)
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		mods = append(mods, &Module{
			Path:        modFile.Module.Mod.Path,
			Root:        m.RootPath,
			ModFilePath: name,
			dir:         m.dir,
			vcs:         m.vcs,
			cache:       m.cache,
		})
	}

	return mods, nil
}

func (m *ModuleRoot) findVersions() error {
	if m.Modules == nil {
		return xerrors.New("should find the module first")
	}

	tags, err := m.vcs.gitRepo.Tags()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	var versions []string
	for {
		tagRef, err := tags.Next()
		if err == io.EOF {
			break
		}

		versions = append(versions, tagRef.Name().Short())
	}

	var allVer []*ModuleVersion
	for _, ver := range versions {
		sVer := ver
		s := strings.Split(ver, "/")
		if len(s) > 1 {
			sVer = s[len(s)-1]
		}
		if !semver.IsValid(sVer) {
			continue
		}

		modVer := &ModuleVersion{Version: ver, Semver: sVer}
		ref, err := m.vcs.gitRepo.Reference(plumbing.NewTagReferenceName(ver), true)
		if err == nil {
			obj, err := m.vcs.gitRepo.Object(plumbing.AnyObject, ref.Hash())
			if err == nil {
				switch v := obj.(type) {
				case *object.Tag:
					modVer.Time = v.Tagger.When.In(time.UTC)
				case *object.Commit:
					modVer.Time = v.Author.When.In(time.UTC)
				}
			} else {
				logger.Log.Debug("Failed to get tag object",
					zap.String("ver", ver),
					zap.String("hash", ref.Hash().String()),
					zap.Error(err),
				)
			}
		} else {
			logger.Log.Debug("Failed ref", zap.String("ver", ver), zap.Error(err))
		}
		if modVer.Time.IsZero() {
			logger.Log.Debug("Failed to get time", zap.String("ver", ver))
		}
		allVer = append(allVer, modVer)
	}

	for _, v := range m.Modules {
		v.setVersions(allVer)
	}

	return nil
}

func (m *Module) ModuleFile(version string) ([]byte, error) {
	isTag := false
	for _, v := range m.Versions {
		if version == v.Semver {
			isTag = true
			break
		}
	}
	if isTag {
		if buf, err := m.cache.GetModFile(m.Path, version); err == nil {
			logger.Log.Debug("Got the go.mod from cache", zap.String("path", m.Path), zap.String("version", version))
			return buf, nil
		}

		if err := m.vcs.Sync(context.Background(), m.dir); err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		tagRef, err := m.vcs.gitRepo.Tag(version)
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		tag, err := m.vcs.gitRepo.TagObject(tagRef.Hash())
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		tree, err := tag.Tree()
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		f, err := tree.File(m.ModFilePath)
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		r, err := f.Reader()
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		buf, err := io.ReadAll(r)
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		if err := r.Close(); err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		if err := m.cache.SetModFile(m.Path, version, buf); err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}

		return buf, nil
	}

	return nil, xerrors.Errorf("%s is not found", version)
}

func (m *Module) setVersions(vers []*ModuleVersion) {
	relPath := strings.TrimPrefix(m.Path, m.Root)
	if len(relPath) > 0 {
		relPath = relPath[1:]
	}

	var modVer []*ModuleVersion
	for _, ver := range vers {
		if len(relPath) > 0 && strings.HasPrefix(ver.Version, relPath) {
			modVer = append(modVer, ver)
		}
	}
	if len(modVer) == 0 {
		for _, ver := range vers {
			if !semver.IsValid(ver.Version) {
				continue
			}
			modVer = append(modVer, ver)
		}
	}
	sort.Slice(modVer, func(i, j int) bool {
		cmp := semver.Compare(modVer[i].Version, modVer[j].Version)
		if cmp != 0 {
			return cmp < 0
		}
		return modVer[i].Version < modVer[j].Version
	})
	m.Versions = modVer
}

type VCS struct {
	Type string
	URL  string

	mu     sync.Mutex
	synced bool

	appId          int64
	installationId int64
	privateKeyFile string
	transport      *ghinstallation.Transport

	gitRepo           *git.Repository
	defaultBranchName string
}

func NewVCS(typ, url string, appId, installationId int64, privateKeyFile string) (*VCS, error) {
	v := &VCS{Type: typ, URL: url, appId: appId, installationId: installationId, privateKeyFile: privateKeyFile}
	if appId > 0 && installationId > 0 && privateKeyFile != "" {
		t, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, appId, installationId, privateKeyFile)
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
		v.transport = t
	}
	return v, nil
}

func (vcs *VCS) Sync(ctx context.Context, dir string) error {
	vcs.mu.Lock()
	if vcs.synced {
		vcs.mu.Unlock()
		return nil
	}
	defer vcs.mu.Unlock()

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return xerrors.Errorf(": %w", err)
		}
		if err := vcs.Create(ctx, dir); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	} else {
		if err := vcs.Download(ctx, dir); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}

func (vcs *VCS) Create(ctx context.Context, dir string) error {
	repo, err := git.PlainCloneContext(ctx, dir, false, &git.CloneOptions{
		URL:        vcs.URL,
		NoCheckout: true,
		Auth:       vcs.getAuthMethod(),
	})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	vcs.gitRepo = repo

	return nil
}

func (vcs *VCS) Download(ctx context.Context, dir string) error {
	if err := vcs.Open(dir); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	err := vcs.gitRepo.FetchContext(ctx, &git.FetchOptions{RemoteName: "origin", Auth: vcs.getAuthMethod()})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func (vcs *VCS) getAuthMethod() *gogitHttp.BasicAuth {
	token, err := vcs.transport.Token(context.Background())
	if err != nil {
		return nil
	}
	return &gogitHttp.BasicAuth{Username: "octocat", Password: token}
}

func (vcs *VCS) Open(dir string) error {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	vcs.gitRepo = repo

	return nil
}

func (vcs *VCS) defaultBranch(ctx context.Context) (string, error) {
	if vcs.defaultBranchName != "" {
		return vcs.defaultBranchName, nil
	}

	remote, err := vcs.gitRepo.Remote("origin")
	if err != nil {
		return "", xerrors.Errorf(": %w", err)
	}
	refs, err := remote.ListContext(ctx, &git.ListOptions{})
	if err != nil {
		return "", xerrors.Errorf(": %w", err)
	}
	var headRef *plumbing.Reference
	for _, ref := range refs {
		if strings.HasPrefix(ref.Name().String(), "refs/pull") {
			continue
		}
		if ref.Name().String() == "HEAD" {
			headRef = ref
			break
		}
	}
	if headRef == nil {
		return "", xerrors.New("can not found HEAD")
	}

	vcs.defaultBranchName = headRef.Target().Short()
	return vcs.defaultBranchName, nil
}
