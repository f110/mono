package main

import (
	"bufio"
	"bytes"
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

	"github.com/google/go-github/v49/github"
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
)

type jujutsuPRSubmitCommand struct {
	*fsm.FSM

	Dir           string
	Repository    string
	DefaultBranch string
	DryRun        bool
	Force         bool
	DisplayStack  bool
	SinglePR      bool

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
	stateUpdateSinglePR
	stateClose
)

func newCommand() *jujutsuPRSubmitCommand {
	c := &jujutsuPRSubmitCommand{}
	c.FSM = fsm.NewFSM(
		map[fsm.State]fsm.StateFunc{
			stateInit:           c.init,
			stateGetToken:       c.getToken,
			stateGetMetadata:    c.getMetadata,
			stateDisplayStack:   c.displayStack,
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
	fs.Bool("display-stack", "Only display the stack").Var(&c.DisplayStack)
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

	return fsm.Next(stateGetToken)
}

func (c *jujutsuPRSubmitCommand) displayStack(ctx context.Context) (fsm.State, error) {
	commits, err := c.getStack(ctx, false)
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
		if v.Branch == "" {
			logger.Log.Debug("Will create branch", zap.String("change_id", v.ChangeID))
			pushArgs = append(pushArgs, []string{fmt.Sprintf("--change=%s", v.ChangeID)})
		} else {
			var found bool
			for _, r := range c.remoteBranches {
				if r.GetName() == v.Branch {
					found = true
					break
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

	return fsm.Next(stateCreatePR)
}

// getStack returns commits of current stack. The first commit is the newest commit.
func (c *jujutsuPRSubmitCommand) getStack(ctx context.Context, withoutNoSend bool) (stackedCommit, error) {
	const logTemplate = `change_id ++ "\\" ++ commit_id ++ "\\[" ++ bookmarks ++ "]\\" ++ description ++ "\\\n"`
	cmd := exec.CommandContext(ctx, "jj", "log", "--revisions", fmt.Sprintf(stackRevsets, c.DefaultBranch), "--no-graph", "--template", logTemplate)
	cmd.Dir = c.Dir
	buf, err := cmd.CombinedOutput()
	if err != nil {
		return nil, xerrors.WithStack(err)
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
			if !(withoutNoSend && strings.HasPrefix(description, noSendTag)) {
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

	if len(commits) > 9 && !c.Force {
		return nil, xerrors.Definef("there are %d commits in the stack.", len(commits)).WithStack()
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
	if c.SinglePR {
		if c.stack[0].PullRequest != nil {
			return fsm.Next(stateUpdateSinglePR)
		}

		if template == "" && len(templates) > 0 {
			template, err = c.pickTemplate(templates, repoRoot)
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
				Head:  github.String(c.stack[0].Branch),
				Base:  github.String(c.DefaultBranch),
				Draft: github.Bool(true),
			})
			if err != nil {
				return fsm.Error(xerrors.WithStack(err))
			}
			c.stack[0].PullRequest = newPullRequest(pr)
			fmt.Printf("Created: %s\n", pr.GetHTMLURL())
		}

		return fsm.Next(stateUpdateSinglePR)
	}
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
			} else {
				title = v.Description
				description = template
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

	return fsm.Next(stateClose)
}

func (c *jujutsuPRSubmitCommand) updateSinglePR(_ context.Context) (fsm.State, error) {
	if c.stack[0].PullRequest == nil {
		if !c.DryRun {
			logger.Log.Error("BUG: Couldn't find the pull request.")
		}
		return fsm.Next(stateClose)
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
	cmd := &cli.Command{
		Use: "jj-pr-submit",
		Run: func(ctx context.Context, _ *cli.Command, _ []string) error {
			return c.LoopContext(ctx)
		},
	}
	c.flags(cmd.Flags())

	return cmd.Execute(os.Args)
}

func main() {
	if err := jujutsuPRSubmit(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
