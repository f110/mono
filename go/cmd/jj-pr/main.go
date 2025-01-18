package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/google/go-github/v49/github"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/logger"
)

func getToken(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", "auth", "token")
	v, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(v)), nil
}

func getDefaultBranch(ctx context.Context, ghClient *github.Client, owner, name string) (string, error) {
	logger.Log.Debug("Retrieve repository metadata")
	cmd := exec.CommandContext(ctx, "jj", "show", "trunk()", "--template", "remote_bookmarks")
	buf, err := cmd.CombinedOutput()
	if err == nil {
		if i := bytes.Index(buf, []byte("@")); i >= 0 {
			logger.Log.Debug("Get default branch from local repository")
			return string(buf[:i]), nil
		}
	}

	repo, _, err := ghClient.Repositories.Get(ctx, owner, name)
	if err != nil {
		logger.Log.Error("Could not get repository metadata from api.github.com", logger.Error(err))
		return "", err
	}
	return repo.GetDefaultBranch(), nil
}

// getStack returns commits of current stack. The first commit is the newest commit.
func getStack(ctx context.Context, withoutNoSend bool, dir, defaultBranch string) (stackedCommit, error) {
	const logTemplate = `change_id ++ "\\" ++ commit_id ++ "\\[" ++ bookmarks ++ "]\\" ++ description ++ "\\\n"`
	cmd := exec.CommandContext(ctx, "jj", "log", "--revisions", fmt.Sprintf(stackRevsets, defaultBranch), "--no-graph", "--template", logTemplate)
	cmd.Dir = dir
	buf, err := cmd.CombinedOutput()
	if err != nil {
		return nil, xerrors.WithStack(xerrors.WithStack(err))
	}

	type readState int
	const (
		readStateChangeID readState = iota
		readCommitID
		readBranches
		readDescription
	)
	var commits stackedCommit
	var changeID, commitID, branches, description string
	var prev int
	var state = readStateChangeID
	for i := range len(buf) {
		if buf[i] != '\\' {
			continue
		}

		switch state {
		case readStateChangeID:
			changeID = string(buf[prev:i])
			state = readCommitID
			prev = i + 1
		case readCommitID:
			commitID = string(buf[prev:i])
			state = readBranches
			prev = i + 1
		case readBranches:
			if buf[i-1] != ']' {
				continue
			}
			branches = string(buf[prev+1 : i-1])
			state = readDescription
			prev = i + 1
		case readDescription:
			if buf[i+1] != '\n' {
				continue
			}
			description = string(buf[prev:i])
			if !(withoutNoSend && (strings.HasPrefix(description, noSendTag) || strings.HasPrefix(description, wipTag))) {
				cm := &commit{
					ChangeID:    changeID,
					CommitID:    commitID,
					Branch:      strings.TrimSuffix(branches, "*"),
					Description: description,
				}
				commits = append(commits, cm)
				logger.Log.Debug("Stack", zap.String("change_id", cm.ChangeID), zap.String("branch", cm.Branch))
			}

			// Next commit
			i++
			changeID, commitID, branches, description = "", "", "", ""
			state = readStateChangeID
			prev = i + 1
		}
	}

	return commits, nil
}

func findRepositoryOwnerName(ctx context.Context, dir string) (string, string, error) {
	c := exec.CommandContext(ctx, "jj", "git", "remote", "list")
	c.Dir = dir
	r, err := c.CombinedOutput()
	if err != nil {
		return "", "", xerrors.WithMessage(err, "failed to get remote list")
	}
	s := bufio.NewScanner(bytes.NewReader(r))
	var owner, name string
	for s.Scan() {
		line := s.Text()
		s := strings.Split(line, " ")
		if len(s) == 1 {
			continue
		}
		u, err := url.Parse(s[1])
		if err != nil {
			continue
		}
		if u.Host != "github.com" {
			continue
		}
		s = strings.Split(u.Path, "/")
		owner, name = s[1], s[2]
		if strings.HasSuffix(name, ".git") {
			name = strings.TrimSuffix(name, ".git")
		}
		break
	}
	if owner == "" || name == "" {
		return "", "", xerrors.New("could not find repository owner and name")
	}

	return owner, name, nil
}

func jujutsuPR() error {
	cmd := &cli.Command{
		Use: "jj pr",
	}

	{
		c := newSubmitCommand()
		submitCmd := &cli.Command{
			Use: "submit",
			Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
				return c.LoopContext(ctx)
			},
		}
		c.flags(submitCmd.Flags())
		cmd.AddCommand(submitCmd)
	}

	{
		stackCmd := &cli.Command{
			Use: "stack",
			Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
				token, err := getToken(ctx)
				if err != nil {
					return err
				}
				ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
				ghClient := github.NewClient(oauth2.NewClient(ctx, ts))

				owner, repoName, err := findRepositoryOwnerName(ctx, "")
				if err != nil {
					return err
				}
				defaultBranch, err := getDefaultBranch(ctx, ghClient, owner, repoName)
				if err != nil {
					return err
				}

				commits, err := getStack(ctx, false, "", defaultBranch)
				if err != nil {
					return err
				}
				for i, v := range commits {
					var title string
					if i := strings.Index(v.Description, "\n"); i > 0 {
						title = v.Description[:i]
					} else {
						title = v.Description
					}
					fmt.Printf("%d. %s: %s\n", i+1, v.ChangeID[:12], title)
				}
				return nil
			},
		}
		cmd.AddCommand(stackCmd)
	}

	{
		dir := ""
		listCmd := &cli.Command{
			Use: "list",
			Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
				owner, repoName, err := findRepositoryOwnerName(ctx, dir)
				if err != nil {
					return err
				}

				cmd := exec.CommandContext(ctx, "jj", "bookmark", "list", "--template", `name ++ "\\\n"`)
				cmd.Dir = dir
				buf, err := cmd.CombinedOutput()
				if err != nil {
					return err
				}
				localBookmarks := make(map[string]struct{})
				for _, v := range strings.Split(string(buf), "\n") {
					localBookmarks[v] = struct{}{}
				}

				token, err := getToken(ctx)
				if err != nil {
					return err
				}
				ghClient := github.NewClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})))

				me, _, err := ghClient.Users.Get(ctx, "")
				if err != nil {
					return err
				}

				opt := &github.PullRequestListOptions{}
				for {
					prs, res, err := ghClient.PullRequests.List(ctx, owner, repoName, opt)
					if err != nil {
						return err
					}

					for _, pr := range prs {
						own := ""
						if _, ok := localBookmarks[pr.Head.GetRef()]; ok {
							own = "*"
						}
						if pr.User.GetLogin() == me.GetLogin() {
							own = "*"
						}
						fmt.Printf("#%d%s: %s %s\n", pr.GetNumber(), own, pr.GetTitle(), pr.GetHTMLURL())
					}

					if res.NextPage == 0 {
						break
					}
					opt.Page = res.NextPage
				}
				return nil
			},
		}
		listCmd.Flags().String("dir", "Working directory").Var(&dir)
		cmd.AddCommand(listCmd)
	}

	return cmd.Execute(os.Args)
}

func main() {
	if err := jujutsuPR(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
