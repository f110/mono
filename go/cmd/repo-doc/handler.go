package main

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"go.f110.dev/mono/go/pkg/git"
)

const (
	pathSeparator = "/-/"
)

var (
	gitHash = regexp.MustCompile(`[[:alnum:]]{40}`)
)

type httpHandler struct {
	client git.GitDataClient
}

var _ http.Handler = &httpHandler{}

func newHttpHandler(client git.GitDataClient) *httpHandler {
	return &httpHandler{client: client}
}

func (h *httpHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	sep := strings.LastIndex(req.URL.Path, pathSeparator)
	repoAndRef := req.URL.Path[1:sep]
	filepath := req.URL.Path[sep+len(pathSeparator):]
	sep = strings.Index(repoAndRef, "/")
	repo, ref := repoAndRef[:sep], repoAndRef[sep+1:]

	var commit string
	if gitHash.MatchString(ref) {
		commit = ref
	} else {
		ref, err := h.client.GetReference(req.Context(), &git.RequestGetReference{Repo: repo, Ref: ref})
		if err != nil {
			http.Error(w, fmt.Sprintf("Reference %s is not found", ref), http.StatusBadRequest)
			return
		}
		commit = ref.Ref.Hash
	}

	tree, err := h.client.GetTree(req.Context(), &git.RequestGetTree{Repo: repo, Sha: commit})
	if err != nil {
		http.Error(w, "Failed to get the tree", http.StatusInternalServerError)
		return
	}
	var blobHash string
	for _, entry := range tree.Tree {
		if entry.Path == filepath {
			blobHash = entry.Sha
			break
		}
	}
	blob, err := h.client.GetBlob(req.Context(), &git.RequestGetBlob{Repo: repo, Sha: blobHash})
	if err != nil {
		http.Error(w, "Failed to get blob", http.StatusInternalServerError)
		return
	}
	w.Write(blob.Content)
}
