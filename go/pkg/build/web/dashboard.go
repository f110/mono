package web

import (
	"html/template"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/google/go-github/v32/github"
	"go.f110.dev/protoc-ddl/probe"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/pkg/build/database"
	"go.f110.dev/mono/go/pkg/build/database/dao"
	"go.f110.dev/mono/go/pkg/build/discovery"
	"go.f110.dev/mono/go/pkg/logger"
	"go.f110.dev/mono/go/pkg/storage"
)

const (
	NumberOfTaskPerJob = 10
)

type Dashboard struct {
	*http.Server

	dao       dao.Options
	apiHost   string
	minio     *storage.MinIO
	discovery *discovery.Viewer
}

func NewDashboard(addr string, daoOpt dao.Options, apiHost string, bucket string, minioOpt storage.MinIOOptions) *Dashboard {
	d := &Dashboard{
		dao:       daoOpt,
		apiHost:   apiHost,
		minio:     storage.NewMinIOStorage(bucket, minioOpt),
		discovery: discovery.NewViewer(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/favicon.ico", http.NotFound)
	mux.HandleFunc("/liveness", d.handleLiveness)
	mux.HandleFunc("/readiness", d.handleReadiness)
	mux.HandleFunc("/logs/", d.handleLogs)
	mux.HandleFunc("/new_repo", d.handleNewRepository)
	mux.HandleFunc("/delete_repo", d.handleDeleteRepository)
	mux.HandleFunc("/add_trusted_user", d.handleAddTrustedUser)
	mux.HandleFunc("/", d.handleIndex)
	s := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	d.Server = s

	return d
}

func (d *Dashboard) Start() error {
	return d.Server.ListenAndServe()
}

type Job struct {
	*database.Job
	Tasks   []*Task
	Success bool
}

type Task struct {
	*database.Task
	RevisionUrl string
}

type RepositoryAndJobs struct {
	Repo          *database.SourceRepository
	IsDiscovering bool
	LogFile       string
	Jobs          []*Job
}

func (d *Dashboard) handleIndex(w http.ResponseWriter, req *http.Request) {
	repoList, err := d.dao.Repository.ListAll(req.Context())
	if err != nil {
		logger.Log.Warn("Failed get repository", zap.Error(err))
		return
	}

	jobs, err := d.dao.Job.ListAll(req.Context())
	if err != nil {
		logger.Log.Warn("Failed get job", zap.Error(err))
		return
	}
	repoAndJobs := make(map[int32]*RepositoryAndJobs)
	for _, v := range repoList {
		repoAndJobs[v.Id] = &RepositoryAndJobs{
			Repo:          v,
			IsDiscovering: d.discovery.IsDiscovering(v.Id),
			LogFile:       v.Name + "-discovery",
			Jobs:          make([]*Job, 0),
		}
	}
	for _, v := range jobs {
		if _, ok := repoAndJobs[v.RepositoryId]; !ok {
			continue
		}

		tasks, err := d.dao.Task.ListByJobId(req.Context(), v.Id, dao.Limit(NumberOfTaskPerJob), dao.Desc)
		if err != nil {
			logger.Log.Warn("Failed get task", zap.Error(err), zap.Int32("job", v.Id))
			continue
		}
		sort.Slice(tasks, func(i, j int) bool {
			return tasks[i].Id > tasks[j].Id
		})
		success := false
		if len(tasks) > 0 {
			success = tasks[0].Success
		}

		var isGitHub bool
		if strings.Contains(v.Repository.Url, "https://github.com") {
			isGitHub = true
		}
		t := make([]*Task, len(tasks))
		for i := range tasks {
			revUrl := ""
			if isGitHub {
				revUrl = v.Repository.Url + "/commit/" + tasks[i].Revision
			}
			t[i] = &Task{
				Task:        tasks[i],
				RevisionUrl: revUrl,
			}
		}
		repoAndJobs[v.RepositoryId].Jobs = append(repoAndJobs[v.RepositoryId].Jobs, &Job{Job: v, Tasks: t, Success: success})
	}

	jobList := make([]*RepositoryAndJobs, 0)
	for _, v := range repoList {
		if r, ok := repoAndJobs[v.Id]; ok {
			jobList = append(jobList, r)
		}
	}

	trustedUsers, err := d.dao.TrustedUser.ListAll(req.Context())
	if err != nil {
		logger.Log.Warn("Failed get trusted user", zap.Error(err))
		return
	}

	err = Template.Execute(w, struct {
		Repositories []*database.SourceRepository
		RepoAndJobs  []*RepositoryAndJobs
		TrustedUsers []*database.TrustedUser
		APIHost      template.JSStr
	}{
		Repositories: repoList,
		RepoAndJobs:  jobList,
		TrustedUsers: trustedUsers,
		APIHost:      template.JSStr(d.apiHost),
	})
	if err != nil {
		logger.Log.Warn("Failed to render template", zap.Error(err))
	}
}

func (d *Dashboard) handleLogs(w http.ResponseWriter, req *http.Request) {
	s := strings.Split(req.URL.Path, "/")
	if len(s) < 2 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	path := strings.Join(s[2:], "/")
	r, err := d.minio.Get(req.Context(), path)
	if err != nil {
		logger.Log.Warn("Failed get a log data", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer r.Close()
	io.Copy(w, r)
}

func (d *Dashboard) handleNewRepository(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err := req.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if req.FormValue("name") == "" || req.FormValue("url") == "" || req.FormValue("clone_url") == "" {
		logger.Log.Info("Name or url is empty",
			zap.String("content_type", req.Header.Get("Content-Type")),
			zap.String("name", req.FormValue("name")),
			zap.String("url", req.FormValue("url")),
			zap.String("clone_url", req.FormValue("clone_url")),
		)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	private := false
	if req.FormValue("private") != "" {
		private = true
	}
	if _, err := d.dao.Repository.Create(req.Context(), &database.SourceRepository{
		Name:     req.FormValue("name"),
		Url:      req.FormValue("url"),
		CloneUrl: req.FormValue("clone_url"),
		Private:  private,
	}); err != nil {
		logger.Log.Warn("Failed create repository", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (d *Dashboard) handleDeleteRepository(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err := req.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if req.FormValue("id") == "" {
		logger.Log.Info("id is empty", zap.String("id", req.FormValue("id")))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(req.FormValue("id"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := d.dao.Repository.Delete(req.Context(), int32(id)); err != nil {
		logger.Log.Warn("Failed delete repository", zap.Error(err), zap.Int("id", id))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (d *Dashboard) handleAddTrustedUser(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err := req.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if req.FormValue("username") == "" {
		logger.Log.Info("username is empty")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	username := req.FormValue("username")
	c := github.NewClient(nil)
	u, res, err := c.Users.Get(req.Context(), username)
	if err != nil {
		logger.Log.Warn("Failed api request", zap.Error(err), zap.String("username", username))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if res.StatusCode != http.StatusOK {
		logger.Log.Info("User not found", zap.String("username", username))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	_, err = d.dao.TrustedUser.Create(req.Context(), &database.TrustedUser{GithubId: u.GetID(), Username: u.GetLogin()})
	if err != nil {
		logger.Log.Warn("Failed create trusted user", zap.Error(err), zap.String("username", username))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (d *Dashboard) handleReadiness(w http.ResponseWriter, req *http.Request) {
	p := probe.NewProbe(d.dao.RawConnection)
	if !p.Ready(req.Context(), database.SchemaHash) {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
}

func (*Dashboard) handleLiveness(_ http.ResponseWriter, _ *http.Request) {}
