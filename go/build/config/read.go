package config

import (
	"bytes"
	"context"
	"strings"

	"github.com/google/go-github/v73/github"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/logger"
)

const (
	BuildConfigurationFile = "build.star"
	BazelVersionFile       = ".bazelversion"
)

func ReadFromRepository(ctx context.Context, githubClient *github.Client, owner, repoName string) (*Config, error) {
	// Find the configuration file
	var commitSHA string
	// If the revision doesn't belong to the main branch, the build configuration will be read from the main branch.
	logger.Log.Debug("GetCommit", logger.String("owner", owner), logger.String("repo", repoName))
	commit, _, err := githubClient.Repositories.GetCommit(ctx, owner, repoName, "HEAD", nil)
	if err != nil {
		return nil, xerrors.WithMessage(err, "failed to get HEAD commit")
	}
	commitSHA = commit.GetSHA()
	logger.Log.Debug("GetTree", logger.String("commitSHA", commitSHA))
	tree, _, err := githubClient.Git.GetTree(ctx, owner, repoName, commitSHA, false)
	if err != nil {
		return nil, xerrors.WithMessagef(err, "failed to get the tree: %s", commitSHA)
	}
	var configBlobSHA string
Entries:
	for _, e := range tree.Entries {
		switch e.GetPath() {
		case BuildConfigurationFile:
			configBlobSHA = e.GetSHA()
			break Entries
		}
	}
	if configBlobSHA == "" {
		logger.Log.Debug("build configuration file is not found", logger.String("repo", repoName), logger.String("revision", commitSHA))
		return nil, nil
	}
	logger.Log.Debug("GetBlob", logger.String("repo", repoName), logger.String("revision", configBlobSHA))
	buildConfFileBlob, _, err := githubClient.Git.GetBlobRaw(ctx, owner, repoName, configBlobSHA)
	if err != nil {
		return nil, err
	}

	// Parse the configuration file
	conf, err := Read(bytes.NewReader(buildConfFileBlob), owner, repoName)
	if err != nil {
		return nil, err
	}

	return conf, nil
}

func GetBazelVersionFromRepository(ctx context.Context, githubClient *github.Client, owner, repoName string) (string, error) {
	commit, _, err := githubClient.Repositories.GetCommit(ctx, owner, repoName, "HEAD", nil)
	if err != nil {
		return "", xerrors.WithMessage(err, "failed to get HEAD commit")
	}
	tree, _, err := githubClient.Git.GetTree(ctx, owner, repoName, commit.GetSHA(), false)
	if err != nil {
		return "", xerrors.WithMessagef(err, "failed to get the tree: %s", commit.GetSHA())
	}
	var versionBlobSHA string
Entries:
	for _, e := range tree.Entries {
		switch e.GetPath() {
		case BazelVersionFile:
			versionBlobSHA = e.GetSHA()
			break Entries
		}
	}
	blob, _, err := githubClient.Git.GetBlobRaw(ctx, owner, repoName, versionBlobSHA)
	if err != nil {
		return "", xerrors.WithMessagef(err, "failed to get the blob: %s", versionBlobSHA)
	}
	bazelVersion := strings.TrimRight(string(blob), "\n")
	return bazelVersion, nil
}
