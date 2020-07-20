package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v29/github"
	"github.com/spf13/pflag"
	"golang.org/x/xerrors"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	gogitHttp "gopkg.in/src-d/go-git.v4/plumbing/transport/http"
)

const (
	ActionClone = "clone"
)

func actionClone(appId, installationId int64, privateKeyFile, dir, repo, commit string) error {
	var auth *gogitHttp.BasicAuth
	rt := http.DefaultTransport
	if _, err := os.Stat(privateKeyFile); !os.IsNotExist(err) {
		t, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, appId, installationId, privateKeyFile)
		if err != nil {
			return xerrors.Errorf(": %v", err)
		}
		token, err := t.Token(context.Background())
		if err != nil {
			return xerrors.Errorf(": %v", err)
		}
		auth = &gogitHttp.BasicAuth{Username: "octocat", Password: token}
		rt = t
	}

	archiveDownload := false
	u, err := url.Parse(repo)
	if err == nil {
		if u.Scheme == "https" && u.Hostname() == "github.com" {
			archiveDownload = true
		}
	}

	if commit != "" && archiveDownload {
		return checkoutCommit(dir, repo, commit, rt)
	} else {
		return cloneByGit(dir, repo, commit, 1, auth)
	}
}

func cloneByGit(dir, repo, commit string, depth int, auth transport.AuthMethod) error {
	if commit != "" {
		depth = 0
	}

	log.Printf("Git clone from %s", repo)
	r, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL:   repo,
		Depth: depth,
		Auth:  auth,
	})
	if err != nil {
		return xerrors.Errorf(": %v", err)
	}

	if commit != "" {
		log.Printf("Checkout %s", commit)
		tree, err := r.Worktree()
		if err != nil {
			return xerrors.Errorf(": %v", err)
		}
		if err := tree.Checkout(&git.CheckoutOptions{Hash: plumbing.NewHash(commit)}); err != nil {
			return xerrors.Errorf(": %v", err)
		}
	}

	return nil
}

func checkoutCommit(dir, u, commit string, rt http.RoundTripper) error {
	addr := u
	if strings.HasSuffix(u, ".git") {
		addr = strings.TrimSuffix(u, ".git")
	}
	parsed, err := url.Parse(addr)
	if err != nil {
		return xerrors.Errorf(": %v", err)
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

	log.Printf("Download archive from %s", archiveLink.String())
	req, err := http.NewRequest(http.MethodGet, archiveLink.String(), nil)
	if err != nil {
		return xerrors.Errorf(": %v", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return xerrors.Errorf(": %v", err)
	}
	defer res.Body.Close()

	gzReader, err := gzip.NewReader(res.Body)
	if err != nil {
		return xerrors.Errorf(": %v", err)
	}
	tarReader := tar.NewReader(gzReader)
	for {
		h, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return xerrors.Errorf(": %v", err)
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
				return xerrors.Errorf(": %v", err)
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
			return xerrors.Errorf(": %v", err)
		}
		dirname := filepath.Dir(filename)
		if _, err := os.Stat(dirname); os.IsNotExist(err) {
			if err := os.MkdirAll(dirname, 755); err != nil {
				return xerrors.Errorf(": %v", err)
			}
		}
		if err := ioutil.WriteFile(filename, b, os.FileMode(h.Mode)); err != nil {
			return xerrors.Errorf(": %v", err)
		}
	}

	return nil
}

func buildSidecar(args []string) error {
	action := ""
	repo := ""
	appId := int64(0)
	installationId := int64(0)
	privateKeyFile := ""
	commit := ""
	workingDir := ""
	fs := pflag.NewFlagSet("build-sidecar", pflag.ContinueOnError)
	fs.StringVarP(&action, "action", "a", action, "Action")
	fs.StringVarP(&workingDir, "work-dir", "w", workingDir, "Working directory")
	fs.Int64Var(&appId, "github-app-id", appId, "GitHub App Id")
	fs.Int64Var(&installationId, "github-installation-id", installationId, "GitHub Installation Id")
	fs.StringVar(&privateKeyFile, "private-key-file", privateKeyFile, "GitHub app private key file")
	fs.StringVar(&repo, "url", repo, "Repository url (e.g. git@github.com:octocat/example.git)")
	fs.StringVarP(&commit, "commit", "b", "", "Specify commit")
	if err := fs.Parse(args); err != nil {
		return xerrors.Errorf(": %v", err)
	}

	switch action {
	case ActionClone:
		return actionClone(appId, installationId, privateKeyFile, workingDir, repo, commit)
	default:
		return xerrors.Errorf("unknown action: %v", action)
	}
}

func main() {
	if err := buildSidecar(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
