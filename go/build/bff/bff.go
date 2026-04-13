package bff

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"connectrpc.com/connect"
	"github.com/rs/cors"
	"go.f110.dev/xerrors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/protobuf/types/known/durationpb"

	"go.f110.dev/mono/go/build/api"
	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/model"
	"go.f110.dev/mono/go/enumerable"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/storage"
)

type Builder interface {
	Build(ctx context.Context, repo *database.SourceRepository, job *config.JobV2, revision, bazelVersion, command string, targets, platforms []string, via string, isMainBranch bool) ([]*database.Task, error)
}

type BFF struct {
	*http.Server

	apiClient api.APIClient
	s3        *storage.S3
}

func NewBFF(addr string, grpcConn *grpc.ClientConn, apiClient api.APIClient, bucket string, s3Opt storage.S3Options) *BFF {
	b := &BFF{
		apiClient: apiClient,
		s3:        storage.NewS3(bucket, s3Opt),
	}
	mux := http.NewServeMux()
	mux.Handle(NewBFFHandler(b, connect.WithInterceptors(newAccessLogInterceptor())))
	mux.HandleFunc("GET /liveness", func(_ http.ResponseWriter, _ *http.Request) { return })
	mux.HandleFunc("GET /readiness", func(w http.ResponseWriter, _ *http.Request) {
		switch grpcConn.GetState() {
		case connectivity.Idle, connectivity.Connecting, connectivity.Ready:
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	})
	c := cors.New(cors.Options{
		AllowOriginFunc:  func(_ string) bool { return true },
		AllowedHeaders:   []string{"Connect-Protocol-Version", "Content-Type"},
		AllowCredentials: true,
	})
	b.Server = &http.Server{
		Addr:      addr,
		Handler:   c.Handler(mux),
		Protocols: new(http.Protocols),
	}
	b.Server.Protocols.SetHTTP1(true)
	b.Server.Protocols.SetHTTP2(true)
	b.Server.Protocols.SetUnencryptedHTTP2(true)
	return b
}

func (b *BFF) Start() error {
	return b.Server.ListenAndServe()
}

func (b *BFF) ListRepositories(ctx context.Context, _ *connect.Request[RequestListRepositories]) (*connect.Response[ResponseListRepositories], error) {
	allRepo, err := b.apiClient.ListRepositories(ctx, api.RequestListRepositories_builder{}.Build())
	if err != nil {
		logger.Log.Warn("Failed to list repositories", logger.Error(err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	repositories := enumerable.Map(allRepo.GetRepositories(), func(v *model.Repository) *model.Repository {
		return model.Repository_builder{
			Id:      new(v.GetId()),
			Name:    new(v.GetName()),
			Url:     new(v.GetUrl()),
			Private: new(v.GetPrivate()),
		}.Build()
	})
	return connect.NewResponse(ResponseListRepositories_builder{Repositories: repositories}.Build()), nil
}

func (b *BFF) ListTasks(ctx context.Context, req *connect.Request[RequestListTasks]) (*connect.Response[ResponseListTasks], error) {
	if req.Msg.HasTaskId() && req.Msg.HasRepositoryId() {
		return nil, connect.NewError(connect.CodeInvalidArgument, xerrors.New("can not specify task_id and repository_id at the same time"))
	}

	var ids []int32
	if req.Msg.HasTaskId() {
		ids = []int32{req.Msg.GetTaskId()}
	}
	var repositoryIds []int32
	if req.Msg.HasRepositoryId() {
		repositoryIds = []int32{req.Msg.GetRepositoryId()}
	}
	var pageToken *string
	if req.Msg.HasPageToken() {
		pageToken = new(req.Msg.GetPageToken())
	}
	var pageSize *int32
	if req.Msg.HasPageSize() {
		pageSize = new(req.Msg.GetPageSize())
	}
	tasks, err := b.apiClient.ListTasks(ctx, api.RequestListTasks_builder{Ids: ids, RepositoryIds: repositoryIds, PageToken: pageToken, PageSize: pageSize}.Build())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if len(tasks.GetTasks()) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	var nextPageToken *string
	if tasks.HasNextPageToken() {
		nextPageToken = new(tasks.GetNextPageToken())
	}

	apiTasks := tasks.GetTasks()
	repositoryIDs := enumerable.Uniq(enumerable.Map(apiTasks, func(v *model.Task) int32 { return v.GetRepositoryId() }), func(t int32) int32 { return t })
	repositories, err := b.apiClient.ListRepositories(ctx, api.RequestListRepositories_builder{Ids: repositoryIDs}.Build())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	repositoriesMap := make(map[int32]*model.Repository)
	for _, v := range repositories.GetRepositories() {
		repositoriesMap[v.GetId()] = v
	}
	var bffTasks []*BFFTask
	if req.Msg.HasTaskId() {
		bffTasks = enumerable.Map(enumerable.FindAll(tasks.GetTasks(), func(v *model.Task) bool { return v.GetId() == req.Msg.GetTaskId() }), b.apiTaskToBFFTask(repositoriesMap))
	} else if req.Msg.HasRepositoryId() {
		bffTasks = enumerable.Map(enumerable.FindAll(tasks.GetTasks(), func(v *model.Task) bool { return v.GetRepositoryId() == req.Msg.GetRepositoryId() }), b.apiTaskToBFFTask(repositoriesMap))
	} else {
		bffTasks = enumerable.Map(tasks.GetTasks(), b.apiTaskToBFFTask(repositoriesMap))
	}

	return connect.NewResponse(ResponseListTasks_builder{Tasks: bffTasks, NextPageToken: nextPageToken}.Build()), nil
}

func (*BFF) apiTaskToBFFTask(repositories map[int32]*model.Repository) func(*model.Task) *BFFTask {
	return func(v *model.Task) *BFFTask {
		u, err := url.Parse(repositories[v.GetRepositoryId()].GetUrl())
		var revisionURL string
		if err == nil {
			switch u.Hostname() {
			case "github.com":
				revisionURL = fmt.Sprintf("%s/commit/%s", repositories[v.GetRepositoryId()].GetUrl(), v.GetRevision())
			}
		}
		return BFFTask_builder{
			Id:                     new(v.GetId()),
			Repository:             repositories[v.GetRepositoryId()],
			JobName:                new(v.GetJobName()),
			ParsedJobConfiguration: new(v.GetParsedJobConfiguration()),
			Revision:               new(v.GetRevision()),
			BazelVersion:           new(v.GetBazelVersion()),
			Command:                new(v.GetCommand()),
			IsTrunk:                new(v.GetIsTrunk()),
			Success:                new(v.GetSuccess()),
			LogFile:                new(v.GetLogFile()),
			Targets:                v.GetTargets(),
			Platform:               new(v.GetPlatform()),
			Via:                    new(v.GetVia()),
			ConfigName:             new(v.GetConfigName()),
			Node:                   new(v.GetNode()),
			Manifest:               new(v.GetManifest()),
			Container:              new(v.GetContainer()),
			ExecutedTestsCount:     new(v.GetExecutedTestsCount()),
			SucceededTestsCount:    new(v.GetSucceededTestsCount()),
			StartAt:                v.GetStartAt(),
			FinishedAt:             v.GetFinishedAt(),
			CreatedAt:              v.GetCreatedAt(),
			UpdatedAt:              v.GetUpdatedAt(),
			RepositoryUrl:          new(repositories[v.GetRepositoryId()].GetUrl()),
			RevisionUrl:            new(revisionURL),
			CpuLimit:               new(v.GetCpuLimit()),
			MemoryLimit:            new(v.GetMemoryLimit()),
			TestReports:            v.GetTestReports(),
			Duration:               durationpb.New(v.GetFinishedAt().AsTime().Sub(v.GetStartAt().AsTime())),
		}.Build()
	}
}

func (b *BFF) GetLogs(ctx context.Context, req *connect.Request[RequestGetLogs]) (*connect.Response[ResponseGetLogs], error) {
	tasks, err := b.apiClient.ListTasks(ctx, api.RequestListTasks_builder{Ids: []int32{req.Msg.GetTaskId()}}.Build())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if len(tasks.GetTasks()) != 1 {
		return nil, connect.NewError(connect.CodeInvalidArgument, xerrors.New("invalid task id"))
	}
	logObj, err := b.s3.Get(ctx, tasks.GetTasks()[0].GetLogFile())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	buf, err := io.ReadAll(logObj.Body)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer logObj.Body.Close()
	return connect.NewResponse(ResponseGetLogs_builder{Body: new(string(buf))}.Build()), nil
}

func (b *BFF) GetServerInfo(ctx context.Context, _ *connect.Request[RequestGetServerInfo]) (*connect.Response[ResponseGetServerInfo], error) {
	res, err := b.apiClient.GetServerInfo(ctx, api.RequestGetServerInfo_builder{}.Build())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(ResponseGetServerInfo_builder{SupportedBazelVersions: res.GetSupportedBazelVersions()}.Build()), nil
}

func (b *BFF) ListJobs(ctx context.Context, req *connect.Request[RequestListJobs]) (*connect.Response[ResponseListJobs], error) {
	jobs, err := b.apiClient.ListJobs(ctx, api.RequestListJobs_builder{RepositoryId: new(req.Msg.GetRepositoryId())}.Build())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(ResponseListJobs_builder{
		Jobs: jobs.GetJobs(),
	}.Build()), nil
}

func (b *BFF) InvokeJob(ctx context.Context, req *connect.Request[RequestInvokeJob]) (*connect.Response[ResponseInvokeJob], error) {
	if !req.Msg.HasRepositoryId() || !req.Msg.HasJobName() {
		return nil, connect.NewError(connect.CodeInvalidArgument, xerrors.New("repository_id and job_name must be specified"))
	}
	_, err := b.apiClient.InvokeJob(ctx, api.RequestInvokeJob_builder{RepositoryId: new(req.Msg.GetRepositoryId()), JobName: new(req.Msg.GetJobName())}.Build())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(ResponseInvokeJob_builder{}.Build()), nil
}

func (b *BFF) SaveRepository(ctx context.Context, req *connect.Request[RequestSaveRepository]) (*connect.Response[ResponseSaveRepository], error) {
	if !req.Msg.HasRepository() {
		return nil, connect.NewError(connect.CodeInvalidArgument, xerrors.New("missing repository"))
	}
	if req.Msg.GetRepository().HasId() {
		return nil, connect.NewError(connect.CodeInvalidArgument, xerrors.New("mutating the repository is not supported yet"))
	}

	createdRepository, err := b.apiClient.SaveRepository(ctx, api.RequestSaveRepository_builder{Repository: model.Repository_builder{
		Name:     new(req.Msg.GetRepository().GetName()),
		Url:      new(req.Msg.GetRepository().GetUrl()),
		CloneUrl: new(req.Msg.GetRepository().GetCloneUrl()),
		Private:  new(req.Msg.GetRepository().GetPrivate()),
	}.Build()}.Build())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(ResponseSaveRepository_builder{Repository: model.Repository_builder{
		Id:       new(createdRepository.GetRepository().GetId()),
		Name:     new(createdRepository.GetRepository().GetName()),
		Url:      new(createdRepository.GetRepository().GetUrl()),
		CloneUrl: new(createdRepository.GetRepository().GetCloneUrl()),
		Private:  new(createdRepository.GetRepository().GetPrivate()),
	}.Build()}.Build()), nil
}

func (b *BFF) RemoveRepository(ctx context.Context, req *connect.Request[RequestRemoveRepository]) (*connect.Response[ResponseRemoveRepository], error) {
	_, err := b.apiClient.DeleteRepository(ctx, api.RequestDeleteRepository_builder{RepositoryId: new(req.Msg.GetRepositoryId())}.Build())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(ResponseRemoveRepository_builder{}.Build()), nil
}

func (b *BFF) SyncRepository(ctx context.Context, req *connect.Request[RequestSyncRepository]) (*connect.Response[ResponseSyncRepository], error) {
	if _, err := b.apiClient.SyncRepository(ctx, api.RequestSyncRepository_builder{RepositoryId: new(req.Msg.GetRepositoryId())}.Build()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(ResponseSyncRepository_builder{}.Build()), nil
}

func (b *BFF) RestartTask(ctx context.Context, req *connect.Request[RequestRestartTask]) (*connect.Response[ResponseRestartTask], error) {
	_, err := b.apiClient.InvokeJob(ctx, api.RequestInvokeJob_builder{TaskId: new(req.Msg.GetTaskId())}.Build())
	if err != nil {
		logger.Log.Error("Failed to restart task", logger.Error(err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(ResponseRestartTask_builder{}.Build()), nil
}

func (b *BFF) ForceStopTask(ctx context.Context, req *connect.Request[RequestForceStopTask]) (*connect.Response[ResponseForceStopTask], error) {
	if !req.Msg.HasTaskId() {
		return nil, connect.NewError(connect.CodeInvalidArgument, xerrors.New("task id must be specified"))
	}
	_, err := b.apiClient.ForceStopTask(ctx, api.RequestForceStopTask_builder{TaskId: new(req.Msg.GetTaskId())}.Build())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(ResponseForceStopTask_builder{}.Build()), nil
}
