// Package mcp exposes the read operations of Build to the coding agents by MCP. Every tool calls the method of the API
// service, so the tools behave the same as the gRPC API.
package mcp

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.f110.dev/xerrors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"go.f110.dev/mono/go/build/api"
)

type listTasksInput struct {
	IDs           []int32  `json:"ids,omitempty" jsonschema:"IDs of the tasks. At most 100"`
	RepositoryIDs []int32  `json:"repository_ids,omitempty" jsonschema:"IDs of the repositories. See list_repositories"`
	Revisions     []string `json:"revisions,omitempty" jsonschema:"Revisions of the commits. repository_ids is required with this"`
	PageSize      *int32   `json:"page_size,omitempty" jsonschema:"Number of the tasks when neither ids nor repository_ids is specified. At most 100"`
	PageToken     *string  `json:"page_token,omitempty" jsonschema:"next_page_token of the previous response"`
}

type repositoryInput struct {
	RepositoryID int32 `json:"repository_id" jsonschema:"ID of the repository. See list_repositories"`
}

type listExternalReleaseTriggersInput struct {
	RepositoryID int32 `json:"repository_id,omitempty" jsonschema:"ID of the repository. Omit to list the triggers of all repositories"`
}

type listGithubEventsInput struct {
	EventID *int32 `json:"event_id,omitempty" jsonschema:"ID of the event. Omit to list all events"`
}

type listArtifactsInput struct {
	TaskID       int32  `json:"task_id,omitempty" jsonschema:"ID of the task"`
	RepositoryID int32  `json:"repository_id,omitempty" jsonschema:"ID of the repository. Specify with revision instead of task_id"`
	Revision     string `json:"revision,omitempty" jsonschema:"Revision of the commit. Specify with repository_id instead of task_id"`
}

type getLogsInput struct {
	TaskID int32  `json:"task_id" jsonschema:"ID of the task"`
	Filter string `json:"filter,omitempty" jsonschema:"Regular expression (RE2) to filter the lines of the log. Omit to get the whole log"`
}

type emptyInput struct{}

// New returns the HTTP handler which serves MCP. The requests are authenticated by authenticate (e.g.
// authproxy.Authenticate).
func New(s api.APIServer, authenticate func(http.Handler) http.Handler) http.Handler {
	server := newServer(s)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true})
	return authenticate(handler)
}

// newServer returns the MCP server which has the tools of the read operations of Build.
func newServer(s api.APIServer) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "build", Version: "v0.1.0"}, nil)
	addTool(server, "list_tasks", "List the tasks. Without ids and repository_ids, the latest tasks of all repositories are listed", s.ListTasks,
		func(in listTasksInput) *api.RequestListTasks {
			return api.RequestListTasks_builder{Ids: in.IDs, RepositoryIds: in.RepositoryIDs, Revisions: in.Revisions, PageSize: in.PageSize, PageToken: in.PageToken}.Build()
		})
	addTool(server, "get_logs", "Get the log of the task", s.GetLogs, func(in getLogsInput) *api.RequestGetLogs {
		return api.RequestGetLogs_builder{TaskId: &in.TaskID, Filter: &in.Filter}.Build()
	})
	addTool(server, "list_repositories", "List the repositories", s.ListRepositories, func(emptyInput) *api.RequestListRepositories {
		return &api.RequestListRepositories{}
	})
	addTool(server, "list_jobs", "List the jobs of the repository", s.ListJobs, func(in repositoryInput) *api.RequestListJobs {
		return api.RequestListJobs_builder{RepositoryId: &in.RepositoryID}.Build()
	})
	addTool(server, "list_artifacts", "List the artifacts of the task or the commit", s.ListArtifacts, func(in listArtifactsInput) *api.RequestListArtifacts {
		return api.RequestListArtifacts_builder{TaskId: &in.TaskID, RepositoryId: &in.RepositoryID, Revision: &in.Revision}.Build()
	})
	addTool(server, "list_external_release_triggers", "List the triggers which run the jobs on the releases of the external repositories",
		s.ListExternalReleaseTriggers, func(in listExternalReleaseTriggersInput) *api.RequestListExternalReleaseTriggers {
			return api.RequestListExternalReleaseTriggers_builder{RepositoryId: &in.RepositoryID}.Build()
		})
	addTool(server, "list_github_events", "List the events of GitHub which Build received", s.ListGithubEvents, func(in listGithubEventsInput) *api.RequestListGithubEvents {
		return api.RequestListGithubEvents_builder{EventId: in.EventID}.Build()
	})
	addTool(server, "get_server_info", "Get the configuration and the supported Bazel versions of the server", s.GetServerInfo,
		func(emptyInput) *api.RequestGetServerInfo { return &api.RequestGetServerInfo{} })
	return server
}

var marshalOptions = protojson.MarshalOptions{UseProtoNames: true}

// addTool adds the tool which converts the input to the request by toRequest and calls rpc. The response is returned as
// JSON. The error is returned as the error of the tool so that the agent can read the reason.
func addTool[In, Req any, Res proto.Message](s *mcp.Server, name, description string, rpc func(context.Context, *Req) (Res, error), toRequest func(In) *Req) {
	mcp.AddTool(s, &mcp.Tool{Name: name, Description: description}, func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
		res, err := rpc(ctx, toRequest(in))
		if err != nil {
			return nil, nil, err
		}
		b, err := marshalOptions.Marshal(res)
		if err != nil {
			return nil, nil, xerrors.WithStack(err)
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}, nil, nil
	})
}
