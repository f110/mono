package git

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v32/github"
	"golang.org/x/xerrors"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	gogitHttp "gopkg.in/src-d/go-git.v4/plumbing/transport/http"
)

func Clone(appId, installationId int64, privateKeyFile, dir, repo, commit string) error {
	var auth *gogitHttp.BasicAuth
	rt := http.DefaultTransport
	if _, err := os.Stat(privateKeyFile); !os.IsNotExist(err) {
		t, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, appId, installationId, privateKeyFile)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		token, err := t.Token(context.Background())
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		auth = &gogitHttp.BasicAuth{Username: "octocat", Password: token}
		rt = t
	}

	archiveDownloadable := false
	u, err := url.Parse(repo)
	if err == nil {
		if u.Scheme == "https" && u.Hostname() == "github.com" {
			archiveDownloadable = true
		}
	}

	if commit != "" && archiveDownloadable {
		return checkoutCommit(dir, repo, commit, rt)
	} else {
		return cloneByGit(dir, repo, commit, 1, auth)
	}
}

func checkoutCommit(dir, u, commit string, rt http.RoundTripper) error {
	addr := u
	if strings.HasSuffix(u, ".git") {
		addr = strings.TrimSuffix(u, ".git")
	}
	parsed, err := url.Parse(addr)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	s := strings.SplitN(parsed.Path, "/", 3)

	ghClient := github.NewClient(&http.Client{Transport: rt})
	archiveLink, _, _ := ghClient.Repositories.GetArchiveLink(
		context.Background(),
		s[1], // owner
		s[2], // repo
		github.Tarball,
		&github.RepositoryContentGetOptions{Ref: commit},
		true,
	)

	req, err := http.NewRequest(http.MethodGet, archiveLink.String(), nil)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	defer res.Body.Close()

	gzReader, err := gzip.NewReader(res.Body)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	tarReader := tar.NewReader(gzReader)
	for {
		h, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return xerrors.Errorf(": %w", err)
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
				return xerrors.Errorf(": %w", err)
			}
			continue
		case tar.TypeSymlink:
			if err := os.Symlink(h.Linkname, filename); err != nil {
				return xerrors.Errorf(": %w", err)
			}
			continue
		}

		b, err := ioutil.ReadAll(tarReader)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		dirname := filepath.Dir(filename)
		if _, err := os.Stat(dirname); os.IsNotExist(err) {
			if err := os.MkdirAll(dirname, 755); err != nil {
				return xerrors.Errorf(": %w", err)
			}
		}
		if err := ioutil.WriteFile(filename, b, os.FileMode(h.Mode)); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}

func cloneByGit(dir, repo, commit string, depth int, auth transport.AuthMethod) error {
	if commit != "" {
		depth = 0
	}

	r, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL:   repo,
		Depth: depth,
		Auth:  auth,
	})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	if commit != "" {
		tree, err := r.Worktree()
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		if err := tree.Checkout(&git.CheckoutOptions{Hash: plumbing.NewHash(commit)}); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}
