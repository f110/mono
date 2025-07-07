package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
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
	const logTemplate = `"{\"change_id\":" ++ json(change_id) ++ ",\"commit_id\":" ++ json(commit_id) ++ ",\"bookmarks\":" ++ json(bookmarks) ++ ",\"description\":" ++ json(description) ++ "}\n"`
	cmd := exec.CommandContext(ctx, "jj", "log", "--revisions", fmt.Sprintf(stackRevsets, defaultBranch), "--no-graph", "--template", logTemplate)
	cmd.Dir = dir
	buf, err := cmd.CombinedOutput()
	if err != nil {
		return nil, xerrors.WithStack(xerrors.WithStack(err))
	}

	var commits stackedCommit
	scanner := bufio.NewScanner(bytes.NewReader(buf))
	for scanner.Scan() {
		c := &commit{}
		if err := json.Unmarshal(scanner.Bytes(), c); err != nil {
			return nil, xerrors.WithStack(err)
		}
		if !(withoutNoSend && (strings.HasPrefix(c.Description, noSendTag) || strings.HasPrefix(c.Description, wipTag))) {
			logger.Log.Debug("Stack", zap.String("change_id", c.ChangeID), zap.Any("bookmarks", c.Bookmarks))
			commits = append(commits, c)
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

func jujutsuPRSubmit(rootCmd *cli.Command) {
	c := newSubmitCommand()
	submitCmd := &cli.Command{
		Use: "submit",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return c.LoopContext(ctx)
		},
	}
	c.flags(submitCmd.Flags())
	rootCmd.AddCommand(submitCmd)
}

func jujutsuPRStack(rootCmd *cli.Command) {
	dir := ""
	stackCmd := &cli.Command{
		Use: "stack",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			token, err := getToken(ctx)
			if err != nil {
				return err
			}
			ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
			ghClient := github.NewClient(oauth2.NewClient(ctx, ts))

			owner, repoName, err := findRepositoryOwnerName(ctx, dir)
			if err != nil {
				return err
			}
			defaultBranch, err := getDefaultBranch(ctx, ghClient, owner, repoName)
			if err != nil {
				return err
			}

			commits, err := getStack(ctx, false, dir, defaultBranch)
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
	stackCmd.Flags().String("dir", "Working directory").Var(&dir)
	rootCmd.AddCommand(stackCmd)
}

func jujutsuPRList(rootCmd *cli.Command) {
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
	rootCmd.AddCommand(listCmd)
}

func jujutsuPR() error {
	cmd := &cli.Command{
		Use: "jj pr",
	}

	jujutsuPRSubmit(cmd)
	jujutsuPRStack(cmd)
	jujutsuPRList(cmd)

	return cmd.Execute(os.Args)
}

func main() {
	if err := jujutsuPR(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
