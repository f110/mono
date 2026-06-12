package config

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/google/go-github/v85/github"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/git"
	"go.f110.dev/mono/go/logger/slogger"
)

const (
	BuildFileDir     = ".build"
	BazelVersionFile = ".bazelversion"
)

func ReadFromRepository(ctx context.Context, githubClient *github.Client, owner, repoName string) (*Config, error) {
	slogger.Log.Debug("GetCommit", slog.String("owner", owner), slog.String("repo", repoName))
	commit, _, err := githubClient.Repositories.GetCommit(ctx, owner, repoName, "HEAD", nil)
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to get HEAD commit")
	}
	return ReadFromSpecifiedCommit(ctx, githubClient, owner, repoName, commit.GetSHA())
}

func ReadFromSpecifiedCommit(ctx context.Context, githubClient *github.Client, owner, repoName string, sha string) (*Config, error) {
	provider, err := newGitHubProvider(ctx, githubClient, owner, repoName, sha)
	if err != nil {
		return nil, err
	}
	return readFromProvider(provider, owner, repoName)
}

// ReadFromGitDataService reads the build configuration via the git-data-service
// gRPC client. repoName must match the repository name registered with the
// service. ref may be a commit SHA or a fully-qualified ref name
// (e.g. "refs/heads/master").
func ReadFromGitDataService(ctx context.Context, client git.GitDataClient, owner, repoName, ref string) (*Config, error) {
	provider := newGitDataServiceProvider(ctx, client, repoName, ref)
	return readFromProvider(provider, owner, repoName)
}

func readFromProvider(provider Provider, owner, repoName string) (*Config, error) {
	jobs, err := ReadJobsFromBuildDir(provider)
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to read build dir")
	}
	bazelVersion, err := ReadBazelVersion(provider)
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to read .bazelversion")
	}
	for _, j := range jobs {
		j.RepositoryOwner = owner
		j.RepositoryName = repoName
	}
	return &Config{RepositoryOwner: owner, RepositoryName: repoName, Jobs: jobs, BazelVersion: bazelVersion}, nil
}

type Provider interface {
	fs.ReadDirFS
}

type localProvider struct {
	fs.ReadDirFS
}

func NewLocalProvider(dir string) Provider {
	fsys, ok := os.DirFS(dir).(fs.ReadDirFS)
	if !ok {
		return nil
	}
	return &localProvider{ReadDirFS: fsys}
}

type entry struct {
	name string
	sha  string
}

type githubProvider struct {
	ctx          context.Context
	githubClient *github.Client
	owner        string
	name         string
	sha          string
}

func newGitHubProvider(ctx context.Context, githubClient *github.Client, owner, repoName string, sha string) (Provider, error) {
	return &githubProvider{
		ctx:          ctx,
		githubClient: githubClient,
		owner:        owner,
		name:         repoName,
		sha:          sha,
	}, nil
}

func (p *githubProvider) ReadDir(path string) ([]fs.DirEntry, error) {
	path = filepath.Clean(path)
	if path[0] == '/' {
		path = path[1:]
	}

	sha := p.sha
	var entries []*github.TreeEntry
GetTree:
	for sha != "" {
		slogger.Log.Debug("GetTree", slog.String("sha", sha))
		tree, _, err := p.githubClient.Git.GetTree(p.ctx, p.owner, p.name, sha, false)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		if path == "" {
			entries = tree.Entries
			break
		}

		s := strings.Split(path, "/")
		if len(s) > 0 {
			dir := s[0]
			for _, entry := range tree.Entries {
				if entry.GetPath() == dir {
					sha = entry.GetSHA()
					path = strings.Join(s[1:], "/")
					continue GetTree
				}
			}
			return nil, fs.ErrNotExist
		}
	}
	if entries == nil {
		return nil, fs.ErrNotExist
	}

	var dirEntries []fs.DirEntry
	for _, entry := range entries {
		var mode fs.FileMode
		switch entry.GetType() {
		case "tree":
			mode = os.ModeDir
		case "blob":
		}
		dirEntries = append(dirEntries, &githubEntry{
			name:  entry.GetPath(),
			isDir: entry.GetType() == "tree",
			mode:  mode,
			size:  int64(entry.GetSize()),
		})
	}
	return dirEntries, nil
}

func (p *githubProvider) Open(name string) (fs.File, error) {
	sha := p.sha
GetTree:
	for sha != "" {
		tree, _, err := p.githubClient.Git.GetTree(p.ctx, p.owner, p.name, sha, false)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		sha = ""

		s := strings.Split(name, "/")
		if len(s) == 1 {
			// Last element
			for _, entry := range tree.Entries {
				if entry.GetPath() == s[0] {
					blob, _, err := p.githubClient.Git.GetBlobRaw(p.ctx, p.owner, p.name, entry.GetSHA())
					if err != nil {
						return nil, xerrors.WithStack(err)
					}
					return &githubFile{buf: bytes.NewReader(blob)}, nil
				}
			}
			return nil, fs.ErrNotExist
		} else if len(s) > 0 {
			dir := s[0]
			for _, entry := range tree.Entries {
				if entry.GetPath() == dir {
					sha = entry.GetSHA()
					name = strings.Join(s[1:], "/")
					continue GetTree
				}
			}
		}
	}
	return nil, fs.ErrNotExist
}

type githubEntry struct {
	name  string
	isDir bool
	mode  fs.FileMode
	size  int64
}

func (e *githubEntry) Name() string {
	return e.name
}

func (e *githubEntry) IsDir() bool {
	return e.isDir
}

func (e *githubEntry) Type() fs.FileMode {
	return e.mode
}

func (e *githubEntry) Info() (fs.FileInfo, error) {
	return e, nil
}

func (e *githubEntry) Size() int64 {
	return e.size
}

func (e *githubEntry) Mode() fs.FileMode {
	return e.mode
}

func (e *githubEntry) ModTime() time.Time {
	return time.Time{}
}

func (e *githubEntry) Sys() any {
	return nil
}

type githubFile struct {
	buf io.Reader
}

func (f *githubFile) Stat() (fs.FileInfo, error) { return nil, nil }
func (f *githubFile) Read(v []byte) (int, error) { return f.buf.Read(v) }
func (f *githubFile) Close() error               { return nil }

type gitDataServiceProvider struct {
	ctx    context.Context
	client git.GitDataClient
	repo   string
	ref    string
}

func newGitDataServiceProvider(ctx context.Context, client git.GitDataClient, repo, ref string) Provider {
	return &gitDataServiceProvider{ctx: ctx, client: client, repo: repo, ref: ref}
}

func (p *gitDataServiceProvider) ReadDir(name string) ([]fs.DirEntry, error) {
	pathStr := normalizeGitPath(name)
	if pathStr == "" {
		pathStr = "/"
	}
	resp, err := p.client.GetTree(p.ctx, &git.RequestGetTree{Repo: p.repo, Ref: p.ref, Path: pathStr})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	entries := make([]fs.DirEntry, 0, len(resp.GetTree()))
	for _, e := range resp.GetTree() {
		entries = append(entries, &gitDataServiceEntry{
			name:  path.Base(e.GetPath()),
			isDir: e.GetMode() == "0040000",
			size:  e.GetSize(),
		})
	}
	return entries, nil
}

func (p *gitDataServiceProvider) Open(name string) (fs.File, error) {
	pathStr := normalizeGitPath(name)
	if pathStr == "" {
		return nil, fs.ErrNotExist
	}
	resp, err := p.client.GetFile(p.ctx, &git.RequestGetFile{Repo: p.repo, Ref: p.ref, Path: pathStr})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	return &githubFile{buf: bytes.NewReader(resp.GetContent())}, nil
}

func normalizeGitPath(name string) string {
	name = filepath.Clean(name)
	if name == "." {
		return ""
	}
	if name == "" || name[0] != '/' {
		name = "/" + name
	}
	return name
}

type gitDataServiceEntry struct {
	name  string
	isDir bool
	size  int64
}

func (e *gitDataServiceEntry) Name() string               { return e.name }
func (e *gitDataServiceEntry) IsDir() bool                { return e.isDir }
func (e *gitDataServiceEntry) Type() fs.FileMode          { return 0 }
func (e *gitDataServiceEntry) Info() (fs.FileInfo, error) { return e, nil }
func (e *gitDataServiceEntry) Size() int64                { return e.size }
func (e *gitDataServiceEntry) Mode() fs.FileMode {
	if e.isDir {
		return fs.ModeDir
	}
	return 0
}
func (e *gitDataServiceEntry) ModTime() time.Time { return time.Time{} }
func (e *gitDataServiceEntry) Sys() any           { return nil }

func ReadJobsFromBuildDir(fileProvider Provider) ([]*JobV2, error) {
	entries, err := fileProvider.ReadDir(BuildFileDir)
	if err != nil {
		return nil, err
	}
	var allJobs []*JobV2
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".cue" {
			continue
		}
		f, err := fileProvider.Open(filepath.Join(BuildFileDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		jobs, err := ParseFile(f)
		if err != nil {
			f.Close()
			return nil, err
		}
		allJobs = append(allJobs, jobs...)
		f.Close()
	}

	return allJobs, nil
}

func ReadBazelVersion(fileProvider Provider) (string, error) {
	f, err := fileProvider.Open(BazelVersionFile)
	if err != nil {
		return "", err
	}
	defer f.Close()
	buf, err := io.ReadAll(f)
	if err != nil {
		return "", xerrors.WithStack(err)
	}
	return strings.TrimRight(string(buf), "\n"), nil
}

func ParseFile(f fs.File) ([]*JobV2, error) {
	cueCtx := cuecontext.New()
	cueSchema := cueCtx.CompileBytes(schema)

	buf, err := io.ReadAll(f)
	if err != nil {
		f.Close()
		return nil, xerrors.WithStack(err)
	}
	f.Close()

	rawConf := cueCtx.CompileBytes(buf)
	if rawConf.Err() != nil {
		return nil, xerrors.WithStack(rawConf.Err())
	}
	parsed := cueSchema.Unify(rawConf)
	if parsed.Err() != nil {
		return nil, xerrors.WithStack(parsed.Err())
	}
	if err := parsed.Validate(cue.Concrete(true)); err != nil {
		return nil, xerrors.WithStack(err)
	}
	parsed = parsed.LookupPath(cue.ParsePath("output"))
	if parsed.Err() != nil {
		return nil, xerrors.WithStack(parsed.Err())
	}
	var jobs []*JobV2
	if err := parsed.Decode(&jobs); err != nil {
		return nil, xerrors.WithStack(err)
	}
	return jobs, nil
}
