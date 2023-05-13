package web

import (
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/go-github/v49/github"
	"go.f110.dev/protoc-ddl/probe"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/storage"
)

const (
	NumberOfTaskPerJob = 10
)

type Dashboard struct {
	*http.Server

	dao     dao.Options
	apiHost string
	minio   *storage.MinIO
}

func NewDashboard(addr string, daoOpt dao.Options, apiHost string, bucket string, minioOpt storage.MinIOOptions) *Dashboard {
	d := &Dashboard{
		dao:     daoOpt,
		apiHost: apiHost,
		minio:   storage.NewMinIOStorage(bucket, minioOpt),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/favicon.ico", http.NotFound)
	mux.HandleFunc("/liveness", d.handleLiveness)
	mux.HandleFunc("/readiness", d.handleReadiness)
	mux.HandleFunc("/logs/", d.handleLogs)
	mux.HandleFunc("/manifest/", d.handleManifest)
	mux.HandleFunc("/task/", d.handleTask)
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

type Task struct {
	*database.Task
	RevisionUrl string
}

type RepositoryAndTasks struct {
	Repo  *database.SourceRepository
	Tasks []*Task
}

func (d *Dashboard) handleIndex(w http.ResponseWriter, req *http.Request) {
	repoList, err := d.dao.Repository.ListAll(req.Context())
	if err != nil {
		logger.Log.Warn("Failed get repository", zap.Error(err))
		return
	}
	tasks, err := d.dao.Task.ListAll(req.Context(), dao.Limit(100), dao.Desc)
	if err != nil {
		logger.Log.Warn("Failed to get the task", zap.Error(err))
		return
	}
	trustedUsers, err := d.dao.TrustedUser.ListAll(req.Context())
	if err != nil {
		logger.Log.Warn("Failed get trusted user", zap.Error(err))
		return
	}

	repoTaskMap := make(map[int32]*RepositoryAndTasks)
	for _, v := range repoList {
		repoTaskMap[v.Id] = &RepositoryAndTasks{
			Repo:  v,
			Tasks: make([]*Task, 0),
		}
	}
	for _, v := range tasks {
		if _, ok := repoTaskMap[v.RepositoryId]; !ok {
			continue
		}

		revUrl := ""
		if strings.Contains(v.Repository.Url, "https://github.com") {
			revUrl = v.Repository.Url + "/commit/" + v.Revision
		}
		repoTaskMap[v.RepositoryId].Tasks = append(repoTaskMap[v.RepositoryId].Tasks, &Task{
			Task:        v,
			RevisionUrl: revUrl,
		})
	}

	var repoAndTasks []*RepositoryAndTasks
	for _, repo := range repoList {
		repoAndTasks = append(repoAndTasks, &RepositoryAndTasks{
			Repo:  repo,
			Tasks: repoTaskMap[repo.Id].Tasks,
		})
	}
	err = IndexTemplate.Execute(w, struct {
		Repositories []*database.SourceRepository
		RepoAndTasks []*RepositoryAndTasks
		TrustedUsers []*database.TrustedUser
		APIHost      template.JSStr
	}{
		Repositories: repoList,
		RepoAndTasks: repoAndTasks,
		TrustedUsers: trustedUsers,
		APIHost:      template.JSStr(d.apiHost),
	})
	if err != nil {
		logger.Log.Warn("Failed to render template", zap.Error(err))
	}
}

func (d *Dashboard) handleTask(w http.ResponseWriter, req *http.Request) {
	s := strings.Split(req.URL.Path, "/")
	if len(s) < 2 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(s[2])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	task, err := d.dao.Task.Select(req.Context(), int32(id))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	jobConf, err := config.Read(strings.NewReader(task.JobConfiguration), "", "")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	var job *config.Job
	for _, v := range jobConf.Jobs {
		if v.Name == task.JobName {
			job = v
			break
		}
	}
	revUrl := ""
	if strings.Contains(task.Repository.Url, "https://github.com") {
		revUrl = task.Repository.Url + "/commit/" + task.Revision
	}
	err = DetailTemplate.Execute(w, struct {
		Task *Task
		Job  *config.Job
	}{
		Task: &Task{
			Task:        task,
			RevisionUrl: revUrl,
		},
		Job: job,
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
		logger.Log.Warn("Failed to get a log data", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer r.Close()
	io.Copy(w, r)
}

func (d *Dashboard) handleManifest(w http.ResponseWriter, req *http.Request) {
	s := strings.Split(req.URL.Path, "/")
	if len(s) < 2 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(s[2])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	task, err := d.dao.Task.Select(req.Context(), int32(id))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if task.Manifest == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	io.WriteString(w, task.Manifest)
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

	tasks, err := d.dao.Task.ListByRepositoryId(req.Context(), int32(id))
	if err != nil {
		logger.Log.Error("Failed to get tasks", zap.Int32("source_repository_id", int32(id)), zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	for _, v := range tasks {
		if err := d.dao.Task.Delete(req.Context(), v.Id); err != nil {
			logger.Log.Error("Failed to delete job", zap.Int32("id", v.Id), zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	if err := d.dao.Repository.Delete(req.Context(), int32(id)); err != nil {
		logger.Log.Warn("Failed delete repository", zap.Error(err), zap.Int("id", id), zap.Error(err))
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
