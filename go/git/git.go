package git

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gogitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v49/github"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/githubutil"
)

func Clone(ctx context.Context, appId, installationId int64, privateKeyFile, dir, repo, commit string) error {
	var auth *gogitHttp.BasicAuth
	rt := http.DefaultTransport
	if _, err := os.Stat(privateKeyFile); !os.IsNotExist(err) {
		app, err := githubutil.NewApp(appId, installationId, privateKeyFile)
		if err != nil {
			return err
		}
		token, err := app.JWT()
		if err != nil {
			return err
		}
		auth = &gogitHttp.BasicAuth{Username: "octocat", Password: token}
		rt = githubutil.NewTransportWithApp(http.DefaultTransport, app)
	}

	archiveDownloadable := false
	u, err := url.Parse(repo)
	if err == nil {
		if u.Scheme == "https" && u.Hostname() == "github.com" {
			archiveDownloadable = true
		}
	}

	if commit != "" && archiveDownloadable {
		return checkoutCommit(ctx, dir, repo, commit, rt)
	} else {
		return cloneByGit(ctx, dir, repo, commit, 1, auth)
	}
}

func checkoutCommit(ctx context.Context, dir, u, commit string, rt http.RoundTripper) error {
	addr := u
	if strings.HasSuffix(u, ".git") {
		addr = strings.TrimSuffix(u, ".git")
	}
	parsed, err := url.Parse(addr)
	if err != nil {
		return xerrors.WithStack(err)
	}
	s := strings.SplitN(parsed.Path, "/", 3)

	ghClient := github.NewClient(&http.Client{Transport: rt})
	archiveLink, _, err := ghClient.Repositories.GetArchiveLink(
		ctx,
		s[1], // owner
		s[2], // repo
		github.Tarball,
		&github.RepositoryContentGetOptions{Ref: commit},
		true,
	)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, archiveLink.String(), nil)
	if err != nil {
		return xerrors.WithStack(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return xerrors.WithStack(err)
	}
	defer res.Body.Close()

	gzReader, err := gzip.NewReader(res.Body)
	if err != nil {
		return xerrors.WithStack(err)
	}
	tarReader := tar.NewReader(gzReader)
	for {
		h, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return xerrors.WithStack(err)
		}
		d, f := filepath.Split(h.Name)
		if d == "" {
			continue
		}
		s := strings.Split(d, "/")
		filename := filepath.Join(dir, strings.Join(s[1:], "/"), f)

		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(filename, os.FileMode(h.Mode)); err != nil {
				return xerrors.WithStack(err)
			}
			continue
		case tar.TypeSymlink:
			if err := os.Symlink(h.Linkname, filename); err != nil {
				return xerrors.WithStack(err)
			}
			continue
		}

		b, err := io.ReadAll(tarReader)
		if err != nil {
			return xerrors.WithStack(err)
		}
		dirname := filepath.Dir(filename)
		if _, err := os.Stat(dirname); os.IsNotExist(err) {
			if err := os.MkdirAll(dirname, 755); err != nil {
				return xerrors.WithStack(err)
			}
		}
		if err := os.WriteFile(filename, b, os.FileMode(h.Mode)); err != nil {
			return xerrors.WithStack(err)
		}
	}

	return nil
}

func cloneByGit(ctx context.Context, dir, repo, commit string, depth int, auth transport.AuthMethod) error {
	if commit != "" {
		depth = 0
	}

	r, err := git.PlainCloneContext(ctx, dir, false, &git.CloneOptions{
		URL:   repo,
		Depth: depth,
		Auth:  auth,
	})
	if err != nil {
		return xerrors.WithStack(err)
	}

	if commit != "" {
		tree, err := r.Worktree()
		if err != nil {
			return xerrors.WithStack(err)
		}
		if err := tree.Checkout(&git.CheckoutOptions{Hash: plumbing.NewHash(commit)}); err != nil {
			return xerrors.WithStack(err)
		}
	}

	return nil
}
