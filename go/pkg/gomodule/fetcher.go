package gomodule

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"go.uber.org/zap"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
	"golang.org/x/tools/go/vcs"
	"golang.org/x/xerrors"

	"go.f110.dev/mono/go/pkg/logger"
)

type ModuleRoot struct {
	RootPath string
	Modules  []*Module

	dir string
	vcs *VCS
}

type Module struct {
	Path     string
	Versions []*ModuleVersion
	Root     string

	modFilePath string
	dir         string
	vcs         *VCS
}

type ModuleVersion struct {
	Version string
	Semver  string
	Time    time.Time
}

type ModuleFetcher struct {
	baseDir string
}

func NewModuleFetcher(baseDir string) *ModuleFetcher {
	return &ModuleFetcher{baseDir: baseDir}
}

func (f *ModuleFetcher) Fetch(ctx context.Context, importPath string) (*ModuleRoot, error) {
	repoRoot, err := vcs.RepoRootForImportPath(importPath, false)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	dir := filepath.Join(f.baseDir, repoRoot.Root)
	vcsRepo := NewVCS("git", repoRoot.Repo)
	if err := f.updateOrCreate(ctx, vcsRepo, dir); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	moduleRoot := NewModuleRoot(repoRoot, vcsRepo, dir)
	modules, err := moduleRoot.findModules()
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	moduleRoot.Modules = modules

	if err := moduleRoot.findVersions(); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	return moduleRoot, nil
}

func (f *ModuleFetcher) updateOrCreate(ctx context.Context, repo *VCS, dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return xerrors.Errorf(": %w", err)
		}
		if err := repo.Create(ctx, dir); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	} else {
		if err := repo.Download(ctx, dir); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}

func NewModuleRoot(repoRoot *vcs.RepoRoot, vcsRepo *VCS, dir string) *ModuleRoot {
	return &ModuleRoot{
		RootPath: repoRoot.Root,
		dir:      dir,
		vcs:      vcsRepo,
	}
}

func (m *ModuleRoot) Archive(w io.Writer, module, version string) error {
	var mod *Module
	for _, v := range m.Modules {
		if v.Path == module {
			mod = v
			break
		}
	}
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
		excludeDirs[filepath.Dir(v.modFilePath)+"/"] = struct{}{}
	}

	if isTag {
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

		zipWriter := zip.NewWriter(w)
		modDir := mod.Path + "@" + version
		goModFileDir := filepath.Dir(mod.modFilePath)
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

			if filepath.Join(filepath.Dir(mod.modFilePath), "LICENSE") == name {
				foundLicenseFile = true
			}

			p := strings.TrimPrefix(name, filepath.Dir(mod.modFilePath))
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
		return nil
	}

	return xerrors.New("specified commit is not support")
}

func (m *ModuleRoot) findModules() ([]*Module, error) {
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
			modFilePath: name,
			dir:         m.dir,
			vcs:         m.vcs,
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
		f, err := tree.File(m.modFilePath)
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

		return buf, nil
	}

	return nil, xerrors.New("specified commit is not supported")
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

	gitRepo           *git.Repository
	defaultBranchName string
}

func NewVCS(typ, url string) *VCS {
	return &VCS{Type: typ, URL: url}
}

func (vcs *VCS) Create(ctx context.Context, dir string) error {
	repo, err := git.PlainCloneContext(ctx, dir, false, &git.CloneOptions{
		URL:        vcs.URL,
		NoCheckout: true,
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
	err := vcs.gitRepo.FetchContext(ctx, &git.FetchOptions{RemoteName: "origin"})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return xerrors.Errorf(": %w", err)
	}

	return nil
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
