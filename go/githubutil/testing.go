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

	"github.com/google/go-github/v73/github"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"

	"go.f110.dev/mono/go/varptr"
)

type Mock struct {
	mu           sync.Mutex
	Repositories map[string]*Repository
}

type Repository struct {
	mu           sync.Mutex
	pullRequests []*PullRequest
	files        []*File
}

func newRepository() *Repository {
	return &Repository{
		files: []*File{{Name: "", sha: newHash(), mode: fileTypeDir}},
	}
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

func (m *Mock) RegisterResponder(tr *httpmock.MockTransport) {
	m.registerPullRequestService(tr)
	m.registerGitService(tr)
	m.registerRepositoriesService(tr)
}

func (m *Mock) registerPullRequestService(tr *httpmock.MockTransport) {
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
		commit := &github.Commit{
			Tree: &github.Tree{
				SHA: varptr.Ptr(r.files[0].sha),
			},
		}
		return newMockJSONResponse(req, http.StatusOK, commit)
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
		for _, v := range r.files {
			if v.sha == s[len(s)-1] {
				prefix = varptr.Ptr(v.Name)
				break
			}
		}
		if prefix == nil {
			return newNotFoundResponse(req)
		}

		var entries []*github.TreeEntry
		for _, v := range r.files[1:] { // Exclude root node
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
		tree := &github.Tree{
			SHA:     varptr.Ptr(r.files[0].sha),
			Entries: entries,
		}
		return newMockJSONResponse(req, http.StatusOK, tree)
	})
	// Get blob
	// Get /repos/octocat/git/blobs/{sha}
	tr.RegisterRegexpResponder(http.MethodGet, regexp.MustCompile(`/repos/[^/?]+/[^/?]+/git/blobs/[^/?]+$`), func(req *http.Request) (*http.Response, error) {
		r := m.findRepository(req.URL.Path)
		if r == nil {
			return newNotFoundResponse(req)
		}
		s := strings.Split(req.URL.Path, "/")
		sha := s[len(s)-1]
		for _, v := range r.files {
			if v.sha == sha {
				return httpmock.NewBytesResponse(http.StatusOK, v.Body), nil
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
		commit := &github.RepositoryCommit{
			SHA: varptr.Ptr(r.files[0].sha),
			Commit: &github.Commit{
				Tree: &github.Tree{
					SHA: varptr.Ptr(r.files[0].sha),
				},
			},
		}
		return newMockJSONResponse(req, http.StatusOK, commit)
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

	return lastIndex + 1
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

func (r *Repository) Files(files ...File) {
	for _, f := range files {
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
			r.addDir(dir)
		}
		r.addFile(f)
	}
}

func (r *Repository) addDir(dir string) {
	for _, v := range r.files {
		if v.Name == dir {
			return
		}
	}
	r.files = append(r.files, &File{
		Name: dir,
		sha:  newHash(),
		mode: fileTypeDir,
	})
}

func (r *Repository) addFile(file File) {
	if file.sha == "" {
		file.sha = newHash()
	}
	file.mode = fileTypeRegular
	r.files = append(r.files, &file)
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

	Comments []*github.PullRequestComment
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

func newHash() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	h := sha256.New()
	hash := h.Sum(buf)
	return hex.EncodeToString(hash)
}
