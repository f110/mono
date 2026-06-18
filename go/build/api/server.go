package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-github/v85/github"
	"go.f110.dev/protoc-ddl/probe"
	"google.golang.org/grpc"

	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/build/webhook"
	"go.f110.dev/mono/go/enumerable"
	"go.f110.dev/mono/go/git"
	"go.f110.dev/mono/go/logger/slogger"
	"go.f110.dev/mono/go/storage"
)

const (
	BuildConfigurationFile = "build.star"
	BazelVersionFile       = ".bazelversion"
)

// Builder is the interface used by the gRPC apiService (ForceStop, etc.).
// The webhook ingestion path no longer depends on this — that flow lives in
// the eventbus package.
type Builder interface {
	Build(ctx context.Context, repo *database.SourceRepository, job *config.JobV2, revision, bazelVersion, command string, targets, platforms []string, via string, isMainBranch bool) ([]*database.Task, error)
	ForceStop(ctx context.Context, taskId int32) error
}

type Api struct {
	*http.Server

	dao               dao.Options
	stClient          *storage.S3
	bazelMirrorPrefix string

	webhookHandler *webhook.Handler
}

// NewApi wires the HTTP/gRPC mux. The webhook endpoint delegates to
// webhook.Handler, which records the delivery and returns 200 without doing
// any business logic — reconciliation runs asynchronously inside the leader's
// Scheduler.
func NewApi(addr string, builder Builder, dao dao.Options, ghClient *github.Client, gitDataClient git.GitDataClient, stClient *storage.S3, bazelMirrorPrefix string, notifier *webhook.Notifier, addRepo chan<- *git.RepositoryConfig, serverConfig *ServerConfig) (*Api, error) {
	api := &Api{
		dao:               dao,
		stClient:          stClient,
		bazelMirrorPrefix: bazelMirrorPrefix,
		webhookHandler:    webhook.NewHandler(dao, notifier),
	}
	mux := http.NewServeMux()
	mux.Handle("/favicon.ico", http.NotFoundHandler())
	mux.HandleFunc("/liveness", api.handleLiveness)
	mux.HandleFunc("/readiness", api.handleReadiness)
	mux.Handle("/webhook", api.webhookHandler)

	bs := newAPIService(builder, dao, ghClient, gitDataClient, stClient, bazelMirrorPrefix, addRepo, serverConfig)
	grpcServer := grpc.NewServer()
	RegisterAPIServer(grpcServer, bs)
	s := &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.ProtoMajor == 2 && strings.HasPrefix(
				req.Header.Get("Content-Type"), "application/grpc") {
				grpcServer.ServeHTTP(w, req)
			} else {
				mux.ServeHTTP(w, req)
			}
		}),
		Protocols: new(http.Protocols),
	}
	s.Protocols.SetHTTP1(true)
	s.Protocols.SetHTTP2(true)
	s.Protocols.SetUnencryptedHTTP2(true)
	api.Server = s

	return api, nil
}

type ReadinessResponse struct {
	Versions []string `json:"versions"`
}

func (a *Api) handleReadiness(w http.ResponseWriter, req *http.Request) {
	p := probe.NewProbe(a.dao.RawConnection)
	if !p.Ready(req.Context(), database.SchemaHash) {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	objs, err := a.stClient.List(req.Context(), a.bazelMirrorPrefix)
	if err != nil {
		slogger.Log.Error("Failed to get the list of the file from the object storage", slogger.E(err), slog.String("prefix", a.bazelMirrorPrefix))
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	var versions semver.Collection
	for _, v := range objs {
		name := filepath.Base(v.Name)
		if !strings.HasPrefix(name, "bazel-") {
			continue
		}
		ver := name[6:]
		ver = ver[:strings.Index(ver, "-")]
		if v, err := semver.NewVersion(ver); err != nil {
			continue
		} else {
			versions = append(versions, v)
		}
	}
	versions = enumerable.Uniq(versions, func(t *semver.Version) string { return t.String() })
	sort.Sort(versions)

	res := &ReadinessResponse{Versions: enumerable.Map(versions, func(t *semver.Version) string { return t.String() })}
	if err := json.NewEncoder(w).Encode(res); err != nil {
		slogger.Log.Error("Failed to encode to json", slogger.E(err))
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
}

func (*Api) handleLiveness(_ http.ResponseWriter, _ *http.Request) {}
