package sidecar

import (
	"context"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5/plumbing/filemode"
	"go.f110.dev/xerrors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/git"
)

type CloneCommand struct {
	WorkDir        string
	AppId          int64
	InstallationId int64
	PrivateKeyFile string
	Repo           string
	Commit         string

	GitDataServiceURL string
	GitDataRepo       string
}

func NewCloneCommand() *CloneCommand {
	return &CloneCommand{}
}

func (c *CloneCommand) Name() string {
	return "clone"
}

func (c *CloneCommand) SetFlags(fs *cli.FlagSet) {
	fs.String("work-dir", "Working directory").Shorthand("w").Var(&c.WorkDir)
	fs.Int64("github-app-id", "GitHub App Id").Var(&c.AppId)
	fs.Int64("github-installation-id", "GitHub Installation Id").Var(&c.InstallationId)
	fs.String("private-key-file", "GitHub app private key file").Var(&c.PrivateKeyFile)
	fs.String("url", "Repository url (e.g. git@github.com:octocat/example.git)").Var(&c.Repo)
	fs.String("commit", "Specify commit").Shorthand("b").Var(&c.Commit)
	fs.String("git-data-service-url", "URL of the git-data-service gRPC endpoint. If set, the source tree is fetched from git-data-service instead of cloning the repository.").Var(&c.GitDataServiceURL)
	fs.String("git-data-repo", "Repository name registered with git-data-service").Var(&c.GitDataRepo)
}

func (c *CloneCommand) Run(ctx context.Context) error {
	if c.GitDataServiceURL != "" {
		return c.exportFromGitDataService(ctx)
	}
	return git.Clone(ctx, c.AppId, c.InstallationId, c.PrivateKeyFile, c.WorkDir, c.Repo, c.Commit)
}

func (c *CloneCommand) exportFromGitDataService(ctx context.Context) error {
	if c.GitDataRepo == "" {
		return xerrors.Define("--git-data-repo is required when --git-data-service-url is set").WithStack()
	}
	if c.Commit == "" {
		return xerrors.Define("--commit is required when --git-data-service-url is set").WithStack()
	}

	conn, err := grpc.NewClient(c.GitDataServiceURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return xerrors.WithStack(err)
	}
	defer conn.Close()

	return exportToDir(ctx, git.NewGitDataClient(conn), c.GitDataRepo, c.Commit, c.WorkDir)
}

// exportToDir materializes the working tree of ref in repo, served by the
// git-data-service client, into dir. It walks the tree recursively and writes
// each blob with the file mode recorded in git. ref may be a commit SHA or a
// reference name.
func exportToDir(ctx context.Context, client git.GitDataClient, repo, ref, dir string) error {
	res, err := client.GetTree(ctx, &git.RequestGetTree{Repo: repo, Ref: ref, Path: "/", Recursive: true})
	if err != nil {
		return xerrors.WithStack(err)
	}

	for _, entry := range res.GetTree() {
		dst := filepath.Join(dir, entry.GetPath())
		switch entry.GetMode() {
		case filemode.Dir.String():
			if err := os.MkdirAll(dst, 0755); err != nil {
				return xerrors.WithStack(err)
			}
		case filemode.Submodule.String():
			// Submodules have no blob to materialize.
			continue
		case filemode.Symlink.String():
			blob, err := client.GetBlob(ctx, &git.RequestGetBlob{Repo: repo, Sha: entry.GetSha()})
			if err != nil {
				return xerrors.WithStack(err)
			}
			if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
				return xerrors.WithStack(err)
			}
			if err := os.Symlink(string(blob.GetContent()), dst); err != nil {
				return xerrors.WithStack(err)
			}
		default:
			mode := os.FileMode(0644)
			if entry.GetMode() == filemode.Executable.String() {
				mode = 0755
			}
			blob, err := client.GetBlob(ctx, &git.RequestGetBlob{Repo: repo, Sha: entry.GetSha()})
			if err != nil {
				return xerrors.WithStack(err)
			}
			if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
				return xerrors.WithStack(err)
			}
			if err := os.WriteFile(dst, blob.GetContent(), mode); err != nil {
				return xerrors.WithStack(err)
			}
		}
	}

	return nil
}
