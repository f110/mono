package httpserver

import (
	"net/http"

	"go.f110.dev/xerrors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"go.f110.dev/mono/go/git"
)

type GitBackendHandler struct {
	client git.GitDataClient
	repo   string
	ref    string
}

var _ http.Handler = (*GitBackendHandler)(nil)

func GitBackend(addr, repo, ref string) (*GitBackendHandler, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	return &GitBackendHandler{client: git.NewGitDataClient(conn), repo: repo, ref: ref}, nil
}

func (g *GitBackendHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	urlPath := req.URL.Path
	if urlPath[0] == '/' {
		urlPath = urlPath[1:]
	}
	res, err := g.client.GetFile(req.Context(), &git.RequestGetFile{Repo: g.repo, Ref: g.ref, Path: urlPath})
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.NotFound:
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				return
			}
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(res.Content)
}
