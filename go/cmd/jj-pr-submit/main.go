package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
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

const (
	stackRevsets         = "ancestors(latest(%s@origin) & remote_branches())..@ ~ empty()"
	stackNavigatorHeader = "\n---\n\nPull request chain:\n\n"
)

type jujutsuPRSubmitCommand struct {
	*fsm.FSM

	Dir           string
	Repository    string
	DefaultBranch string
	DryRun        bool
	Force         bool
	DisplayStack  bool

	repositoryOwner string
	repositoryName  string
	token           string
	ghClient        *github.Client

	stack          stackedCommit
	remoteBranches []*github.Branch
	pullRequests   []*github.PullRequest
	prTemplate     string
}

const (
	stateInit fsm.State = iota
	stateGetToken
	stateGetMetadata
	stateDisplayStack
	statePushCommit
	stateCreatePR
	stateUpdatePR
	stateClose
)

func newCommand() *jujutsuPRSubmitCommand {
	c := &jujutsuPRSubmitCommand{}
	c.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateInit:         c.init,
			stateGetToken:     c.getToken,
			stateGetMetadata:  c.getMetadata,
			stateDisplayStack: c.displayStack,
			statePushCommit:   c.pushCommit,
			stateCreatePR:     c.createPR,
			stateUpdatePR:     c.updatePR,
			stateClose:        c.close,
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
	fs.BoolVar(&c.Force, "force", false, "Push commits when there are more than 10 commits in the stack")
	fs.BoolVar(&c.DisplayStack, "display-stack", false, "Only display the stack")
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

func (c *jujutsuPRSubmitCommand) displayStack(ctx context.Context) (fsm.State, error) {
	commits, err := c.getStack(ctx)
	if err != nil {
		return fsm.Error(err)
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
	return fsm.Finish()
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
		logger.Log.Debug("Retrieve pull requests")
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
	if c.DisplayStack {
		return fsm.Next(stateDisplayStack)
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
	ID      int
	Title   string
	Body    string
	Head    string
	HeadSHA string
	Base    string
	URL     string
	Draft   bool
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
	var changedPR []*commit
	for _, v := range c.stack {
		if v.Branch == "" {
			logger.Log.Debug("Will create branch", zap.String("change_id", v.ChangeID))
			pushArgs = append(pushArgs, fmt.Sprintf("--change=%s", v.ChangeID))
		} else {
			var found bool
			for _, r := range c.remoteBranches {
				if r.GetName() == v.Branch {
					found = true
					break
				}
			}
			if !found {
				pushArgs = append(pushArgs, fmt.Sprintf("--change=%s", v.ChangeID))
			}
		}
	}
	// Check remote branches to update PR
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
				changedPR = append(changedPR, h)
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

		// Comment PR
		if !c.DryRun {
			for _, pr := range changedPR {
				if pr.PullRequest == nil {
					continue
				}
				// If the state of the pull request is draft, we don't need to make the comment.
				if pr.PullRequest.Draft {
					continue
				}

				body := fmt.Sprintf("Update changes: https://github.com/%s/%s/compare/%s..%s", c.repositoryOwner, c.repositoryName, pr.PullRequest.HeadSHA, pr.CommitID)
				logger.Log.Debug("Make a new comment", zap.Int("number", pr.PullRequest.ID))
				_, _, err = c.ghClient.Issues.CreateComment(ctx, c.repositoryOwner, c.repositoryName, pr.PullRequest.ID, &github.IssueComment{Body: &body})
				if err != nil {
					return fsm.Error(xerrors.WithStack(err))
				}
			}
		}
	}

	return fsm.Next(stateCreatePR)
}

// getStack returns commits of current stack. The first commit is the newest commit.
func (c *jujutsuPRSubmitCommand) getStack(ctx context.Context) (stackedCommit, error) {
	const logTemplate = `change_id ++ "," ++ commit_id ++ "," ++ branches ++ ",\"" ++ description ++ "\"\n"`
	cmd := exec.CommandContext(ctx, "jj", "log", "--revisions", fmt.Sprintf(stackRevsets, c.DefaultBranch), "--no-graph", "--template", logTemplate)
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
			Branch:      strings.TrimSuffix(line[2], "*"),
			Description: line[3],
		}
		commits = append(commits, cm)
		logger.Log.Debug("Stack", zap.String("change_id", cm.ChangeID), zap.String("branch", cm.Branch))
	}
	if len(commits) > 9 && !c.Force {
		return nil, xerrors.Newf("there are %d commits in the stack.", len(commits))
	}
	if len(c.pullRequests) > 0 {
		for _, v := range commits {
			for _, pr := range c.pullRequests {
				if v.Branch == pr.Head.GetRef() {
					v.PullRequest = newPullRequest(pr)
					break
				}
			}
		}
	}

	return commits, nil
}

func (c *jujutsuPRSubmitCommand) createPR(ctx context.Context) (fsm.State, error) {
	// Find pull request template
	cmd := exec.CommandContext(ctx, "jj", "workspace", "root")
	cmd.Dir = c.Dir
	buf, err := cmd.CombinedOutput()
	if err != nil {
		return fsm.Error(err)
	}
	repoRoot := strings.TrimSpace(string(buf))
	templates, err := c.findPullRequestTemplate(repoRoot)
	if err != nil {
		return fsm.Error(err)
	}

	var template string
	// Create pull requests
	// Scan reverse order to create PR for older commit first.
	for i := len(c.stack) - 1; i >= 0; i-- {
		v := c.stack[i]
		if v.PullRequest != nil {
			continue
		}
		if template == "" && len(templates) > 0 {
			template, err = c.pickTemplate(templates, repoRoot)
			if err != nil {
				return fsm.Error(err)
			}
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
					description = v.Description[i+2:] + "\n" + template
				} else {
					description = template
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
			fmt.Printf("Created: %s\n", pr.GetHTMLURL())
		}
	}

	return fsm.Next(stateUpdatePR)
}

func (c *jujutsuPRSubmitCommand) pickTemplate(templates []string, repoRoot string) (string, error) {
	if c.prTemplate != "" {
		return c.prTemplate, nil
	}

	var templateFile string
	if len(templates) > 0 {
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
		if num == len(templates)+1 {
			templateFile = ""
		}
		if 0 < num && num <= len(templates) {
			templateFile = templates[num-1]
		}
	}
	var template string
	if templateFile != "" {
		buf, err := os.ReadFile(templateFile)
		if err != nil {
			return "", xerrors.WithStack(err)
		}
		template = string(buf)
	}

	c.prTemplate = template
	return template, nil
}

func (*jujutsuPRSubmitCommand) findPullRequestTemplate(root string) ([]string, error) {
	if _, err := os.Lstat(filepath.Join(root, ".github")); os.IsNotExist(err) {
		return nil, nil
	}

	var templates []string
	err := filepath.Walk(filepath.Join(root, ".github"), func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return xerrors.WithStack(err)
		}
		if path == filepath.Join(root, ".github") {
			return nil
		}
		if info.Mode().IsDir() && info.Name() != "PULL_REQUEST_TEMPLATE" {
			return filepath.SkipDir
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		filename := filepath.Base(path)
		if strings.ToLower(filename) == "pull_request_template.md" {
			templates = append(templates, path)
		}
		if strings.Contains(path, ".github/PULL_REQUEST_TEMPLATE/") {
			templates = append(templates, path)
		}
		return nil
	})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	if _, err := os.Lstat(filepath.Join(root, "docs/pull_request_template.md")); err == nil {
		templates = append(templates, filepath.Join(root, "docs/pull_request_template.md"))
	}

	sort.Strings(templates)
	return templates, nil
}

func (c *jujutsuPRSubmitCommand) updatePR(ctx context.Context) (fsm.State, error) {
	for i := len(c.stack) - 1; i >= 0; i-- {
		v := c.stack[i]
		if v.PullRequest == nil {
			if !c.DryRun {
				logger.Log.Error("BUG: The pull request must update. but we can't find the pull request.")
			}
			logger.Log.Info("Skip to update the pull request")
			continue
		}
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
			stackNav := stackNavigatorHeader
			for i := len(c.stack) - 1; i >= 0; i-- {
				c := c.stack[i]
				var arrow string
				if v == c {
					arrow = " 👉"
				}
				// Sometimes, PullRequest is nil when dry-run is enabled.
				if c.PullRequest != nil {
					stackNav += fmt.Sprintf("1.%s #%d\n", arrow, c.PullRequest.ID)
				}
			}
			if i := strings.LastIndex(v.PullRequest.Body, stackNavigatorHeader+"1."); i >= 0 {
				body = v.PullRequest.Body[:i]
			}
			if len(body) > 0 && body[len(body)-1] != '\n' {
				body += "\n"
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
			fmt.Printf("Update pull request: %s\n", v.PullRequest.URL)
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
		ID:      pr.GetNumber(),
		Title:   pr.GetTitle(),
		Body:    pr.GetBody(),
		Head:    pr.GetHead().GetRef(),
		HeadSHA: pr.GetHead().GetSHA(),
		Base:    pr.GetBase().GetRef(),
		URL:     pr.GetHTMLURL(),
		Draft:   pr.GetDraft(),
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
