package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/google/go-github/v49/github"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/logger"
)

type jujutsuPRSubmitCommand struct {
	*fsm.FSM

	Dir           string
	Repository    string
	DefaultBranch string
	DryRun        bool

	repositoryOwner string
	repositoryName  string
	token           string
	ghClient        *github.Client

	stack          stackedCommit
	remoteBranches []*github.Branch
	pullRequests   []*github.PullRequest
}

const (
	stateInit fsm.State = iota
	stateGetToken
	stateGetMetadata
	statePushCommit
	stateCreatePR
	stateUpdatePR
	stateClose
)

func newCommand() *jujutsuPRSubmitCommand {
	c := &jujutsuPRSubmitCommand{}
	c.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateInit:        c.init,
			stateGetToken:    c.getToken,
			stateGetMetadata: c.getMetadata,
			statePushCommit:  c.pushCommit,
			stateCreatePR:    c.createPR,
			stateUpdatePR:    c.updatePR,
			stateClose:       c.close,
		},
		stateInit,
		stateClose,
	)
	c.FSM.DisableErrorOutput = true
	return c
}

func (c *jujutsuPRSubmitCommand) flags(fs *pflag.FlagSet) {
	fs.StringVar(&c.Dir, "dir", "", "Working directory")
	fs.StringVar(&c.Repository, "repository", "", "Repository name. If not specified, try to get from remote url")
	fs.StringVar(&c.DefaultBranch, "default-branch", "", "Default branch name. If not specified, get from API")
	fs.BoolVar(&c.DryRun, "dry-run", false, "Not impact on remote")
}

func (c *jujutsuPRSubmitCommand) init(ctx context.Context) (fsm.State, error) {
	if strings.HasPrefix(c.Repository, "https://github.com") {
		u, err := url.Parse(c.Repository)
		if err != nil {
			return fsm.Error(xerrors.WithStack(err))
		}
		s := strings.Split(u.Path, "/")
		if len(s) == 3 {
			c.repositoryOwner = s[1]
			c.repositoryName = s[2]
		}
	} else if strings.Contains(c.Repository, "/") {
		s := strings.Split(c.Repository, "/")
		if len(s) == 2 {
			c.repositoryOwner = s[0]
			c.repositoryName = s[1]
		}
	}
	if c.repositoryOwner == "" || c.repositoryName == "" {
		var err error
		c.repositoryOwner, c.repositoryName, err = findRepositoryOwnerName(ctx, c.Dir)
		if err != nil {
			return fsm.Error(err)
		}
	}

	return fsm.Next(stateGetToken)
}

func (c *jujutsuPRSubmitCommand) getToken(ctx context.Context) (fsm.State, error) {
	cmd := exec.CommandContext(ctx, "gh", "auth", "token")
	if v, err := cmd.CombinedOutput(); err != nil {
		return fsm.Error(xerrors.WithStack(err))
	} else {
		c.token = strings.TrimSpace(string(v))
	}
	if c.token == "" {
		return fsm.Error(xerrors.New("could not get api token"))
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: c.token})
	c.ghClient = github.NewClient(oauth2.NewClient(ctx, ts))

	return fsm.Next(stateGetMetadata)
}

func (c *jujutsuPRSubmitCommand) getMetadata(ctx context.Context) (fsm.State, error) {
	var wg sync.WaitGroup
	gotError := false
	if c.DefaultBranch == "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Log.Debug("Retrieve repository metadata")
			repo, _, err := c.ghClient.Repositories.Get(ctx, c.repositoryOwner, c.repositoryName)
			if err != nil {
				logger.Log.Error("Could not get repository metadata from api.github.com", logger.Error(err))
				gotError = true
				return
			}
			c.DefaultBranch = repo.GetDefaultBranch()
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Log.Debug("Retrieve the branch list")
		branches, _, err := c.ghClient.Repositories.ListBranches(ctx, c.repositoryOwner, c.repositoryName, &github.BranchListOptions{})
		if err != nil {
			logger.Log.Error("Could not get the branch list of the remote repository", logger.Error(err))
			gotError = true
			return
		}
		c.remoteBranches = branches
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		pullRequests, _, err := c.ghClient.PullRequests.List(ctx, c.repositoryOwner, c.repositoryName, &github.PullRequestListOptions{})
		if err != nil {
			logger.Log.Error("Could not get pull requests", logger.Error(err))
			gotError = true
			return
		}
		c.pullRequests = pullRequests
	}()
	wg.Wait()

	if gotError {
		return fsm.Error(xerrors.New(""))
	}
	return fsm.Next(statePushCommit)
}

type commit struct {
	ChangeID string
	CommitID string
	// Branch is local branch name if exists.
	Branch      string
	Description string

	PullRequest *pullRequest
}

type pullRequest struct {
	ID    int
	Title string
	Body  string
	Head  string
	Base  string
}

type stackedCommit []*commit

func (c *jujutsuPRSubmitCommand) pushCommit(ctx context.Context) (fsm.State, error) {
	// Get all commits in current branch
	stack, err := c.getStack(ctx)
	if err != nil {
		return fsm.Error(err)
	}
	c.stack = stack

	// Push all commits of current branch
	var pushArgs []string
	for _, v := range c.stack {
		if v.Branch == "" {
			logger.Log.Debug("Will create branch", zap.String("change_id", v.ChangeID))
			pushArgs = append(pushArgs, fmt.Sprintf("--change=%s", v.ChangeID))
		}
	}
	for _, v := range c.remoteBranches {
		if v.GetName() == "" {
			continue
		}
		if !strings.HasPrefix(v.GetName(), "push-") {
			continue
		}
		shortChangeID := strings.TrimPrefix(v.GetName(), "push-")
		commitID := v.GetCommit().GetSHA()
		for _, h := range c.stack {
			if strings.HasPrefix(h.ChangeID, shortChangeID) && commitID != h.CommitID {
				logger.Log.Debug("Will update branch", zap.String("change_id", h.ChangeID))
				pushArgs = append(pushArgs, fmt.Sprintf("--change=%s", h.ChangeID))
				break
			}
		}
	}
	if len(pushArgs) > 0 {
		pushArgs = append([]string{"git", "push"}, pushArgs...)
		logger.Log.Debug("Push commits to create branches")
		cmd := exec.CommandContext(ctx, "jj", pushArgs...)
		cmd.Dir = c.Dir
		if c.DryRun {
			cmd.Args = append(cmd.Args, "--dry-run")
		}
		cmd.Stdout = os.Stdout
		if err = cmd.Run(); err != nil {
			return fsm.Error(xerrors.WithStack(err))
		}

		// Get all commits because the stack has been changed.
		logger.Log.Debug("Re-fetch commits from jj")
		stack, err := c.getStack(ctx)
		if err != nil {
			return fsm.Error(err)
		}
		c.stack = stack
	}

	return fsm.Next(stateCreatePR)
}

func (c *jujutsuPRSubmitCommand) getStack(ctx context.Context) (stackedCommit, error) {
	const logTemplate = `change_id ++ "," ++ commit_id ++ "," ++ branches ++ ",\"" ++ description ++ "\"\n"`
	cmd := exec.CommandContext(ctx, "jj", "log", "--revisions", "(latest(present(main) | present(master)) & remote_branches())..@ ~ empty()", "--no-graph", "--template", logTemplate)
	cmd.Dir = c.Dir
	buf, err := cmd.CombinedOutput()
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	r := csv.NewReader(bytes.NewReader(buf))
	var commits stackedCommit
	for {
		line, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, xerrors.WithStack(err)
		}

		if len(line) < 3 {
			continue
		}
		cm := &commit{
			ChangeID:    line[0],
			CommitID:    line[1],
			Branch:      line[2],
			Description: line[3],
		}
		commits = append(commits, cm)
		logger.Log.Debug("Stack", zap.String("change_id", cm.ChangeID), zap.String("branch", cm.Branch))
	}

	return commits, nil
}

func (c *jujutsuPRSubmitCommand) createPR(ctx context.Context) (fsm.State, error) {
	for _, v := range c.stack {
		for _, pr := range c.pullRequests {
			if v.Branch == pr.Head.GetRef() {
				v.PullRequest = newPullRequest(pr)
				break
			}
		}
	}

	// Create pull requests
	for i := len(c.stack) - 1; i >= 0; i-- {
		v := c.stack[i]
		if v.PullRequest != nil {
			continue
		}

		// There is no pull request for the commit.
		// We need to create a pull request.
		fmt.Printf("Create pull request for %s\n", v.ChangeID)
		if !c.DryRun {
			baseBranch := c.DefaultBranch
			if i != len(c.stack)-1 {
				baseBranch = c.stack[i+1].Branch
			}
			var title, description string
			if i := strings.Index(v.Description, "\n"); i > 0 {
				title = v.Description[:i]
				if len(v.Description) > i+2 {
					description = v.Description[i+2:]
				}
			}

			pr, _, err := c.ghClient.PullRequests.Create(ctx, c.repositoryOwner, c.repositoryName, &github.NewPullRequest{
				Title: github.String(title),
				Body:  github.String(description),
				Head:  github.String(v.Branch),
				Base:  github.String(baseBranch),
				Draft: github.Bool(true),
			})
			if err != nil {
				return fsm.Error(xerrors.WithStack(err))
			}
			v.PullRequest = newPullRequest(pr)
		}
	}

	return fsm.Next(stateUpdatePR)
}

func (c *jujutsuPRSubmitCommand) updatePR(ctx context.Context) (fsm.State, error) {
	for i := len(c.stack) - 1; i >= 0; i-- {
		v := c.stack[i]
		var updatedPR github.PullRequest

		var needUpdateBaseBranch, needUpdateTitle, needUpdateBody bool
		if i != len(c.stack)-1 && v.PullRequest.Base != c.stack[i+1].Branch {
			needUpdateBaseBranch = true
			updatedPR.Base = &github.PullRequestBranch{Ref: github.String(c.stack[i+1].Branch)}
		}
		if i := strings.Index(v.Description, "\n"); i > 0 {
			if v.PullRequest.Title != v.Description[:i] {
				updatedPR.Title = github.String(v.Description[:i])
				needUpdateTitle = true
			}
		}
		body := v.PullRequest.Body
		if len(c.stack) > 1 {
			stackNav := "\n---\n\nPull request chain:\n\n"
			for _, c := range c.stack {
				var arrow string
				if v == c {
					arrow = " 👉"
				}
				stackNav += fmt.Sprintf("-%s #%d\n", arrow, c.PullRequest.ID)
			}
			if i := strings.LastIndex(v.PullRequest.Body, "\n---\n\nPull request chain:\n\n-"); i >= 0 {
				body = v.PullRequest.Body[:i]
			}
			body += stackNav
		}
		if body == "" {
			body = v.Description
		}
		if body != v.PullRequest.Body {
			updatedPR.Body = github.String(body)
			needUpdateBody = true
		}

		if needUpdateBaseBranch || needUpdateTitle || needUpdateBody {
			// Update the pull request
			fmt.Printf("Update pull request for %s\n", v.ChangeID)
			logger.Log.Debug("Update pull request reason", zap.Bool("base_branch", needUpdateBaseBranch), zap.Bool("title", needUpdateTitle), zap.Bool("body", needUpdateBody))
			if !c.DryRun {
				pr, _, err := c.ghClient.PullRequests.Edit(ctx, c.repositoryOwner, c.repositoryName, v.PullRequest.ID, &updatedPR)
				if err != nil {
					return fsm.Error(xerrors.WithStack(err))
				}
				v.PullRequest = newPullRequest(pr)
			}
		}
	}

	return fsm.Next(stateClose)
}

func (c *jujutsuPRSubmitCommand) close(_ context.Context) (fsm.State, error) {
	return fsm.Finish()
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

func newPullRequest(pr *github.PullRequest) *pullRequest {
	return &pullRequest{
		ID:    pr.GetNumber(),
		Title: pr.GetTitle(),
		Body:  pr.GetBody(),
		Head:  pr.GetHead().GetRef(),
		Base:  pr.GetBase().GetRef(),
	}
}

func jujutsuPRSubmit() error {
	c := newCommand()
	cmd := &cobra.Command{
		Use: "jj-pr-submit",
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return logger.Init()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.LoopContext(cmd.Context())
		},
		SilenceErrors: true,
	}
	c.flags(cmd.Flags())
	logger.Flags(cmd.Flags())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return cmd.ExecuteContext(ctx)
}

func main() {
	if err := jujutsuPRSubmit(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
