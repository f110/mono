package githubutil

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/go-github/v73/github"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/varptr"
)

type Mock struct {
	mu           sync.Mutex
	Repositories map[string]*Repository
}

type Repository struct {
	mu           sync.Mutex
	pullRequests []*PullRequest
	issues       []*Issue
	tags         []*Tag
	commits      []*Commit

	headCommit *Commit
	rootCommit *Commit
}

type Commit struct {
	Parents []*Commit `json:"-"`
	Files   []*File   `json:"-"`
	IsHead  bool      `json:"-"`

	files    []*File
	ghCommit *github.Commit
}

type fileType int

const (
	fileTypeRegular fileType = iota
	fileTypeDir
)

type File struct {
	Name string
	Body []byte

	sha  string
	mode fileType
}

type PullRequest struct {
	github.PullRequest

	Comments []*github.PullRequestComment `json:"-"`
}

type Issue struct {
	github.Issue

	Comments []*github.IssueComment `json:"-"`
}

type Tag struct {
	Name   string
	Commit *Commit

	ghTag *github.Tag
}

func newRepository() *Repository {
	return &Repository{}
}

func NewMock() *Mock {
	return &Mock{Repositories: make(map[string]*Repository)}
}

func (m *Mock) Repository(name string) *Repository {
	m.mu.Lock()
	defer m.mu.Unlock()

	if r, ok := m.Repositories[name]; ok {
		return r
	}

	r := newRepository()
	m.Repositories[name] = r
	return r
}

func (m *Mock) RegisteredTransport() *httpmock.MockTransport {
	tr := httpmock.NewMockTransport()
	m.RegisterResponder(tr)
	return tr
}

func (m *Mock) Client() *github.Client {
	return github.NewClient(&http.Client{Transport: m.RegisteredTransport()})
}

func (r *Repository) AssertPullRequest(t *testing.T, number int) *PullRequest {
	t.Helper()
	for _, v := range r.pullRequests {
		if v.GetNumber() == number {
			return v
		}
	}

	assert.Failf(t, "Pull request is not found", "pull request %d is not found", number)
	return nil
}

func (r *Repository) PullRequests(pullRequests ...*github.PullRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range pullRequests {
		r.pullRequests = append(r.pullRequests, &PullRequest{
			PullRequest: *v,
		})
		if v.GetNumber() == 0 {
			r.pullRequests[len(r.pullRequests)-1].Number = varptr.Ptr(r.nextIndex())
		}
	}
}

func (r *Repository) GetPullRequest(num int) *PullRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range r.pullRequests {
		if v.GetNumber() == num {
			return v
		}
	}
	return nil
}

func (r *Repository) Issues(issues ...*github.Issue) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range issues {
		r.issues = append(r.issues, &Issue{
			Issue: *v,
		})
		if v.GetNumber() == 0 {
			r.issues[len(r.issues)-1].Number = varptr.Ptr(r.nextIndex())
		}
	}
}

func (r *Repository) GetIssue(num int) *Issue {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range r.issues {
		if v.GetNumber() == num {
			return v
		}
	}
	return nil
}

func (r *Repository) Commits(commits ...*Commit) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var rootCommit *Commit
	for _, v := range commits {
		if len(v.Parents) == 0 {
			if rootCommit != nil {
				return xerrors.New("multiple root commits are found")
			}
			rootCommit = v
		}
		if v.IsHead {
			if r.headCommit != nil {
				return xerrors.New("multiple head commits are found")
			}
			r.headCommit = v
		}

		v.ghCommit = &github.Commit{SHA: varptr.Ptr(NewHash())}
		v.files = []*File{{Name: "", sha: NewHash(), mode: fileTypeDir}} // Root directory
		v.ghCommit.Tree = &github.Tree{SHA: varptr.Ptr(v.files[0].sha)}
		for _, f := range v.Files {
			if f.Name == "" {
				continue
			}

			name := f.Name
			if name[0] == '/' {
				name = name[1:]
			}
			s := strings.Split(name, "/")
			f.Name = name
			var dirs []string
			if len(s) > 1 {
				dirs = s[:len(s)-1]
			}
			for i := 1; i <= len(dirs); i++ {
				dir := strings.Join(dirs[:i], "/")
				v.addDir(dir)
			}
			v.addFile(f)
		}
	}
	if rootCommit != nil {
		if r.rootCommit != nil {
			return xerrors.New("multiple root commits are found")
		}
		r.rootCommit = rootCommit
	}
	r.commits = append(r.commits, commits...)
	return nil
}

func (r *Repository) Tags(tags ...*Tag) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range tags {
		v.ghTag = &github.Tag{
			Tag: varptr.Ptr(v.Name),
		}
	}
	r.tags = append(r.tags, tags...)
}

func (m *Mock) RegisterResponder(tr *httpmock.MockTransport) {
	m.registerIssuesService(tr)
	m.registerPullRequestService(tr)
	m.registerGitService(tr)
	m.registerRepositoriesService(tr)
}

func (m *Mock) registerPullRequestService(tr *httpmock.MockTransport) {
	// Get a pull request
	// GET /repos/octocat/example/pulls/1
	tr.RegisterRegexpResponder(http.MethodGet, regexp.MustCompile(`/repos/[^/?]+/[^/?]+/pulls/\d+$`), func(req *http.Request) (*http.Response, error) {
		r := m.findRepository(req.URL.Path)
		if r == nil {
			return newNotFoundResponse(req)
		}
		s := strings.Split(req.URL.Path, "/")
		num, err := strconv.Atoi(s[5])
		if err != nil {
			return newErrResponse(req, http.StatusBadRequest, err.Error())
		}
		pr := r.GetPullRequest(num)
		return newMockJSONResponse(req, http.StatusOK, pr)
	})
	// Create a pull request
	// POST /repos/octocat/example/pulls
	tr.RegisterRegexpResponder(http.MethodPost, regexp.MustCompile(`/repos/[^/?]+/[^/?]+/pulls$`), func(req *http.Request) (*http.Response, error) {
		r := m.findRepository(req.URL.Path)
		if r == nil {
			return newNotFoundResponse(req)
		}
		var reqPR github.NewPullRequest
		if err := json.NewDecoder(req.Body).Decode(&reqPR); err != nil {
			return newErrResponse(req, http.StatusBadRequest, err.Error())
		}

		newNumber := r.NextIndex()
		pr := &github.PullRequest{
			Number: &newNumber,
			Title:  reqPR.Title,
			Body:   reqPR.Body,
			Head: &github.PullRequestBranch{
				Ref: reqPR.Head,
			},
			Base: &github.PullRequestBranch{
				Ref: reqPR.Base,
			},
		}
		r.PullRequests(pr)
		return newMockJSONResponse(req, http.StatusOK, pr)
	})

	// Update a pull request
	// PATCH /repos/octocat/example/pulls/1
	tr.RegisterRegexpResponder(http.MethodPatch, regexp.MustCompile(`/repos/[^/?]+/[^/?]+/pulls/\d+$`), func(req *http.Request) (*http.Response, error) {
		r := m.findRepository(req.URL.Path)
		if r == nil {
			return newNotFoundResponse(req)
		}
		s := strings.Split(req.URL.Path, "/")
		num, err := strconv.Atoi(s[5])
		if err != nil {
			return newErrResponse(req, http.StatusBadRequest, err.Error())
		}
		pr := r.GetPullRequest(num)
		if pr == nil {
			return newNotFoundResponse(req)
		}
		var reqPR struct {
			Title               *string `json:"title,omitempty"`
			Body                *string `json:"body,omitempty"`
			State               *string `json:"state,omitempty"`
			Base                *string `json:"base,omitempty"`
			MaintainerCanModify *bool   `json:"maintainer_can_modify,omitempty"`
		}
		if err := json.NewDecoder(req.Body).Decode(&reqPR); err != nil {
			return newErrResponse(req, http.StatusBadRequest, err.Error())
		}

		if reqPR.Title != nil {
			pr.Title = reqPR.Title
		}
		if reqPR.Body != nil {
			pr.Body = reqPR.Body
		}
		if reqPR.State != nil {
			pr.State = reqPR.State
		}
		if reqPR.Base != nil {
			if pr.Base == nil {
				pr.Base = &github.PullRequestBranch{}
			}
			pr.Base.Ref = reqPR.Base
		}
		if reqPR.MaintainerCanModify != nil {
			pr.MaintainerCanModify = reqPR.MaintainerCanModify
		}

		return newMockJSONResponse(req, http.StatusOK, pr)
	})

	// Create a new comment
	// POST /repos/octocat/example/pulls/1/comments
	tr.RegisterRegexpResponder(http.MethodPost, regexp.MustCompile(`/repos/[^/?]+/[^/?]+/pulls/\d+/comments`), func(req *http.Request) (*http.Response, error) {
		r := m.findRepository(req.URL.Path)
		if r == nil {
			return newNotFoundResponse(req)
		}
		s := strings.Split(req.URL.Path, "/")
		num, err := strconv.Atoi(s[5])
		if err != nil {
			return newErrResponse(req, http.StatusBadRequest, err.Error())
		}
		pr := r.GetPullRequest(num)
		if pr == nil {
			return newNotFoundResponse(req)
		}
		var comment github.PullRequestComment
		if err := json.NewDecoder(req.Body).Decode(&comment); err != nil {
			return newErrResponse(req, http.StatusBadRequest, err.Error())
		}

		pr.Comments = append(pr.Comments, &comment)
		return newMockJSONResponse(req, http.StatusOK, comment)
	})
}

func (m *Mock) registerGitService(tr *httpmock.MockTransport) {
	// Get commit
	// GET /repos/octocat/example/git/commits/{sha}
	tr.RegisterRegexpResponder(http.MethodGet, regexp.MustCompile(`/repos/[^/?]+/[^/?]+/git/commits/[^/?]+$`), func(req *http.Request) (*http.Response, error) {
		r := m.findRepository(req.URL.Path)
		if r == nil {
			return newNotFoundResponse(req)
		}
		s := strings.Split(req.URL.Path, "/")
		sha := s[len(s)-1]
		if sha == "HEAD" { // Special case
			if r.headCommit == nil {
				return newNotFoundResponse(req)
			}
			return newMockJSONResponse(req, http.StatusOK, r.headCommit.ghCommit)
		}
		for _, v := range r.commits {
			if v.ghCommit.GetSHA() == sha {
				return newMockJSONResponse(req, http.StatusOK, v.ghCommit)
			}
		}
		return newNotFoundResponse(req)
	})
	// Get tree
	// Get /repos/octocat/example/git/trees/{sha}
	tr.RegisterRegexpResponder(http.MethodGet, regexp.MustCompile(`/repos/[^/?]+/[^/?]+/git/trees/[^/?]+$`), func(req *http.Request) (*http.Response, error) {
		r := m.findRepository(req.URL.Path)
		if r == nil {
			return newNotFoundResponse(req)
		}
		s := strings.Split(req.URL.Path, "/")
		var prefix *string
		for _, c := range r.commits {
			for _, v := range c.files {
				if v.sha == s[len(s)-1] {
					prefix = varptr.Ptr(v.Name)
					break
				}
			}
		}
		if prefix == nil {
			return newNotFoundResponse(req)
		}

		var entries []*github.TreeEntry
		for _, c := range r.commits {
			for _, v := range c.files[1:] { // Exclude root node
				// Repository root
				if *prefix == "" {
					if strings.Index(v.Name, "/") == -1 {
						ft := "blob"
						if v.mode == fileTypeDir {
							ft = "tree"
						}
						entries = append(entries, &github.TreeEntry{
							SHA:  varptr.Ptr(v.sha),
							Type: varptr.Ptr(ft),
							Path: varptr.Ptr(v.Name),
						})
					}
					continue
				}

				if strings.HasPrefix(v.Name, *prefix) && v.Name != *prefix {
					// Exclude children
					rest := v.Name[strings.Index(v.Name, *prefix)+len(*prefix)+1:]
					if strings.Index(rest, "/") != -1 {
						continue
					}

					ft := "blog"
					if v.mode == fileTypeDir {
						ft = "tree"
					}
					entries = append(entries, &github.TreeEntry{
						SHA:  varptr.Ptr(v.sha),
						Type: varptr.Ptr(ft),
						Path: varptr.Ptr(strings.TrimPrefix(v.Name, *prefix+"/")),
					})
				}
			}
		}
		tree := &github.Tree{
			SHA:     varptr.Ptr(s[len(s)-1]),
			Entries: entries,
		}
		return newMockJSONResponse(req, http.StatusOK, tree)
	})
	// Get blob
	// GET /repos/octocat/example/git/blobs/{sha}
	tr.RegisterRegexpResponder(http.MethodGet, regexp.MustCompile(`/repos/[^/?]+/[^/?]+/git/blobs/[^/?]+$`), func(req *http.Request) (*http.Response, error) {
		r := m.findRepository(req.URL.Path)
		if r == nil {
			return newNotFoundResponse(req)
		}
		s := strings.Split(req.URL.Path, "/")
		sha := s[len(s)-1]
		for _, c := range r.commits {
			for _, v := range c.files {
				if v.sha == sha {
					return httpmock.NewBytesResponse(http.StatusOK, v.Body), nil
				}
			}
		}
		return newNotFoundResponse(req)
	})
	// Get ref
	// GET /repos/octocat/example/git/ref/tags/{sha}
	tr.RegisterRegexpResponder(http.MethodGet, regexp.MustCompile(`/repos/[^/?]+/[^/?]+/git/ref/[^?]+$`), func(req *http.Request) (*http.Response, error) {
		r := m.findRepository(req.URL.Path)
		if r == nil {
			return newNotFoundResponse(req)
		}
		s := strings.Split(req.URL.Path, "/")
		ref := plumbing.ReferenceName("refs/" + strings.Join(s[6:], "/"))
		if ref.IsTag() {
			for _, v := range r.tags {
				if v.Name == ref.Short() {
					reference := &github.Reference{
						Ref: varptr.Ptr(ref.String()),
						Object: &github.GitObject{
							SHA:  v.Commit.ghCommit.SHA,
							Type: varptr.Ptr("commit"),
						},
					}
					return newMockJSONResponse(req, http.StatusOK, reference)
				}
			}
		}
		return newNotFoundResponse(req)
	})
}

func (m *Mock) registerRepositoriesService(tr *httpmock.MockTransport) {
	// Get commit
	// Get /repos/octocat/exampe/commits/{sha}
	tr.RegisterRegexpResponder(http.MethodGet, regexp.MustCompile(`/repos/[^/?]+/[^/?]+/commits/[^/?]+$`), func(req *http.Request) (*http.Response, error) {
		r := m.findRepository(req.URL.Path)
		if r == nil {
			return newNotFoundResponse(req)
		}
		s := strings.Split(req.URL.Path, "/")
		sha := s[len(s)-1]
		if sha == "HEAD" { // Special case
			if r.headCommit == nil {
				return newNotFoundResponse(req)
			}
			return newMockJSONResponse(req, http.StatusOK, &github.RepositoryCommit{
				SHA:    r.headCommit.ghCommit.SHA,
				Commit: r.headCommit.ghCommit,
			})
		}
		for _, c := range r.commits {
			if c.ghCommit.GetSHA() == sha {
				commit := &github.RepositoryCommit{
					SHA:    c.ghCommit.SHA,
					Commit: c.ghCommit,
				}
				return newMockJSONResponse(req, http.StatusOK, commit)
			}
		}
		return newNotFoundResponse(req)
	})
}

func (m *Mock) registerIssuesService(tr *httpmock.MockTransport) {
	// Create issue
	// Post /repos/octocat/example/issues
	tr.RegisterRegexpResponder(http.MethodPost, regexp.MustCompile(`/repos/[^/?]+/[^/?]+/issues$`), func(req *http.Request) (*http.Response, error) {
		r := m.findRepository(req.URL.Path)
		if r == nil {
			return newNotFoundResponse(req)
		}

		var reqIssue github.IssueRequest
		if err := json.NewDecoder(req.Body).Decode(&reqIssue); err != nil {
			return newErrResponse(req, http.StatusBadRequest, err.Error())
		}

		newNumber := r.NextIndex()
		issue := &github.Issue{
			Number: &newNumber,
			Title:  reqIssue.Title,
			Body:   reqIssue.Body,
		}
		r.Issues(issue)
		return newMockJSONResponse(req, http.StatusOK, issue)
	})
	// Create a new comment
	// POST /repos/octocat/example/issues/1/comments
	tr.RegisterRegexpResponder(http.MethodPost, regexp.MustCompile(`/repos/[^/?]+/[^/?]+/issues/\d+/comments`), func(req *http.Request) (*http.Response, error) {
		r := m.findRepository(req.URL.Path)
		if r == nil {
			return newNotFoundResponse(req)
		}
		s := strings.Split(req.URL.Path, "/")
		num, err := strconv.Atoi(s[5])
		if err != nil {
			return newErrResponse(req, http.StatusBadRequest, err.Error())
		}

		issue := r.GetIssue(num)
		if issue != nil {
			var comment github.IssueComment
			if err := json.NewDecoder(req.Body).Decode(&comment); err != nil {
				return newErrResponse(req, http.StatusBadRequest, err.Error())
			}

			issue.Comments = append(issue.Comments, &comment)
			return newMockJSONResponse(req, http.StatusOK, comment)
		}

		pr := r.GetPullRequest(num)
		if pr != nil {
			var comment github.IssueComment
			if err := json.NewDecoder(req.Body).Decode(&comment); err != nil {
				return newErrResponse(req, http.StatusBadRequest, err.Error())
			}

			pr.Comments = append(pr.Comments, &github.PullRequestComment{
				Body: comment.Body,
			})
			return newMockJSONResponse(req, http.StatusOK, comment)
		}

		return newNotFoundResponse(req)
	})
}

func (m *Mock) findRepository(p string) *Repository {
	s := strings.Split(p, "/")
	name := fmt.Sprintf("%s/%s", s[2], s[3])

	if r, ok := m.Repositories[name]; ok {
		return r
	}
	return nil
}

func (r *Repository) NextIndex() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.nextIndex()
}

func (r *Repository) nextIndex() int {
	var lastIndex int
	for _, v := range r.pullRequests {
		if v.GetNumber() > lastIndex {
			lastIndex = v.GetNumber()
		}
	}
	for _, v := range r.issues {
		if v.GetNumber() > lastIndex {
			lastIndex = v.GetNumber()
		}
	}

	return lastIndex + 1
}

func (c *Commit) addDir(dir string) {
	for _, v := range c.files {
		if v.Name == dir {
			return
		}
	}
	c.files = append(c.files, &File{
		Name: dir,
		sha:  NewHash(),
		mode: fileTypeDir,
	})
}

func (c *Commit) addFile(file *File) {
	if file.sha == "" {
		file.sha = NewHash()
	}
	file.mode = fileTypeRegular
	c.files = append(c.files, file)
}

func newNotFoundResponse(req *http.Request) (*http.Response, error) {
	return newErrResponse(req, http.StatusNotFound, "Not found")
}

func newErrResponse(req *http.Request, status int, message string) (*http.Response, error) {
	res, err := httpmock.NewJsonResponse(status, &struct {
		Message string `json:"message"`
	}{Message: message})
	if res != nil {
		res.Request = req
	}
	return res, err
}

func newMockJSONResponse(req *http.Request, status int, body any) (*http.Response, error) {
	res, err := httpmock.NewJsonResponse(status, body)
	if res != nil {
		res.Request = req
	}
	return res, err
}

func NewHash() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	h := sha256.New()
	hash := h.Sum(buf)
	return hex.EncodeToString(hash)
}
