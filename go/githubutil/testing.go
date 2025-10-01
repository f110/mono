package githubutil

import (
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
)

type Mock struct {
	mu           sync.Mutex
	Repositories map[string]*Repository
}

type Repository struct {
	mu           sync.Mutex
	pullRequests []*PullRequest
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

	r := &Repository{}
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
			r.pullRequests[len(r.pullRequests)-1].Number = github.Int(r.nextIndex())
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
	res.Request = req
	return res, err
}

func newMockJSONResponse(req *http.Request, status int, body any) (*http.Response, error) {
	res, err := httpmock.NewJsonResponse(status, body)
	res.Request = req
	return res, err
}
