package git

import (
	"context"
	"errors"
	"io"

	goGit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"go.f110.dev/xerrors"
)

type gitDataService struct {
	repo map[string]*goGit.Repository
}

var _ GitDataServer = &gitDataService{}

type Repository struct {
	Name   string
	URL    string
	Prefix string
}

func NewDataService(repo map[string]*goGit.Repository) (*gitDataService, error) {
	return &gitDataService{repo: repo}, nil
}

func (g *gitDataService) ListRepositories(_ context.Context, _ *RequestListRepositories) (*ResponseListRepositories, error) {
	var list []string
	for k := range g.repo {
		list = append(list, k)
	}

	return &ResponseListRepositories{Repositories: list}, nil
}

func (g *gitDataService) ListReferences(_ context.Context, req *RequestListReferences) (*ResponseListReferences, error) {
	repo, ok := g.repo[req.Repo]
	if !ok {
		return nil, errors.New("repository not found")
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

func (g *gitDataService) GetReference(_ context.Context, req *RequestGetReference) (*ResponseGetReference, error) {
	repo, ok := g.repo[req.Repo]
	if !ok {
		return nil, errors.New("repository not found")
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

func (g *gitDataService) GetCommit(_ context.Context, req *RequestGetCommit) (*ResponseGetCommit, error) {
	repo, ok := g.repo[req.Repo]
	if !ok {
		return nil, errors.New("repository not found")
	}
	if req.Sha == "" {
		return nil, errors.New("SHA field is required")
	}

	h := plumbing.NewHash(req.Sha)
	commit, err := repo.CommitObject(h)
	if err != nil {
		return nil, err
	}

	res := &ResponseGetCommit{
		Commit: &Commit{
			Sha:     commit.Hash.String(),
			Message: commit.Message,
			Committer: &Signature{
				Name:  commit.Committer.Name,
				Email: commit.Committer.Email,
			},
			Author: &Signature{
				Name:  commit.Author.Name,
				Email: commit.Author.Email,
			},
			Tree: commit.TreeHash.String(),
		},
	}
	if len(commit.ParentHashes) > 0 {
		parents := make([]string, len(commit.ParentHashes))
		for i := 0; i < len(commit.ParentHashes); i++ {
			parents[i] = commit.ParentHashes[i].String()
		}
		res.Commit.Parents = parents
	}

	return res, nil
}

func (g *gitDataService) GetTree(_ context.Context, req *RequestGetTree) (*ResponseGetTree, error) {
	repo, ok := g.repo[req.Repo]
	if !ok {
		return nil, errors.New("repository not found")
	}
	commit, err := repo.CommitObject(plumbing.NewHash(req.Sha))
	if err != nil {
		return nil, xerrors.Newf("commit object not found: %v", err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}

	var treeEntry []*TreeEntry
	walker := object.NewTreeWalker(tree, true, nil)
	for {
		_, e, err := walker.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		treeEntry = append(treeEntry, &TreeEntry{
			Sha:  e.Hash.String(),
			Path: e.Name,
		})
	}
	return &ResponseGetTree{Sha: req.Sha, Tree: treeEntry}, nil
}

func (g *gitDataService) GetBlob(_ context.Context, req *RequestGetBlob) (*ResponseGetBlob, error) {
	repo, ok := g.repo[req.Repo]
	if !ok {
		return nil, errors.New("repository not found")
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

func (g *gitDataService) ListTag(_ context.Context, req *RequestListTag) (*ResponseListTag, error) {
	repo, ok := g.repo[req.Repo]
	if !ok {
		return nil, errors.New("repository not found")
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

func (g *gitDataService) ListBranch(_ context.Context, req *RequestListBranch) (*ResponseListBranch, error) {
	repo, ok := g.repo[req.Repo]
	if !ok {
		return nil, errors.New("repository not found")
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
