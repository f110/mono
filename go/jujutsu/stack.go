package jujutsu

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/google/go-github/v85/github"
	"go.f110.dev/xerrors"
	"golang.org/x/oauth2"

	"go.f110.dev/mono/go/fsm"
	githubutil "go.f110.dev/mono/go/github"
	"go.f110.dev/mono/go/logger"
)

const (
	stackRevsets           = "ancestors(latest(%s@origin) & remote_bookmarks())..change_id(%s) ~ empty()"
	stackTemplate          = `"{\"change_id\":" ++ json(change_id) ++ ",\"commit_id\":" ++ json(commit_id) ++ ",\"bookmarks\":" ++ json(bookmarks) ++ ",\"description\":" ++ json(description) ++ "}\n"`
	lastPickedTemplateFile = ".last_template_file"
	noSendTag              = "no-send:"
	wipTag                 = "WIP:"
)

const (
	PullRequestCreated PullRequestAction = iota
	PullRequestUpdated
	// PullRequestExisting is the pull request which already exists and is not changed.
	PullRequestExisting
)

var trailerLineRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]*:\s`)

// PullRequestSubmitter pushes the commits of the stack and creates or updates the pull requests of them on GitHub.
type PullRequestSubmitter struct {
	*fsm.FSM

	Dir string
	// Repository is the repository on GitHub (owner/name or the URL). It is found from the remotes if it is empty.
	Repository string
	// DefaultBranch is found from trunk() or GitHub if it is empty.
	DefaultBranch string
	DryRun        bool
	// Force allows the stack which has more than 9 commits.
	Force bool
	// SinglePR makes the pull request of the newest commit which contains the whole stack.
	SinglePR bool
	// Revision is the change ID of the newest commit of the stack. It is the working copy if it is empty.
	Revision string
	RootDir  string
	// SelectTemplate returns the template of the new pull requests from the candidates. The empty string means that the
	// template is not used. It is called only when there are multiple candidates.
	SelectTemplate func(templates []string) (string, error)
	// GitHubClient is the client of GitHub. The client with the token of gh is used if it is nil.
	GitHubClient *github.Client

	repositoryOwner string
	repositoryName  string

	stack          []*StackCommit
	remoteBranches []*github.Branch
	pullRequests   []*github.PullRequest
	prTemplate     string
	result         *SubmitResult

	stateInit           fsm.State
	stateGetToken       fsm.State
	stateGetMetadata    fsm.State
	statePushCommit     fsm.State
	stateCreatePR       fsm.State
	stateUpdatePR       fsm.State
	stateUpdateSinglePR fsm.State
	stateSyncStack      fsm.State
	stateClose          fsm.State
}

func NewPullRequestSubmitter() *PullRequestSubmitter {
	const (
		stateInit fsm.State = iota
		stateGetToken
		stateGetMetadata
		statePushCommit
		stateCreatePR
		stateUpdatePR
		stateUpdateSinglePR
		stateSyncStack
		stateClose
	)
	c := &PullRequestSubmitter{
		result:              &SubmitResult{},
		stateInit:           stateInit,
		stateGetToken:       stateGetToken,
		stateGetMetadata:    stateGetMetadata,
		statePushCommit:     statePushCommit,
		stateCreatePR:       stateCreatePR,
		stateUpdatePR:       stateUpdatePR,
		stateUpdateSinglePR: stateUpdateSinglePR,
		stateSyncStack:      stateSyncStack,
		stateClose:          stateClose,
	}
	c.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateInit:           c.init,
			stateGetToken:       c.getToken,
			stateGetMetadata:    c.getMetadata,
			statePushCommit:     c.pushCommit,
			stateCreatePR:       c.createPR,
			stateUpdatePR:       c.updatePR,
			stateUpdateSinglePR: c.updateSinglePR,
			stateSyncStack:      c.syncStack,
			stateClose:          c.close,
		},
		stateInit,
		stateClose,
	)
	c.FSM.DisableErrorOutput = true
	return c
}

// Submit pushes the commits and creates or updates the pull requests. The submitter can't be reused.
func (c *PullRequestSubmitter) Submit(ctx context.Context) (*SubmitResult, error) {
	if err := c.LoopContext(ctx); err != nil {
		return c.result, err
	}
	return c.result, nil
}

func (c *PullRequestSubmitter) init(ctx context.Context) (fsm.State, error) {
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
		c.repositoryOwner, c.repositoryName, err = GitHubRepository(ctx, c.Dir)
		if err != nil {
			return fsm.Error(err)
		}
	}

	return fsm.Next(c.stateGetToken)
}

func (c *PullRequestSubmitter) getToken(ctx context.Context) (fsm.State, error) {
	if c.GitHubClient != nil {
		return fsm.Next(c.stateGetMetadata)
	}
	token, err := githubutil.CLIToken(ctx)
	if err != nil {
		return fsm.Error(err)
	}

	if token == "" {
		return fsm.Error(xerrors.New("could not get api token"))
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	c.GitHubClient = github.NewClient(oauth2.NewClient(ctx, ts))

	return fsm.Next(c.stateGetMetadata)
}

func (c *PullRequestSubmitter) getMetadata(ctx context.Context) (fsm.State, error) {
	var wg sync.WaitGroup
	gotError := false
	if c.DefaultBranch == "" {
		wg.Go(func() {

			v, err := DefaultBranch(ctx, c.GitHubClient, c.Dir, c.repositoryOwner, c.repositoryName)
			if err != nil {
				gotError = true
				return
			}
			c.DefaultBranch = v
		})
	}

	wg.Go(func() {
		logger.Log.Debug("Retrieve the branch list")
		branches, _, err := c.GitHubClient.Repositories.ListBranches(ctx, c.repositoryOwner, c.repositoryName, &github.BranchListOptions{})
		if err != nil {
			logger.Log.Error("Could not get the branch list of the remote repository", logger.E(err))
			gotError = true
			return
		}
		c.remoteBranches = branches
	})

	wg.Go(func() {
		logger.Log.Debug("Retrieve pull requests")
		pullRequests, _, err := c.GitHubClient.PullRequests.List(ctx, c.repositoryOwner, c.repositoryName, &github.PullRequestListOptions{})
		if err != nil {
			logger.Log.Error("Could not get pull requests", logger.E(err))
			gotError = true
			return
		}
		c.pullRequests = pullRequests
	})
	wg.Wait()

	if gotError {
		return fsm.Error(xerrors.New(""))
	}
	return fsm.Next(c.statePushCommit)
}

func (c *PullRequestSubmitter) pushCommit(ctx context.Context) (fsm.State, error) {
	// Get all commits in current branch
	stack, err := c.getStack(ctx, true)
	if err != nil {
		return fsm.Error(err)
	}
	c.stack = stack

	// Push all commits of current branch
	var pushChanges []string
	var changedPR []*StackCommit
	for i, v := range c.stack {
		if c.SinglePR && i > 0 {
			break
		}
		if len(v.Bookmarks) == 0 {
			logger.Log.Debug("Will create branch", slog.String("change_id", v.ChangeID))
			pushChanges = append(pushChanges, v.ChangeID)
		} else {
			var found bool
		RemoteBranch:
			for _, r := range c.remoteBranches {
				for _, b := range v.Bookmarks {
					if r.GetName() == b.Name {
						found = true
						break RemoteBranch
					}
				}
			}
			if !found {
				pushChanges = append(pushChanges, v.ChangeID)
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
				logger.Log.Debug("Will update branch", slog.String("change_id", h.ChangeID))
				pushChanges = append(pushChanges, h.ChangeID)
				changedPR = append(changedPR, h)
				break
			}
		}
	}
	if len(pushChanges) > 0 {
		for _, v := range pushChanges {
			c.result.Pushed = append(c.result.Pushed, v)
			args := []string{"git", "push", fmt.Sprintf("--change=%s", v)}
			if c.DryRun {
				args = append(args, "--dry-run")
			}
			stdout, stderr, err := runWithStderr(ctx, c.Dir, args...)
			if err != nil {
				return fsm.Error(err)
			}
			logger.Log.Debug("Pushed", slog.String("change_id", v), slog.String("stdout", string(stdout)), slog.String("stderr", string(stderr)))
		}

		// Get all commits because the stack has been changed.
		logger.Log.Debug("Re-fetch commits from jj")
		stack, err := c.getStack(ctx, true)
		if err != nil {
			return fsm.Error(err)
		}
		c.stack = stack

		// Comment PR
		if !c.DryRun {
			for _, pr := range changedPR {
				if pr.pullRequest == nil {
					continue
				}
				// If the state of the pull request is draft, we don't need to make the comment.
				if pr.pullRequest.Draft {
					continue
				}

				body := fmt.Sprintf("Update changes: https://github.com/%s/%s/compare/%s..%s", c.repositoryOwner, c.repositoryName, pr.pullRequest.HeadSHA, pr.CommitID)
				logger.Log.Debug("Make a new comment", slog.Int("number", pr.pullRequest.ID))
				_, _, err = c.GitHubClient.Issues.CreateComment(ctx, c.repositoryOwner, c.repositoryName, pr.pullRequest.ID, &github.IssueComment{Body: &body})
				if err != nil {
					return fsm.Error(xerrors.WithStack(err))
				}
			}
		}
	}

	return fsm.Next(c.stateCreatePR)
}

func (c *PullRequestSubmitter) getStack(ctx context.Context, withoutNoSend bool) ([]*StackCommit, error) {
	commits, err := Stack(ctx, withoutNoSend, c.Dir, c.DefaultBranch, c.Revision)
	if err != nil {
		return nil, err
	}

	if len(commits) > 9 && !c.Force {
		return nil, xerrors.Definef("there are %d commits in the stack.", len(commits)).WithStack()
	}
	if len(c.pullRequests) > 0 {
		for _, v := range commits {
		PullRequest:
			for _, pr := range c.pullRequests {
				for _, b := range v.Bookmarks {
					if pr.Head.GetRef() == b.Name {
						v.pullRequest = newPullRequest(pr)
						break PullRequest
					}
				}
			}
		}
	}

	return commits, nil
}

func (c *PullRequestSubmitter) createPR(ctx context.Context) (fsm.State, error) {
	// Find pull request template
	if c.RootDir == "" {
		cmd := exec.CommandContext(ctx, "jj", "workspace", "root")
		cmd.Dir = c.Dir
		buf, err := cmd.CombinedOutput()
		if err != nil {
			return fsm.Error(xerrors.WithStack(err))
		}
		c.RootDir = strings.TrimSpace(string(buf))
	}
	templates, err := PullRequestTemplates(c.RootDir)
	if err != nil {
		return fsm.Error(err)
	}

	var template string
	// Create pull requests
	if c.SinglePR {
		if c.stack[0].pullRequest != nil {
			return fsm.Next(c.stateUpdateSinglePR)
		}
		if pr, err := c.findExistingPR(ctx, c.stack[0].Bookmarks[0].Name); err != nil {
			return fsm.Error(err)
		} else if pr != nil {
			c.result.add(c.stack[0].ChangeID, pr.URL, PullRequestExisting)
			c.stack[0].pullRequest = pr
			return fsm.Next(c.stateUpdateSinglePR)
		}

		if template == "" && len(templates) > 0 {
			template, err = c.pickTemplate(templates, c.RootDir)
			if err != nil {
				return fsm.Error(err)
			}
		}

		if c.DryRun {
			c.result.add(c.stack[0].ChangeID, "", PullRequestCreated)
		} else {
			title, body := splitDescription(c.stack[0].Description)
			description := template
			if body != "" {
				description = body + "\n" + template
			}
			pr, _, err := c.GitHubClient.PullRequests.Create(ctx, c.repositoryOwner, c.repositoryName, &github.NewPullRequest{
				Title: new(title),
				Body:  new(description),
				Head:  new(c.stack[0].Bookmarks[0].Name),
				Base:  new(c.DefaultBranch),
				Draft: new(true),
			})
			if err != nil {
				return fsm.Error(xerrors.WithStack(err))
			}
			c.stack[0].pullRequest = newPullRequest(pr)
			c.result.add(c.stack[0].ChangeID, pr.GetHTMLURL(), PullRequestCreated)
		}

		return fsm.Next(c.stateUpdateSinglePR)
	}
	// Scan reverse order to create PR for older commit first.
	for i := len(c.stack) - 1; i >= 0; i-- {
		v := c.stack[i]
		if v.pullRequest != nil {
			continue
		}
		if pr, err := c.findExistingPR(ctx, v.Bookmarks[0].Name); err != nil {
			return fsm.Error(err)
		} else if pr != nil {
			c.result.add(v.ChangeID, pr.URL, PullRequestExisting)
			v.pullRequest = pr
			continue
		}
		if template == "" && len(templates) > 0 {
			template, err = c.pickTemplate(templates, c.RootDir)
			if err != nil {
				return fsm.Error(err)
			}
		}

		// There is no pull request for the commit.
		// We need to create a pull request.
		if c.DryRun {
			c.result.add(v.ChangeID, "", PullRequestCreated)
		} else {
			baseBranch := c.DefaultBranch
			if i != len(c.stack)-1 {
				baseBranch = c.stack[i+1].Bookmarks[0].Name
			}
			title, body := splitDescription(v.Description)
			description := template
			if body != "" {
				description = body + "\n" + template
			}

			pr, _, err := c.GitHubClient.PullRequests.Create(ctx, c.repositoryOwner, c.repositoryName, &github.NewPullRequest{
				Title: new(title),
				Body:  new(description),
				Head:  new(v.Bookmarks[0].Name),
				Base:  new(baseBranch),
				Draft: new(true),
			})
			if err != nil {
				return fsm.Error(xerrors.WithStack(err))
			}
			v.pullRequest = newPullRequest(pr)
			c.result.add(v.ChangeID, pr.GetHTMLURL(), PullRequestCreated)
		}
	}

	return fsm.Next(c.stateUpdatePR)
}

// findExistingPR returns the open pull request whose head is branch, or nil
// when none exists. The pre-fetched pull request list can miss a pull request
// (for example when it is not on the first page), so we query GitHub directly
// before creating a new one to avoid a "pull request already exists" error.
func (c *PullRequestSubmitter) findExistingPR(ctx context.Context, branch string) (*pullRequest, error) {
	prs, _, err := c.GitHubClient.PullRequests.List(ctx, c.repositoryOwner, c.repositoryName, &github.PullRequestListOptions{
		State: "open",
		Head:  fmt.Sprintf("%s:%s", c.repositoryOwner, branch),
	})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	for _, pr := range prs {
		if pr.GetHead().GetRef() == branch {
			return newPullRequest(pr), nil
		}
	}
	return nil, nil
}

func (c *PullRequestSubmitter) pickTemplate(templates []string, repoRoot string) (string, error) {
	if c.prTemplate != "" {
		return c.prTemplate, nil
	}

	var templateFile string
	if len(templates) == 1 {
		templateFile = templates[0]
	} else if len(templates) > 0 {
		if c.SelectTemplate == nil {
			return "", xerrors.New("please pick the template")
		}
		v, err := c.SelectTemplate(templates)
		if err != nil {
			return "", err
		}
		if v != "" && !slices.Contains(templates, v) {
			return "", xerrors.Newf("%s is not the template", v)
		}
		templateFile = v
	}
	var template string
	if templateFile != "" {
		logger.Log.Debug("Read template", slog.String("path", templateFile))
		buf, err := os.ReadFile(templateFile)
		if err != nil {
			return "", xerrors.WithStack(err)
		}
		template = string(buf)
		if _, err := os.Lstat(filepath.Join(repoRoot, ".jj", lastPickedTemplateFile)); os.IsNotExist(err) {
			shortPath := strings.TrimPrefix(templateFile, repoRoot)
			err = os.WriteFile(filepath.Join(repoRoot, ".jj", lastPickedTemplateFile), []byte(shortPath), 0644)
			if err != nil {
				return "", xerrors.WithStack(err)
			}
		}
	}

	c.prTemplate = template
	return template, nil
}

func (c *PullRequestSubmitter) updatePR(ctx context.Context) (fsm.State, error) {
	for i := len(c.stack) - 1; i >= 0; i-- {
		v := c.stack[i]
		if v.pullRequest == nil {
			if !c.DryRun {
				logger.Log.Error("BUG: The pull request must update. but we can't find the pull request.")
			}
			logger.Log.Info("Skip to update the pull request")
			continue
		}
		var updatedPR github.PullRequest

		var needUpdateBaseBranch, needUpdateTitle, needUpdateBody bool
		if i != len(c.stack)-1 && v.pullRequest.Base != c.stack[i+1].Bookmarks[0].Name {
			needUpdateBaseBranch = true
			updatedPR.Base = &github.PullRequestBranch{Ref: new(c.stack[i+1].Bookmarks[0].Name)}
		}
		if i := strings.Index(v.Description, "\n"); i > 0 {
			if v.pullRequest.Title != v.Description[:i] {
				updatedPR.Title = new(v.Description[:i])
				needUpdateTitle = true
			}
		}
		_, descBody := splitDescription(v.Description)
		body := v.pullRequest.Body
		if body == "" {
			body = descBody
		}
		if body != v.pullRequest.Body {
			updatedPR.Body = new(body)
			needUpdateBody = true
		}

		if !v.pullRequest.Draft && needUpdateBaseBranch || needUpdateTitle || needUpdateBody {
			// Update the pull request
			c.result.add(v.ChangeID, v.pullRequest.URL, PullRequestUpdated)
			logger.Log.Debug("Update pull request reason", slog.Bool("base_branch", needUpdateBaseBranch), slog.Bool("title", needUpdateTitle), slog.Bool("body", needUpdateBody))
			if !c.DryRun {
				pr, _, err := c.GitHubClient.PullRequests.Edit(ctx, c.repositoryOwner, c.repositoryName, v.pullRequest.ID, &updatedPR)
				if err != nil {
					return fsm.Error(xerrors.WithStack(err))
				}
				v.pullRequest = newPullRequest(pr)
			}
		}
	}

	return fsm.Next(c.stateSyncStack)
}

func (c *PullRequestSubmitter) updateSinglePR(_ context.Context) (fsm.State, error) {
	if c.stack[0].pullRequest == nil {
		if !c.DryRun {
			logger.Log.Error("BUG: Couldn't find the pull request.")
		}
		return fsm.Next(c.stateClose)
	}
	return fsm.Next(c.stateClose)
}

func (c *PullRequestSubmitter) close(_ context.Context) (fsm.State, error) {
	return fsm.Finish()
}

func (c *PullRequestSubmitter) listStacks(ctx context.Context) ([]*ghStack, error) {
	path := fmt.Sprintf("repos/%s/%s/stacks", c.repositoryOwner, c.repositoryName)
	req, err := c.GitHubClient.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	var stacks []*ghStack
	if _, err := c.GitHubClient.Do(ctx, req, &stacks); err != nil {
		return nil, xerrors.WithStack(err)
	}
	return stacks, nil
}

func (c *PullRequestSubmitter) createStack(ctx context.Context, prNumbers []int) (*ghStack, error) {
	path := fmt.Sprintf("repos/%s/%s/stacks", c.repositoryOwner, c.repositoryName)
	req, err := c.GitHubClient.NewRequest(http.MethodPost, path, &ghStackRequest{PullRequests: prNumbers})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	var stack ghStack
	if _, err := c.GitHubClient.Do(ctx, req, &stack); err != nil {
		return nil, xerrors.WithStack(err)
	}
	return &stack, nil
}

func (c *PullRequestSubmitter) addToStack(ctx context.Context, number int, prNumbers []int) (*ghStack, error) {
	path := fmt.Sprintf("repos/%s/%s/stacks/%d/add", c.repositoryOwner, c.repositoryName, number)
	req, err := c.GitHubClient.NewRequest(http.MethodPost, path, &ghStackRequest{PullRequests: prNumbers})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	var stack ghStack
	if _, err := c.GitHubClient.Do(ctx, req, &stack); err != nil {
		return nil, xerrors.WithStack(err)
	}
	return &stack, nil
}

func (c *PullRequestSubmitter) unstack(ctx context.Context, number int) error {
	path := fmt.Sprintf("repos/%s/%s/stacks/%d/unstack", c.repositoryOwner, c.repositoryName, number)
	req, err := c.GitHubClient.NewRequest(http.MethodPost, path, nil)
	if err != nil {
		return xerrors.WithStack(err)
	}
	if _, err := c.GitHubClient.Do(ctx, req, nil); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

// syncStack reflects the local stack onto GitHub by using the Stacks API. It
// creates a stack, appends new pull requests to an existing stack, or, when the
// order changed, recreates the stack in the new order. Failures are logged as a
// warning and do not fail the command because the pull requests have already
// been created.
func (c *PullRequestSubmitter) syncStack(ctx context.Context) (fsm.State, error) {
	// Collect the pull request numbers in stack order, from the bottom (closest
	// to the trunk) to the top.
	var prNumbers []int
	for i := len(c.stack) - 1; i >= 0; i-- {
		if c.stack[i].pullRequest != nil {
			prNumbers = append(prNumbers, c.stack[i].pullRequest.ID)
		}
	}

	// A stack needs at least two pull requests.
	if len(prNumbers) < 2 || c.DryRun {
		return fsm.Next(c.stateClose)
	}

	stacks, err := c.listStacks(ctx)
	if err != nil {
		logger.Log.Warn("Could not list stacks. Skip to make a stack", logger.E(err))
		return fsm.Next(c.stateClose)
	}

	existing := findStackContaining(stacks, prNumbers)
	if existing == nil {
		if _, err := c.createStack(ctx, prNumbers); err != nil {
			logger.Log.Warn("Could not create a stack", logger.E(err))
		} else {
			logger.Log.Info("Created a stack", slog.Int("pull_requests", len(prNumbers)))
		}
		return fsm.Next(c.stateClose)
	}

	current := existing.prNumbers()
	if slices.Equal(current, prNumbers) {
		// Already in sync.
		return fsm.Next(c.stateClose)
	}

	// When the local stack is the remote stack with new pull requests appended
	// onto the top, the add endpoint can express it.
	if delta, ok := appendDelta(current, prNumbers); ok {
		if _, err := c.addToStack(ctx, existing.Number, delta); err != nil {
			logger.Log.Warn("Could not add pull requests to the stack", logger.E(err))
		} else {
			logger.Log.Info("Added pull requests to the stack", slog.Int("pull_requests", len(delta)))
		}
		return fsm.Next(c.stateClose)
	}

	// The order changed (or a pull request was removed). The Stacks API cannot
	// express a reorder, so dissolve the stack and recreate it in the new order.
	if err := c.unstack(ctx, existing.Number); err != nil {
		logger.Log.Warn("Could not unstack the existing stack", logger.E(err))
		return fsm.Next(c.stateClose)
	}
	if _, err := c.createStack(ctx, prNumbers); err != nil {
		logger.Log.Warn("Could not recreate the stack", logger.E(err))
	} else {
		logger.Log.Info("Recreated the stack", slog.Int("pull_requests", len(prNumbers)))
	}
	return fsm.Next(c.stateClose)
}

// DefaultBranch returns the name of the branch of trunk() in the repository of dir. It is retrieved from GitHub if
// trunk() doesn't have the remote bookmark.
func DefaultBranch(ctx context.Context, ghClient *github.Client, dir, owner, name string) (string, error) {
	logger.Log.Debug("Retrieve repository metadata")
	cmd := exec.CommandContext(ctx, "jj", "show", "trunk()", "--template", "remote_bookmarks")
	cmd.Dir = dir
	buf, err := cmd.CombinedOutput()
	if err == nil {
		if before, _, ok := bytes.Cut(buf, []byte("@")); ok {
			logger.Log.Debug("Get default branch from local repository")
			return string(before), nil
		}
	}

	repo, _, err := ghClient.Repositories.Get(ctx, owner, name)
	if err != nil {
		logger.Log.Error("Could not get repository metadata from api.github.com", logger.E(err))
		return "", err
	}
	return repo.GetDefaultBranch(), nil
}

// Stack returns the commits from the default branch to the change. The first commit is the newest commit. The change is
// the working copy if changeID is empty. The commits whose description starts with "no-send:" or "WIP:" are excluded if
// withoutNoSend is true.
func Stack(ctx context.Context, withoutNoSend bool, dir, defaultBranch, changeID string) ([]*StackCommit, error) {
	if changeID == "" {
		cmd := exec.CommandContext(ctx, "jj", "log", "--revisions", "@", "--no-graph", "--template", "change_id")
		cmd.Dir = dir
		buf, err := cmd.CombinedOutput()
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		changeID = strings.TrimSpace(string(buf))
	}

	cmd := exec.CommandContext(ctx, "jj", "log", "--revisions", fmt.Sprintf(stackRevsets, defaultBranch, changeID), "--no-graph", "--template", stackTemplate)
	cmd.Dir = dir
	buf, err := cmd.CombinedOutput()
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	var commits []*StackCommit
	scanner := bufio.NewScanner(bytes.NewReader(buf))
	for scanner.Scan() {
		c := &StackCommit{}
		if err := json.Unmarshal(scanner.Bytes(), c); err != nil {
			return nil, xerrors.WithStack(err)
		}
		if !(withoutNoSend && (strings.HasPrefix(c.Description, noSendTag) || strings.HasPrefix(c.Description, wipTag))) {
			logger.Log.Debug("Stack", slog.String("change_id", c.ChangeID), slog.Any("bookmarks", c.Bookmarks))
			commits = append(commits, c)
		}
	}

	return commits, nil
}

// GitHubRepository returns the owner and the name of the repository on GitHub from the remotes of the repository.
func GitHubRepository(ctx context.Context, dir string) (string, string, error) {
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
		if before, ok := strings.CutSuffix(name, ".git"); ok {
			name = before
		}
		break
	}
	if owner == "" || name == "" {
		return "", "", xerrors.New("could not find repository owner and name")
	}

	return owner, name, nil
}

// PullRequestTemplates returns the paths of the templates of the pull request in the repository. It returns only the
// template picked last time if it exists.
func PullRequestTemplates(root string) ([]string, error) {
	if _, err := os.Lstat(filepath.Join(root, ".github")); os.IsNotExist(err) {
		return nil, nil
	}

	if _, err := os.Lstat(filepath.Join(root, ".jj", lastPickedTemplateFile)); err == nil {
		buf, err := os.ReadFile(filepath.Join(root, ".jj", lastPickedTemplateFile))
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		if _, err := os.Lstat(filepath.Join(root, string(buf))); err == nil {
			return []string{filepath.Join(root, string(buf))}, nil
		}
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
			logger.Log.Debug("Template found", slog.String("path", path))
			templates = append(templates, path)
		}
		if strings.Contains(path, ".github/PULL_REQUEST_TEMPLATE/") {
			logger.Log.Debug("Template found", slog.String("path", path))
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

// SubmitResult is the result of PullRequestSubmitter.Submit.
type SubmitResult struct {
	// Pushed is the change IDs pushed to the remote.
	Pushed []string
	// PullRequests is in order of the operations. The pull request of the change may appear twice when it is found and
	// updated.
	PullRequests []*SubmittedPullRequest
}

func (r *SubmitResult) add(changeID, url string, action PullRequestAction) {
	r.PullRequests = append(r.PullRequests, &SubmittedPullRequest{ChangeID: changeID, URL: url, Action: action})
}

type PullRequestAction int

// SubmittedPullRequest is the pull request handled by PullRequestSubmitter. URL is empty if the pull request is not
// created by the dry run.
type SubmittedPullRequest struct {
	ChangeID string
	URL      string
	Action   PullRequestAction
}

// StackCommit is the commit in the stack. ChangeID and CommitID are the full IDs.
type StackCommit struct {
	ChangeID    string           `json:"change_id"`
	CommitID    string           `json:"commit_id"`
	Description string           `json:"description"`
	Bookmarks   []*StackBookmark `json:"bookmarks"`

	pullRequest *pullRequest
}

type StackBookmark struct {
	Name   string   `json:"name"`
	Target []string `json:"target"`
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

// ghStack is a subset of the stack object of the GitHub Stacks API.
// go-github does not support the Stacks API yet, so we call it directly through
// the client's request plumbing.
type ghStack struct {
	ID           int         `json:"id"`
	Number       int         `json:"number"`
	NodeID       string      `json:"node_id"`
	URL          string      `json:"url"`
	Open         bool        `json:"open"`
	PullRequests []ghStackPR `json:"pull_requests"`
}

// prNumbers returns the ordered pull request numbers in the stack, from the
// bottom (closest to the base branch) to the top.
func (s *ghStack) prNumbers() []int {
	out := make([]int, len(s.PullRequests))
	for i, v := range s.PullRequests {
		out[i] = v.Number
	}
	return out
}

type ghStackPR struct {
	Number int `json:"number"`
}

type ghStackRequest struct {
	PullRequests []int `json:"pull_requests"`
}

// stripTrailers returns desc with the trailing trailer block removed. A
// paragraph (a run of non-blank lines preceded by a blank line) is considered
// a trailer paragraph only when every line in it matches the "Key: value"
// pattern. Everything from the first trailer paragraph to the end of desc
// (and the blank line(s) before it) is dropped.
func stripTrailers(desc string) string {
	lines := strings.Split(desc, "\n")
	paraStart := -1
	cut := 0
	for i := 0; i <= len(lines); i++ {
		if i == len(lines) || lines[i] == "" {
			if paraStart > 0 && isTrailerParagraph(lines[paraStart:i]) {
				return strings.Join(lines[:cut], "\n")
			}
			if paraStart != -1 {
				cut = i
			}
			paraStart = -1
			continue
		}
		if paraStart == -1 {
			paraStart = i
		}
	}
	return desc
}

func isTrailerParagraph(lines []string) bool {
	if len(lines) == 0 {
		return false
	}
	for _, l := range lines {
		if !trailerLineRe.MatchString(l) {
			return false
		}
	}
	return true
}

// splitDescription splits a jj commit description into a subject (the first
// line) and a body (the remainder with trailers stripped). The body is empty
// when desc has no body content or contains only trailers after the subject.
func splitDescription(desc string) (title, body string) {
	desc = stripTrailers(desc)
	if i := strings.Index(desc, "\n"); i > 0 {
		title = desc[:i]
		if len(desc) > i+2 {
			body = desc[i+2:]
		}
		return
	}
	title = desc
	return
}

// findStackContaining returns the stack that already contains any of the given
// pull request numbers, or nil when none of them are stacked.
func findStackContaining(stacks []*ghStack, prNumbers []int) *ghStack {
	for _, s := range stacks {
		for _, n := range s.prNumbers() {
			if slices.Contains(prNumbers, n) {
				return s
			}
		}
	}
	return nil
}

// appendDelta returns the pull request numbers that desired adds on top of
// current when desired is current followed by zero or more new numbers. ok is
// false when desired is not a clean append onto current, which happens on a
// reorder or a removal.
func appendDelta(current, desired []int) (delta []int, ok bool) {
	if len(desired) <= len(current) {
		return nil, false
	}
	if !slices.Equal(current, desired[:len(current)]) {
		return nil, false
	}
	return desired[len(current):], true
}
