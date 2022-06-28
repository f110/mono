package main

import (
	"context"
	"errors"
	"io"

	goGit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"go.f110.dev/mono/go/pkg/git"
)

type gitDataService struct {
	repo map[string]*goGit.Repository
}

var _ git.GitDataServer = &gitDataService{}

type repository struct {
	Name   string
	URL    string
	Prefix string
}

func newService(repo map[string]*goGit.Repository) (*gitDataService, error) {
	return &gitDataService{repo: repo}, nil
}

func (g *gitDataService) ListRepositories(_ context.Context, _ *git.RequestListRepositories) (*git.ResponseListRepositories, error) {
	var list []string
	for k := range g.repo {
		list = append(list, k)
	}

	return &git.ResponseListRepositories{Repositories: list}, nil
}

func (g *gitDataService) ListReferences(_ context.Context, req *git.RequestListReferences) (*git.ResponseListReferences, error) {
	repo, ok := g.repo[req.Repo]
	if !ok {
		return nil, errors.New("repository not found")
	}

	refs, err := repo.References()
	if err != nil {
		return nil, err
	}

	res := &git.ResponseListReferences{}
	for {
		ref, err := refs.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		res.Refs = append(res.Refs, &git.Reference{
			Name:   ref.Name().String(),
			Hash:   ref.Hash().String(),
			Target: ref.Target().String(),
		})
	}
	return res, nil
}

func (g *gitDataService) GetCommit(_ context.Context, req *git.RequestGetCommit) (*git.ResponseGetCommit, error) {
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

	res := &git.ResponseGetCommit{
		Commit: &git.Commit{
			Sha:     commit.Hash.String(),
			Message: commit.Message,
			Committer: &git.Signature{
				Name:  commit.Committer.Name,
				Email: commit.Committer.Email,
			},
			Author: &git.Signature{
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

func (g *gitDataService) GetTree(_ context.Context, req *git.RequestGetTree) (*git.ResponseGetTree, error) {
	repo, ok := g.repo[req.Repo]
	if !ok {
		return nil, errors.New("repository not found")
	}
	commit, err := repo.CommitObject(plumbing.NewHash(req.Sha))
	if err != nil {
		return nil, err
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}

	var treeEntry []*git.TreeEntry
	iter := tree.Files()
	for {
		f, err := iter.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		treeEntry = append(treeEntry, &git.TreeEntry{
			Sha:  f.Hash.String(),
			Path: f.Name,
			Size: f.Size,
		})
	}
	return &git.ResponseGetTree{Sha: req.Sha, Tree: treeEntry}, nil
}

func (g *gitDataService) GetBlob(_ context.Context, req *git.RequestGetBlob) (*git.ResponseGetBlob, error) {
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

	return &git.ResponseGetBlob{
		Sha:     req.Sha,
		Size:    blob.Size,
		Content: buf,
	}, nil
}

func (g *gitDataService) ListTag(_ context.Context, req *git.RequestListTag) (*git.ResponseListTag, error) {
	repo, ok := g.repo[req.Repo]
	if !ok {
		return nil, errors.New("repository not found")
	}
	iter, err := repo.Tags()
	if err != nil {
		return nil, err
	}

	res := &git.ResponseListTag{}
	for {
		ref, err := iter.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		res.Tags = append(res.Tags, &git.Reference{
			Name:   ref.Name().String(),
			Target: ref.Target().String(),
			Hash:   ref.Hash().String(),
		})
	}

	return res, nil
}
