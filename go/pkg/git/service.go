package git

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	goGit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"go.f110.dev/xerrors"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type DataService struct {
	repo map[string]*goGit.Repository
}

var _ GitDataServer = &DataService{}

func NewDataService(repo map[string]*goGit.Repository) (*DataService, error) {
	return &DataService{repo: repo}, nil
}

func (g *DataService) ListRepositories(_ context.Context, _ *RequestListRepositories) (*ResponseListRepositories, error) {
	var list []*Repository
	for k, v := range g.repo {
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
			Name:          k,
			DefaultBranch: headRef.Name().Short(),
			Url:           repoURL,
			GitUrl:        gitURL,
		})
	}

	return &ResponseListRepositories{Repositories: list}, nil
}

func (g *DataService) ListReferences(_ context.Context, req *RequestListReferences) (*ResponseListReferences, error) {
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

func (g *DataService) GetRepository(_ context.Context, req *RequestGetRepository) (*ResponseGetRepository, error) {
	repo, ok := g.repo[req.Repo]
	if !ok {
		return nil, errors.New("repository not found")
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

func (g *DataService) GetReference(_ context.Context, req *RequestGetReference) (*ResponseGetReference, error) {
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

func (g *DataService) GetCommit(_ context.Context, req *RequestGetCommit) (*ResponseGetCommit, error) {
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
				When:  timestamppb.New(commit.Committer.When),
			},
			Author: &Signature{
				Name:  commit.Author.Name,
				Email: commit.Author.Email,
				When:  timestamppb.New(commit.Author.When),
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

func (g *DataService) GetTree(_ context.Context, req *RequestGetTree) (*ResponseGetTree, error) {
	repo, ok := g.repo[req.Repo]
	if !ok {
		return nil, errors.New("repository not found")
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
				return nil, errors.New("ref is not found")
			}
			commitHash = ref.Hash()
		}
		commit, err := repo.CommitObject(commitHash)
		if err != nil {
			return nil, errors.New("commit is not found")
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
			return nil, errors.New("path is not found")
		}
		if t == nil {
			return nil, errors.New("tree object is not found")
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

func (g *DataService) GetBlob(_ context.Context, req *RequestGetBlob) (*ResponseGetBlob, error) {
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

func (g *DataService) GetFile(_ context.Context, req *RequestGetFile) (*ResponseGetFile, error) {
	repo, ok := g.repo[req.Repo]
	if !ok {
		return nil, errors.New("repository not found")
	}

	var commitHash plumbing.Hash
	if plumbing.IsHash(req.Ref) {
		commitHash = plumbing.NewHash(req.Ref)
	} else {
		ref, err := repo.Reference(plumbing.ReferenceName(req.Ref), false)
		if err != nil {
			return nil, errors.New("ref is not found")
		}
		commitHash = ref.Hash()
	}

	commit, err := repo.CommitObject(commitHash)
	if err != nil {
		return nil, errors.New("commit is not found")
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, xerrors.Newf("failed to get tree: %v", err)
	}
	var treeEntry *object.TreeEntry
	if req.Path != "/" {
		te, err := tree.FindEntry(req.Path)
		if err != nil {
			return nil, xerrors.Newf("could not find the tree entry %s in %s: %v", req.Path, tree.Hash.String(), err)
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
			return nil, xerrors.Newf("failed to get blob: %v", err)
		}
		r, err := blob.Reader()
		if err != nil {
			return nil, xerrors.Newf("failed to get file reader: %v", err)
		}
		defer r.Close()
		buf, err := io.ReadAll(r)
		if err != nil {
			return nil, xerrors.Newf("failed to read file: %v", err)
		}

		var rawURL, editURL string
		remote, err := repo.Remote("origin")
		if err != nil {
			return nil, xerrors.Newf("failed to get remote: %v", err)
		}
		u, err := url.Parse(remote.Config().URLs[0])
		if err != nil {
			return nil, xerrors.Newf("invalid remote url %s: %v", remote.Config().URLs[0], err)
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
	repo, ok := g.repo[req.Repo]
	if !ok {
		return nil, errors.New("repository not found")
	}

	var commitHash plumbing.Hash
	if plumbing.IsHash(req.Ref) {
		commitHash = plumbing.NewHash(req.Ref)
	} else {
		ref, err := repo.Reference(plumbing.ReferenceName(req.Ref), false)
		if err != nil {
			return nil, errors.New("ref is not found")
		}
		commitHash = ref.Hash()
	}

	commit, err := repo.CommitObject(commitHash)
	if err != nil {
		return nil, errors.New("commit is not found")
	}
	if req.Path == "" || req.Path == "/" {
		return &ResponseStat{Hash: commit.TreeHash.String(), Mode: uint32(filemode.Dir)}, nil
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, xerrors.Newf("failed to get tree: %v", err)
	}
	if req.Path[0] == '/' {
		req.Path = req.Path[1:]
	}
	if req.Path[len(req.Path)-1] == '/' {
		req.Path = req.Path[:len(req.Path)-1]
	}
	treeEntry, err := tree.FindEntry(req.Path)
	if err != nil {
		return nil, xerrors.Newf("could not find the tree entry %s in %s: %v", req.Path, tree.Hash.String(), err)
	}

	return &ResponseStat{Name: path.Join(path.Dir(req.Path), treeEntry.Name), Hash: treeEntry.Hash.String(), Mode: uint32(treeEntry.Mode)}, nil
}

func (g *DataService) ListTag(_ context.Context, req *RequestListTag) (*ResponseListTag, error) {
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

func (g *DataService) ListBranch(_ context.Context, req *RequestListBranch) (*ResponseListBranch, error) {
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
