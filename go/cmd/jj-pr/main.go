package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/google/go-github/v85/github"
	"go.f110.dev/xerrors"
	"golang.org/x/oauth2"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/cli/flag"
	githubutil "go.f110.dev/mono/go/github"
	"go.f110.dev/mono/go/jujutsu"
)

func jujutsuPRSubmit(rootCmd *cli.Command) {
	c := jujutsu.NewPullRequestSubmitter()
	c.SelectTemplate = func(templates []string) (string, error) {
		return pickTemplate(templates, c.RootDir)
	}
	submitCmd := &cli.Command{
		Use: "submit",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			res, err := c.Submit(ctx)
			printSubmitResult(res)
			return err
		},
	}
	submitFlags(submitCmd.Flags(), c)
	rootCmd.AddCommand(submitCmd)
}

func submitFlags(fs *flag.FlagSet, c *jujutsu.PullRequestSubmitter) {
	fs.String("dir", "Working directory").Var(&c.Dir)
	fs.String("repository", "Repository name. If not specified, try to get from remote url").Var(&c.Repository)
	fs.String("default-branch", "Default branch name. If not specified, get from API").Var(&c.DefaultBranch)
	fs.Bool("dry-run", "Not impact on remote").Var(&c.DryRun)
	fs.Bool("force", "Push commits when there are more than 10 commits in the stack").Var(&c.Force)
	fs.Bool("single-pr", "Make the PR in multiple commits").Var(&c.SinglePR)
	fs.Bool("comment-on-update", "Comment the link to the changes on the updated PRs except the drafts").Var(&c.CommentOnUpdate)
	fs.String("revision", "Revision to create PR from. If not specified, use current revision").Shorthand("r").Var(&c.Revision)
}

func pickTemplate(templates []string, repoRoot string) (string, error) {
	fmt.Println("Found multiple templates. Please pick the template.")
	for i, v := range templates {
		fmt.Printf("%d: %s\n", i+1, strings.TrimPrefix(v, repoRoot))
	}
	fmt.Printf("%d: Don't use\n", len(templates)+1)
	fmt.Printf("Which template do you want to use?) ")
	num := -1
	n, _ := fmt.Scanf("%d", &num)
	if n != 1 {
		return "", xerrors.New("please pick the template")
	}
	if 0 < num && num <= len(templates) {
		return templates[num-1], nil
	}
	return "", nil
}

func printSubmitResult(res *jujutsu.SubmitResult) {
	if res == nil {
		return
	}
	for _, v := range res.Pushed {
		fmt.Printf("Push %s\n", v)
	}
	for _, v := range res.PullRequests {
		switch v.Action {
		case jujutsu.PullRequestCreated:
			if v.URL == "" {
				fmt.Printf("Create pull request for %s\n", v.ChangeID)
			} else {
				fmt.Printf("Created: %s\n", v.URL)
			}
		case jujutsu.PullRequestUpdated:
			fmt.Printf("Update pull request: %s\n", v.URL)
		case jujutsu.PullRequestExisting:
			fmt.Printf("Pull request already exists for %s: %s\n", v.ChangeID, v.URL)
		}
	}
}

func jujutsuPRStack(rootCmd *cli.Command) {
	dir := ""
	stackCmd := &cli.Command{
		Use: "stack",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			token, err := githubutil.CLIToken(ctx)
			if err != nil {
				return err
			}
			ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
			ghClient := github.NewClient(oauth2.NewClient(ctx, ts))

			owner, repoName, err := jujutsu.GitHubRepository(ctx, dir)
			if err != nil {
				return err
			}
			defaultBranch, err := jujutsu.DefaultBranch(ctx, ghClient, dir, owner, repoName)
			if err != nil {
				return err
			}

			commits, err := jujutsu.Stack(ctx, false, dir, defaultBranch, "")
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
			owner, repoName, err := jujutsu.GitHubRepository(ctx, dir)
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
			for v := range strings.SplitSeq(string(buf), "\n") {
				localBookmarks[v] = struct{}{}
			}

			token, err := githubutil.CLIToken(ctx)
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
