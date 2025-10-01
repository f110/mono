package main

import (
	"context"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/google/go-github/v73/github"
	"go.f110.dev/xerrors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/oauth2"

	"go.f110.dev/mono/go/cli"
	"go.f110.dev/mono/go/fsm"
	"go.f110.dev/mono/go/logger"
)

const (
	stackRevsets           = "ancestors(latest(%s@origin) & remote_bookmarks())..@ ~ empty()"
	stackNavigatorHeader   = "\n---\n\nPull request chain:\n\n"
	lastPickedTemplateFile = ".last_template_file"
	noSendTag              = "no-send:"
	wipTag                 = "WIP:"
)

type jujutsuPRSubmitCommand struct {
	*fsm.FSM

	Dir           string
	Repository    string
	DefaultBranch string
	DryRun        bool
	Force         bool
	SinglePR      bool
	RootDir       string

	repositoryOwner string
	repositoryName  string
	token           string
	ghClient        *github.Client

	stack          stackedCommit
	remoteBranches []*github.Branch
	pullRequests   []*github.PullRequest
	prTemplate     string

	stateInit           fsm.State
	stateGetToken       fsm.State
	stateGetMetadata    fsm.State
	statePushCommit     fsm.State
	stateCreatePR       fsm.State
	stateUpdatePR       fsm.State
	stateUpdateSinglePR fsm.State
	stateClose          fsm.State
}

func newSubmitCommand() *jujutsuPRSubmitCommand {
	const (
		stateInit fsm.State = iota
		stateGetToken
		stateGetMetadata
		statePushCommit
		stateCreatePR
		stateUpdatePR
		stateUpdateSinglePR
		stateClose
	)
	c := &jujutsuPRSubmitCommand{
		stateInit:           stateInit,
		stateGetToken:       stateGetToken,
		stateGetMetadata:    stateGetMetadata,
		statePushCommit:     statePushCommit,
		stateCreatePR:       stateCreatePR,
		stateUpdatePR:       stateUpdatePR,
		stateUpdateSinglePR: stateUpdateSinglePR,
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
			stateClose:          c.close,
		},
		stateInit,
		stateClose,
	)
	c.FSM.DisableErrorOutput = true
	return c
}

func (c *jujutsuPRSubmitCommand) flags(fs *cli.FlagSet) {
	fs.String("dir", "Working directory").Var(&c.Dir)
	fs.String("repository", "Repository name. If not specified, try to get from remote url").Var(&c.Repository)
	fs.String("default-branch", "Default branch name. If not specified, get from API").Var(&c.DefaultBranch)
	fs.Bool("dry-run", "Not impact on remote").Var(&c.DryRun)
	fs.Bool("force", "Push commits when there are more than 10 commits in the stack").Var(&c.Force)
	fs.Bool("single-pr", "Make the PR in multiple commits").Var(&c.SinglePR)
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

	return fsm.Next(c.stateGetToken)
}

func (c *jujutsuPRSubmitCommand) getToken(ctx context.Context) (fsm.State, error) {
	token, err := getToken(ctx)
	if err != nil {
		return fsm.Error(err)
	}

	if token == "" {
		return fsm.Error(xerrors.New("could not get api token"))
	}
	c.token = token
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: c.token})
	c.ghClient = github.NewClient(oauth2.NewClient(ctx, ts))

	return fsm.Next(c.stateGetMetadata)
}

func (c *jujutsuPRSubmitCommand) getMetadata(ctx context.Context) (fsm.State, error) {
	var wg sync.WaitGroup
	gotError := false
	if c.DefaultBranch == "" {
		wg.Add(1)
		go func() {
			defer wg.Done()

			v, err := getDefaultBranch(ctx, c.ghClient, c.repositoryOwner, c.repositoryName)
			if err != nil {
				gotError = true
				return
			}
			c.DefaultBranch = v
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
	return fsm.Next(c.statePushCommit)
}

type commit struct {
	ChangeID string `json:"change_id"`
	CommitID string `json:"commit_id"`
	// Branch is local branch name if exists.
	Branch      string      `json:"-"`
	Description string      `json:"description"`
	Bookmarks   []*bookmark `json:"bookmarks"`

	PullRequest *pullRequest
}

type bookmark struct {
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

type stackedCommit []*commit

func (c *jujutsuPRSubmitCommand) pushCommit(ctx context.Context) (fsm.State, error) {
	// Get all commits in current branch
	stack, err := c.getStack(ctx, true)
	if err != nil {
		return fsm.Error(err)
	}
	c.stack = stack

	// Push all commits of current branch
	var pushArgs [][]string
	var changedPR []*commit
	for i, v := range c.stack {
		if c.SinglePR && i > 0 {
			break
		}
		if len(v.Bookmarks) == 0 {
			logger.Log.Debug("Will create branch", zap.String("change_id", v.ChangeID))
			pushArgs = append(pushArgs, []string{fmt.Sprintf("--change=%s", v.ChangeID)})
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
				pushArgs = append(pushArgs, []string{fmt.Sprintf("--change=%s", v.ChangeID)})
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
				pushArgs = append(pushArgs, []string{fmt.Sprintf("--change=%s", h.ChangeID)})
				changedPR = append(changedPR, h)
				break
			}
		}
	}
	if len(pushArgs) > 0 {
		for _, v := range pushArgs {
			args := append([]string{"git", "push"}, v...)
			logger.Log.Debug("Push commits to create branches")
			cmd := exec.CommandContext(ctx, "jj", args...)
			cmd.Dir = c.Dir
			if c.DryRun {
				cmd.Args = append(cmd.Args, "--dry-run")
			}
			cmd.Stdout = os.Stdout
			if logger.Log.Level() == zapcore.DebugLevel {
				cmd.Stderr = os.Stderr
			}
			if err = cmd.Run(); err != nil {
				return fsm.Error(xerrors.WithStack(err))
			}
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

	return fsm.Next(c.stateCreatePR)
}

func (c *jujutsuPRSubmitCommand) getStack(ctx context.Context, withoutNoSend bool) (stackedCommit, error) {
	commits, err := getStack(ctx, withoutNoSend, c.Dir, c.DefaultBranch)
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
						v.PullRequest = newPullRequest(pr)
						break PullRequest
					}
				}
			}
		}
	}

	return commits, nil
}

func (c *jujutsuPRSubmitCommand) createPR(ctx context.Context) (fsm.State, error) {
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
	templates, err := c.findPullRequestTemplate(c.RootDir)
	if err != nil {
		return fsm.Error(err)
	}

	var template string
	// Create pull requests
	if c.SinglePR {
		if c.stack[0].PullRequest != nil {
			return fsm.Next(c.stateUpdateSinglePR)
		}

		if template == "" && len(templates) > 0 {
			template, err = c.pickTemplate(templates, c.RootDir)
			if err != nil {
				return fsm.Error(err)
			}
		}

		fmt.Println("Create pull request")
		if !c.DryRun {
			var title, description string
			if i := strings.Index(c.stack[0].Description, "\n"); i > 0 {
				title = c.stack[0].Description[:i]
				if len(c.stack[0].Description) > i+2 {
					description = c.stack[0].Description[i+2:] + "\n" + template
				} else {
					description = template
				}
			} else {
				title = c.stack[0].Description
				description = template
			}
			pr, _, err := c.ghClient.PullRequests.Create(ctx, c.repositoryOwner, c.repositoryName, &github.NewPullRequest{
				Title: github.String(title),
				Body:  github.String(description),
				Head:  github.String(c.stack[0].Bookmarks[0].Name),
				Base:  github.String(c.DefaultBranch),
				Draft: github.Bool(true),
			})
			if err != nil {
				return fsm.Error(xerrors.WithStack(err))
			}
			c.stack[0].PullRequest = newPullRequest(pr)
			fmt.Printf("Created: %s\n", pr.GetHTMLURL())
		}

		return fsm.Next(c.stateUpdateSinglePR)
	}
	// Scan reverse order to create PR for older commit first.
	for i := len(c.stack) - 1; i >= 0; i-- {
		v := c.stack[i]
		if v.PullRequest != nil {
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
		fmt.Printf("Create pull request for %s\n", v.ChangeID)
		if !c.DryRun {
			baseBranch := c.DefaultBranch
			if i != len(c.stack)-1 {
				baseBranch = c.stack[i+1].Bookmarks[0].Name
			}
			var title, description string
			if i := strings.Index(v.Description, "\n"); i > 0 {
				title = v.Description[:i]
				if len(v.Description) > i+2 {
					description = v.Description[i+2:] + "\n" + template
				} else {
					description = template
				}
			} else {
				title = v.Description
				description = template
			}

			pr, _, err := c.ghClient.PullRequests.Create(ctx, c.repositoryOwner, c.repositoryName, &github.NewPullRequest{
				Title: github.String(title),
				Body:  github.String(description),
				Head:  github.String(v.Bookmarks[0].Name),
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

	return fsm.Next(c.stateUpdatePR)
}

func (c *jujutsuPRSubmitCommand) pickTemplate(templates []string, repoRoot string) (string, error) {
	if c.prTemplate != "" {
		return c.prTemplate, nil
	}

	var templateFile string
	if len(templates) == 1 {
		fmt.Println("Found one template. will use it.")
		templateFile = templates[0]
	} else if len(templates) > 0 {
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
		logger.Log.Debug("Read template", zap.String("path", templateFile))
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

func (*jujutsuPRSubmitCommand) findPullRequestTemplate(root string) ([]string, error) {
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
			logger.Log.Debug("Template found", zap.String("path", path))
			templates = append(templates, path)
		}
		if strings.Contains(path, ".github/PULL_REQUEST_TEMPLATE/") {
			logger.Log.Debug("Template found", zap.String("path", path))
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
		if i != len(c.stack)-1 && v.PullRequest.Base != c.stack[i+1].Bookmarks[0].Name {
			needUpdateBaseBranch = true
			updatedPR.Base = &github.PullRequestBranch{Ref: github.String(c.stack[i+1].Bookmarks[0].Name)}
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

		if !v.PullRequest.Draft && needUpdateBaseBranch || needUpdateTitle || needUpdateBody {
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

	return fsm.Next(c.stateClose)
}

func (c *jujutsuPRSubmitCommand) updateSinglePR(_ context.Context) (fsm.State, error) {
	if c.stack[0].PullRequest == nil {
		if !c.DryRun {
			logger.Log.Error("BUG: Couldn't find the pull request.")
		}
		return fsm.Next(c.stateClose)
	}
	return fsm.Next(c.stateClose)
}

func (c *jujutsuPRSubmitCommand) close(_ context.Context) (fsm.State, error) {
	return fsm.Finish()
}
