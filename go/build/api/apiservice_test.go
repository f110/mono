package api

import (
	"context"
	"regexp"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/build/database/dao/daotest"
	"go.f110.dev/mono/go/git"
	"go.f110.dev/mono/go/storage"
	"go.f110.dev/mono/go/testing/assertion"
)

func TestExtractRepositoryFromPayload(t *testing.T) {
	cases := []struct {
		name     string
		payload  string
		wantName string
		wantURL  string
	}{
		{
			name:     "pull_request style payload",
			payload:  `{"action":"opened","repository":{"full_name":"f110/ops","html_url":"https://github.com/f110/ops"}}`,
			wantName: "f110/ops",
			wantURL:  "https://github.com/f110/ops",
		},
		{
			name:     "empty payload",
			payload:  ``,
			wantName: "",
			wantURL:  "",
		},
		{
			name:     "no repository field",
			payload:  `{"action":"created"}`,
			wantName: "",
			wantURL:  "",
		},
		{
			name:     "malformed JSON falls back to empty",
			payload:  `{"action": broken`,
			wantName: "",
			wantURL:  "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotName, gotURL := extractRepositoryFromPayload([]byte(tc.payload))
			assertion.Equal(t, tc.wantName, gotName)
			assertion.Equal(t, tc.wantURL, gotURL)
		})
	}
}

type fakeBuilder struct {
	Builder
	job *config.JobV2
}

func (b *fakeBuilder) Build(_ context.Context, _ *database.SourceRepository, job *config.JobV2, _, _, _ string, _, _ []string, _ string, _ bool) ([]*database.Task, error) {
	b.job = job
	return []*database.Task{{Id: 1}}, nil
}

type fakeGitDataClient struct {
	git.GitDataClient
}

func (*fakeGitDataClient) GetReference(_ context.Context, _ *git.RequestGetReference, _ ...grpc.CallOption) (*git.ResponseGetReference, error) {
	return &git.ResponseGetReference{Ref: &git.Reference{Hash: "abcdef"}}, nil
}

func newJobDAO(t *testing.T, job *config.JobV2) *daotest.Job {
	conf, err := config.MarshalJob(job)
	assertion.MustNoError(t, err)
	m := daotest.NewJob()
	m.RegisterSelect(1, job.Name, &database.Job{RepositoryId: 1, Name: job.Name, Configuration: conf})
	m.RegisterListByRepositoryId(1, []*database.Job{{RepositoryId: 1, Name: job.Name, Configuration: conf}}, nil)
	return m
}

func TestInvokeJob(t *testing.T) {
	job := &config.JobV2{
		Name:      "release",
		Command:   "run",
		Targets:   []string{"//:release"},
		Platforms: []string{"linux_amd64"},
		Params:    map[string]*config.Param{"version": {Required: true}},
	}

	cases := []struct {
		Name   string
		Params map[string]string
		Args   []string
		Err    bool
	}{
		{Name: "With params", Params: map[string]string{"version": "v0.18.0"}, Args: []string{"--version=v0.18.0"}},
		{Name: "Missing required param", Err: true},
		{Name: "Unknown param", Params: map[string]string{"version": "v0.18.0", "unknown": "foo"}, Err: true},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			repoDAO := daotest.NewSourceRepository()
			repoDAO.RegisterSelect(1, &database.SourceRepository{Id: 1, Url: "https://github.com/f110/heimdallr", DefaultBranch: "master"})
			builder := &fakeBuilder{}
			s := &apiService{
				builder:       builder,
				dao:           dao.Options{Repository: repoDAO, Job: newJobDAO(t, job)},
				gitDataClient: &fakeGitDataClient{},
			}

			_, err := s.InvokeJob(context.Background(), RequestInvokeJob_builder{RepositoryId: new(int32(1)), JobName: new("release"), Params: tc.Params}.Build())
			if tc.Err {
				assertion.MustError(t, err)
				return
			}
			assertion.MustNoError(t, err)
			assertion.Equal(t, tc.Args, builder.job.Args)
		})
	}
}

func TestListJobs(t *testing.T) {
	job := &config.JobV2{
		Name:    "release",
		Command: "run",
		Params: map[string]*config.Param{
			"version": {Description: "Release version", Required: true},
			"branch":  {},
		},
	}
	s := &apiService{dao: dao.Options{Job: newJobDAO(t, job)}}

	res, err := s.ListJobs(context.Background(), RequestListJobs_builder{RepositoryId: new(int32(1))}.Build())
	assertion.MustNoError(t, err)
	assertion.Len(t, res.GetJobs(), 1)
	params := res.GetJobs()[0].GetParams()
	assertion.Len(t, params, 2)
	assertion.Equal(t, "branch", params[0].GetName())
	assertion.False(t, params[0].GetRequired())
	assertion.Equal(t, "version", params[1].GetName())
	assertion.Equal(t, "Release version", params[1].GetDescription())
	assertion.True(t, params[1].GetRequired())
}

func TestListTasks(t *testing.T) {
	t.Run("by repository and revisions", func(t *testing.T) {
		t.Parallel()
		taskDAO := daotest.NewTask()
		taskDAO.RegisterListByRevision(1, "rev1", []*database.Task{{Id: 10, RepositoryId: 1, Revision: "rev1"}}, nil)
		taskDAO.RegisterListByRevision(1, "rev2", []*database.Task{{Id: 11, RepositoryId: 1, Revision: "rev2"}}, nil)
		taskDAO.RegisterListByRevision(2, "rev1", nil, nil)
		taskDAO.RegisterListByRevision(2, "rev2", []*database.Task{{Id: 12, RepositoryId: 2, Revision: "rev2"}}, nil)
		testReportDAO := daotest.NewTestReport()
		buildMetricsDAO := daotest.NewBuildMetrics()
		for _, id := range []int32{10, 11, 12} {
			testReportDAO.RegisterListByTaskId(id, nil, nil)
			buildMetricsDAO.RegisterListByTaskId(id, nil, nil)
		}
		s := &apiService{dao: dao.Options{Task: taskDAO, TestReport: testReportDAO, BuildMetrics: buildMetricsDAO}}

		req := &RequestListTasks{}
		req.SetRepositoryIds([]int32{1, 2})
		req.SetRevisions([]string{"rev1", "rev2"})
		res, err := s.ListTasks(context.Background(), req)
		assertion.MustNoError(t, err)
		var ids []int32
		for _, v := range res.GetTasks() {
			ids = append(ids, v.GetId())
		}
		assertion.Equal(t, []int32{10, 11, 12}, ids)
	})

	t.Run("revisions without repository is rejected", func(t *testing.T) {
		t.Parallel()
		s := &apiService{}
		req := &RequestListTasks{}
		req.SetRevisions([]string{"rev1"})
		_, err := s.ListTasks(context.Background(), req)
		assertion.Error(t, err)
	})
}

func TestGetLogs(t *testing.T) {
	taskDAO := daotest.NewTask()
	taskDAO.RegisterSelect(1, &database.Task{Id: 1, LogFile: "task/1.log"})
	taskDAO.RegisterSelect(2, &database.Task{Id: 2})
	logStorage := storage.NewMock()
	logStorage.AddTree("task/1.log", []byte("INFO: start\nERROR: failed\nINFO: done\n"))
	s := &apiService{dao: dao.Options{Task: taskDAO}, logStorage: logStorage}

	cases := []struct {
		Name   string
		TaskID int32
		Filter string
		Want   string
		Code   codes.Code
	}{
		{Name: "without the filter", TaskID: 1, Want: "INFO: start\nERROR: failed\nINFO: done\n"},
		{Name: "with the filter", TaskID: 1, Filter: "^ERROR", Want: "ERROR: failed\n"},
		{Name: "invalid filter", TaskID: 1, Filter: "(", Code: codes.InvalidArgument},
		{Name: "the task has no log", TaskID: 2, Code: codes.NotFound},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			res, err := s.GetLogs(context.Background(), RequestGetLogs_builder{TaskId: new(tc.TaskID), Filter: new(tc.Filter)}.Build())
			if tc.Code != codes.OK {
				assertion.Equal(t, tc.Code, status.Code(err))
				return
			}
			assertion.MustNoError(t, err)
			assertion.Equal(t, tc.Want, res.GetBody())
		})
	}
}

func TestReadLog(t *testing.T) {
	st := storage.NewMock()
	st.AddTree("task/1.log", []byte("INFO: start\nERROR: foo failed\nINFO: done\nERROR: bar failed"))

	cases := []struct {
		Name   string
		Filter *regexp.Regexp
		Want   string
	}{
		{Name: "without the filter", Want: "INFO: start\nERROR: foo failed\nINFO: done\nERROR: bar failed"},
		{Name: "with the filter", Filter: regexp.MustCompile(`^ERROR`), Want: "ERROR: foo failed\nERROR: bar failed\n"},
		{Name: "no line matches", Filter: regexp.MustCompile(`WARN`), Want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			got, err := ReadLog(context.Background(), st, "task/1.log", tc.Filter)
			assertion.MustNoError(t, err)
			assertion.Equal(t, tc.Want, got)
		})
	}

	t.Run("the log is not found", func(t *testing.T) {
		_, err := ReadLog(context.Background(), st, "task/2.log", nil)
		assertion.MustError(t, err)
	})
}
