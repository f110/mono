package git

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	goGit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"go.f110.dev/xerrors"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.f110.dev/mono/go/enumerable"
	githubutil "go.f110.dev/mono/go/github"
)

// DataService serves the repositories which are provided by RepositoryProvider.
// The repositories are fetched from the provider when the requested repository is not known,
// and are opened when they are used.
type DataService struct {
	repos         RepositoryProvider
	backend       Backend
	tokenProvider *githubutil.TokenProvider
	initTimeout   time.Duration

	mu   sync.RWMutex
	repo map[string]*RepositoryConfig
}

var _ GitDataServer = &DataService{}

func NewDataService(backend Backend, tokenProvider *githubutil.TokenProvider, repos RepositoryProvider) (*DataService, error) {
	return &DataService{
		repos:         repos,
		backend:       backend,
		tokenProvider: tokenProvider,
		initTimeout:   5 * time.Minute,
		repo:          make(map[string]*RepositoryConfig),
	}, nil
}

func NewDataServiceWithGoGit(repo map[string]*goGit.Repository) (*DataService, error) {
	repos := make([]*RepositoryConfig, 0, len(repo))
	for k, v := range repo {
		repos = append(repos, &RepositoryConfig{Name: k, goGit: v})
	}
	return NewDataService(nil, nil, NewStaticRepositoryProvider(repos))
}

func (g *DataService) SetInitTimeout(d time.Duration) *DataService {
	g.initTimeout = d
	return g
}

// refresh replaces the known repositories with the ones from the provider.
func (g *DataService) refresh(ctx context.Context) ([]*RepositoryConfig, error) {
	repos, err := g.repos.Repositories(ctx)
	if err != nil {
		return nil, err
	}
	m := make(map[string]*RepositoryConfig, len(repos))
	for _, v := range repos {
		m[v.Name] = v
	}
	g.mu.Lock()
	g.repo = m
	g.mu.Unlock()
	return repos, nil
}

func (g *DataService) openRepository(ctx context.Context, repo *RepositoryConfig) (*goGit.Repository, error) {
	if err := repo.Open(ctx, g.backend, g.tokenProvider, g.initTimeout); err != nil {
		return nil, err
	}
	return repo.goGit, nil
}

// lookup returns the opened repository. The second return value is false if the provider doesn't provide the repository.
func (g *DataService) lookup(ctx context.Context, name string) (*goGit.Repository, bool, error) {
	g.mu.RLock()
	repo, ok := g.repo[name]
	g.mu.RUnlock()
	if !ok {
		repos, err := g.refresh(ctx)
		if err != nil {
			return nil, false, err
		}
		for _, v := range repos {
			if v.Name == name {
				repo, ok = v, true
				break
			}
		}
		if !ok {
			return nil, false, nil
		}
	}

	r, err := g.openRepository(ctx, repo)
	if err != nil {
		return nil, false, err
	}
	return r, true, nil
}

func (g *DataService) ListRepositories(ctx context.Context, _ *RequestListRepositories) (*ResponseListRepositories, error) {
	repos, err := g.refresh(ctx)
	if err != nil {
		return nil, err
	}
	var list []*Repository
	for _, repo := range repos {
		v, err := g.openRepository(ctx, repo)
		if err != nil {
			return nil, err
		}
		headRef, err := v.Head()
		if err != nil {
			return nil, err
		}
		remote, err := v.Remote("origin")
		if err != nil {
			return nil, err
		}
		remoteURL, err := url.Parse(remote.Config().URLs[0])
		if err != nil {
			return nil, err
		}
		var repoURL, gitURL string
		switch remoteURL.Host {
		case "github.com":
			repoURL = strings.TrimSuffix(remoteURL.String(), ".git")
			gitURL = remoteURL.String()
		}

		list = append(list, &Repository{
			Name:          repo.Name,
			DefaultBranch: headRef.Name().Short(),
			Url:           repoURL,
			GitUrl:        gitURL,
		})
	}
	// Sort by name so the order is stable regardless of map iteration order.
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })

	return &ResponseListRepositories{Repositories: list}, nil
}

func (g *DataService) ListReferences(ctx context.Context, req *RequestListReferences) (*ResponseListReferences, error) {
	repo, ok, err := g.lookup(ctx, req.Repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, xerrors.New("repository not found")
	}

	refs, err := repo.References()
	if err != nil {
		return nil, err
	}

	res := &ResponseListReferences{}
	for {
		ref, err := refs.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		res.Refs = append(res.Refs, &Reference{
			Name:   ref.Name().String(),
			Hash:   ref.Hash().String(),
			Target: ref.Target().String(),
		})
	}
	return res, nil
}

func (g *DataService) GetRepository(ctx context.Context, req *RequestGetRepository) (*ResponseGetRepository, error) {
	repo, ok, err := g.lookup(ctx, req.Repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, xerrors.New("repository not found")
	}

	r, err := repo.Remote("origin")
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(r.Config().URLs[0])
	if err != nil {
		return nil, err
	}
	var hosting string
	switch u.Host {
	case "github.com":
		hosting = "github"
	}

	return &ResponseGetRepository{
		Name:    req.Repo,
		Url:     r.Config().URLs[0],
		Hosting: hosting,
	}, nil
}

func (g *DataService) GetReference(ctx context.Context, req *RequestGetReference) (*ResponseGetReference, error) {
	repo, ok, err := g.lookup(ctx, req.Repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, xerrors.New("repository not found")
	}

	ref, err := repo.Reference(plumbing.ReferenceName(req.Ref), false)
	if err != nil {
		return nil, err
	}

	return &ResponseGetReference{
		Ref: &Reference{
			Name:   ref.Name().String(),
			Hash:   ref.Hash().String(),
			Target: ref.Target().String(),
		},
	}, nil
}

func (g *DataService) GetCommit(ctx context.Context, req *RequestGetCommit) (*ResponseGetCommit, error) {
	repo, ok, err := g.lookup(ctx, req.Repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, status.Error(codes.NotFound, "repository not found")
	}
	if req.Sha == "" && req.Ref == "" {
		return nil, xerrors.New("SHA or ref field is required")
	}

	var h plumbing.Hash
	if req.Ref != "" {
		ref, err := repo.Reference(plumbing.ReferenceName(req.Ref), false)
		if err != nil {
			return nil, err
		}
		h = ref.Hash()
	} else {
		h = plumbing.NewHash(req.Sha)
	}
	commit, err := repo.CommitObject(h)
	if errors.Is(err, plumbing.ErrObjectNotFound) {
		return nil, status.Error(codes.NotFound, "commit not found")
	}
	if err != nil {
		return nil, err
	}

	return &ResponseGetCommit{Commit: commitToProto(commit)}, nil
}

func commitToProto(commit *object.Commit) *Commit {
	c := &Commit{
		Sha:     commit.Hash.String(),
		Message: commit.Message,
		Committer: &Signature{
			Name:  commit.Committer.Name,
			Email: commit.Committer.Email,
			When:  timestamppb.New(commit.Committer.When),
		},
		Author: &Signature{
			Name:  commit.Author.Name,
			Email: commit.Author.Email,
			When:  timestamppb.New(commit.Author.When),
		},
		Tree: commit.TreeHash.String(),
	}
	if len(commit.ParentHashes) > 0 {
		c.Parents = enumerable.Map(commit.ParentHashes, func(t plumbing.Hash) string { return t.String() })
	}
	for _, h := range commit.ExtraHeaders {
		if h.Key == "change-id" {
			c.ChangeId = h.Value
			break
		}
	}
	return c
}

func (g *DataService) GetTree(ctx context.Context, req *RequestGetTree) (*ResponseGetTree, error) {
	repo, ok, err := g.lookup(ctx, req.Repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, xerrors.New("repository not found")
	}

	var tree *object.Tree
	if req.Sha != "" {
		t, err := repo.TreeObject(plumbing.NewHash(req.Sha))
		if err != nil {
			return nil, err
		}
		tree = t
	}
	if req.Ref != "" {
		var commitHash plumbing.Hash
		if plumbing.IsHash(req.Ref) {
			commitHash = plumbing.NewHash(req.Ref)
		} else {
			ref, err := repo.Reference(plumbing.ReferenceName(req.Ref), false)
			if err != nil {
				return nil, xerrors.New("ref is not found")
			}
			commitHash = ref.Hash()
		}
		commit, err := repo.CommitObject(commitHash)
		if err != nil {
			return nil, xerrors.New("commit is not found")
		}
		t, err := commit.Tree()
		if err != nil {
			return nil, err
		}
		tree = t
	}
	var pathTree *object.Tree
	if req.Path != "/" {
		t, err := tree.Tree(req.Path)
		if err != nil {
			return nil, xerrors.New("path is not found")
		}
		if t == nil {
			return nil, xerrors.New("tree object is not found")
		}
		pathTree = t
	} else {
		pathTree = tree
	}

	var treeEntry []*TreeEntry
	walker := object.NewTreeWalker(pathTree, req.Recursive, nil)
	for {
		name, e, err := walker.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		treeEntry = append(treeEntry, &TreeEntry{
			Sha:  e.Hash.String(),
			Path: name,
			Mode: e.Mode.String(),
		})
	}
	return &ResponseGetTree{Sha: tree.Hash.String(), Tree: treeEntry}, nil
}

func (g *DataService) GetBlob(ctx context.Context, req *RequestGetBlob) (*ResponseGetBlob, error) {
	repo, ok, err := g.lookup(ctx, req.Repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, xerrors.New("repository not found")
	}
	blob, err := repo.BlobObject(plumbing.NewHash(req.Sha))
	if err != nil {
		return nil, err
	}

	r, err := blob.Reader()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return &ResponseGetBlob{
		Sha:     req.Sha,
		Size:    blob.Size,
		Content: buf,
	}, nil
}

func (g *DataService) GetFile(ctx context.Context, req *RequestGetFile) (*ResponseGetFile, error) {
	repo, ok, err := g.lookup(ctx, req.Repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, xerrors.New("repository not found")
	}

	var commitHash plumbing.Hash
	if plumbing.IsHash(req.Ref) {
		commitHash = plumbing.NewHash(req.Ref)
	} else {
		ref, err := repo.Reference(plumbing.ReferenceName(req.Ref), false)
		if err != nil {
			return nil, xerrors.New("ref is not found")
		}
		commitHash = ref.Hash()
	}

	commit, err := repo.CommitObject(commitHash)
	if err != nil {
		return nil, xerrors.New("commit is not found")
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to get tree")
	}
	var treeEntry *object.TreeEntry
	if req.Path != "/" {
		te, err := tree.FindEntry(req.Path)
		if err != nil {
			return nil, xerrors.WithMessagef(err, "could not find the tree entry %s in %s", req.Path, tree.Hash.String())
		}
		treeEntry = te
	} else {
		// "/" is root directory
		d := &errdetails.BadRequest{
			FieldViolations: []*errdetails.BadRequest_FieldViolation{
				{Field: "path", Description: "path is directory"},
			},
		}
		st := status.New(codes.InvalidArgument, "path is directory")
		if rpcErr, err := st.WithDetails(d); err != nil {
			return nil, status.Error(codes.Internal, "")
		} else {
			return nil, rpcErr.Err()
		}
	}

	switch treeEntry.Mode {
	case filemode.Regular, filemode.Executable:
		blob, err := repo.BlobObject(treeEntry.Hash)
		if err != nil {
			return nil, xerrors.WithMessage(err, "failed to get blob")
		}
		r, err := blob.Reader()
		if err != nil {
			return nil, xerrors.WithMessage(err, "failed to get file reader")
		}
		defer r.Close()
		buf, err := io.ReadAll(r)
		if err != nil {
			return nil, xerrors.WithMessage(err, "failed to read file")
		}

		var rawURL, editURL string
		remote, err := repo.Remote("origin")
		if err != nil {
			return nil, xerrors.WithMessage(err, "failed to get remote")
		}
		u, err := url.Parse(remote.Config().URLs[0])
		if err != nil {
			return nil, xerrors.WithMessagef(err, "invalid remote url %s", remote.Config().URLs[0])
		}
		switch u.Host {
		case "github.com":
			s := strings.Split(u.Path, "/")
			owner, repoName := s[1], s[2]
			repoName = strings.TrimSuffix(repoName, ".git")
			rawURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repoName, req.Ref, req.Path)

			editURL = fmt.Sprintf("https://github.com/%s/%s/edit/%s/%s", owner, repoName, req.Ref, req.Path)
		}

		return &ResponseGetFile{Content: buf, RawUrl: rawURL, EditUrl: editURL, Sha: blob.Hash.String()}, nil
	case filemode.Dir:
		d := &errdetails.BadRequest{
			FieldViolations: []*errdetails.BadRequest_FieldViolation{
				{Field: "path", Description: "path is directory"},
			},
		}
		st := status.New(codes.InvalidArgument, "path is directory")
		if rpcErr, err := st.WithDetails(d); err != nil {
			return nil, status.Error(codes.Internal, "")
		} else {
			return nil, rpcErr.Err()
		}
	}

	return nil, xerrors.Newf("unsupported object: %s", treeEntry.Mode.String())
}

func (g *DataService) Stat(ctx context.Context, req *RequestStat) (*ResponseStat, error) {
	repo, ok, err := g.lookup(ctx, req.Repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, xerrors.New("repository not found")
	}

	var commitHash plumbing.Hash
	if plumbing.IsHash(req.Ref) {
		commitHash = plumbing.NewHash(req.Ref)
	} else {
		ref, err := repo.Reference(plumbing.ReferenceName(req.Ref), false)
		if err != nil {
			return nil, xerrors.New("ref is not found")
		}
		commitHash = ref.Hash()
	}

	commit, err := repo.CommitObject(commitHash)
	if err != nil {
		return nil, xerrors.New("commit is not found")
	}
	if req.Path == "" || req.Path == "/" {
		return &ResponseStat{Hash: commit.TreeHash.String(), Mode: uint32(filemode.Dir)}, nil
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to get tree")
	}
	if req.Path[0] == '/' {
		req.Path = req.Path[1:]
	}
	if req.Path[len(req.Path)-1] == '/' {
		req.Path = req.Path[:len(req.Path)-1]
	}
	treeEntry, err := tree.FindEntry(req.Path)
	if err != nil {
		return nil, xerrors.WithMessagef(err, "could not find the tree entry %s in %s", req.Path, tree.Hash.String())
	}

	return &ResponseStat{Name: path.Join(path.Dir(req.Path), treeEntry.Name), Hash: treeEntry.Hash.String(), Mode: uint32(treeEntry.Mode)}, nil
}

func (g *DataService) GetRepositoryStatistics(ctx context.Context, req *RequestGetRepositoryStatistics) (*ResponseGetRepositoryStatistics, error) {
	repo, ok, err := g.lookup(ctx, req.Repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, xerrors.New("repository not found")
	}

	head, err := repo.Head()
	if err != nil {
		return nil, err
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return nil, err
	}

	return &ResponseGetRepositoryStatistics{HeadCommit: commitToProto(commit)}, nil
}

func (g *DataService) ListTag(ctx context.Context, req *RequestListTag) (*ResponseListTag, error) {
	repo, ok, err := g.lookup(ctx, req.Repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, xerrors.New("repository not found")
	}
	iter, err := repo.Tags()
	if err != nil {
		return nil, err
	}

	res := &ResponseListTag{}
	for {
		ref, err := iter.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		res.Tags = append(res.Tags, &Reference{
			Name:   ref.Name().String(),
			Target: ref.Target().String(),
			Hash:   ref.Hash().String(),
		})
	}

	return res, nil
}

func (g *DataService) ListBranch(ctx context.Context, req *RequestListBranch) (*ResponseListBranch, error) {
	repo, ok, err := g.lookup(ctx, req.Repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, xerrors.New("repository not found")
	}
	iter, err := repo.Branches()
	if err != nil {
		return nil, err
	}

	res := &ResponseListBranch{}
	for {
		ref, err := iter.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		res.Branches = append(res.Branches, &Reference{
			Name:   ref.Name().String(),
			Target: ref.Target().String(),
			Hash:   ref.Hash().String(),
		})
	}

	return res, nil
}
