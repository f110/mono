package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/google/go-github/v49/github"
	"go.f110.dev/protoc-ddl/probe"
	"go.uber.org/zap"

	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/enumerable"
	"go.f110.dev/mono/go/logger"
	"go.f110.dev/mono/go/storage"
)

const (
	NumberOfTaskPerJob = 10
)

type Dashboard struct {
	*http.Server

	dao         dao.Options
	apiHost     string
	internalAPI string
	minio       *storage.MinIO
}

func NewDashboard(addr string, daoOpt dao.Options, apiHost, internalAPI string, bucket string, minioOpt storage.MinIOOptions) *Dashboard {
	d := &Dashboard{
		dao:         daoOpt,
		apiHost:     apiHost,
		internalAPI: internalAPI,
		minio:       storage.NewMinIOStorage(bucket, minioOpt),
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
	mux.HandleFunc("/server_info", d.handleServerInfo)
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

func (d *Dashboard) handleIndex(w http.ResponseWriter, req *http.Request) {
	allRepo, err := d.dao.Repository.ListAll(req.Context())
	if err != nil {
		logger.Log.Warn("Failed get repository", logger.Error(err))
		return
	}

	var repoId int32
	if v := req.URL.Query().Get("repo"); v != "" {
		i, err := strconv.Atoi(v)
		if err != nil {
			http.Error(w, "invalid format", http.StatusBadRequest)
			return
		}
		for _, v := range allRepo {
			if v.Id == int32(i) {
				repoId = int32(i)
				break
			}
		}
	}
	var tasks []*database.Task
	if repoId != 0 {
		t, err := d.dao.Task.ListByRepositoryId(req.Context(), repoId, dao.Limit(100), dao.Desc)
		if err != nil {
			logger.Log.Warn("Failed to get the task", logger.Error(err))
			return
		}
		tasks = t
	} else {
		t, err := d.dao.Task.ListAll(req.Context(), dao.Limit(100), dao.Desc)
		if err != nil {
			logger.Log.Warn("Failed to get the task", logger.Error(err))
			return
		}
		tasks = t
	}
	repoList := make(map[int32]*database.SourceRepository)
	taskList := make([]*Task, 0, len(tasks))
	for _, v := range tasks {
		revUrl := ""
		if strings.Contains(v.Repository.Url, "https://github.com") {
			revUrl = v.Repository.Url + "/commit/" + v.Revision
		}
		taskList = append(taskList, &Task{
			Task:        v,
			RevisionUrl: revUrl,
		})
		repoList[v.RepositoryId] = v.Repository
	}
	var jobs []string
	for repoId := range repoList {
		tasks, err := d.dao.Task.ListUniqJobName(req.Context(), repoId)
		if err != nil {
			logger.Log.Error("Failed to get job list", logger.Error(err))
			return
		}
		for _, v := range tasks {
			jobs = append(jobs, v.JobName)
		}
	}
	jobs = enumerable.Uniq(jobs, func(t string) string { return t })

	trustedUsers, err := d.dao.TrustedUser.ListAll(req.Context())
	if err != nil {
		logger.Log.Warn("Failed get trusted user", logger.Error(err))
		return
	}

	err = IndexTemplate.Execute(w, struct {
		Repositories       []*database.SourceRepository
		TrustedUsers       []*database.TrustedUser
		Tasks              []*Task
		Jobs               []string
		FilterRepositoryId int32
		APIHost            template.JSStr
	}{
		Repositories:       allRepo,
		TrustedUsers:       trustedUsers,
		Tasks:              taskList,
		Jobs:               jobs,
		FilterRepositoryId: repoId,
		APIHost:            template.JSStr(d.apiHost),
	})
	if err != nil {
		logger.Log.Warn("Failed to render template", logger.Error(err))
	}
}

func (d *Dashboard) handleServerInfo(w http.ResponseWriter, req *http.Request) {
	readinessReq, err := http.NewRequestWithContext(req.Context(), http.MethodGet, fmt.Sprintf("%s/readiness", d.internalAPI), nil)
	if err != nil {
		logger.Log.Error("Failed to create the request", logger.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	res, err := http.DefaultClient.Do(readinessReq)
	if err != nil {
		logger.Log.Error("Failed to request", logger.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	readinessAPI := &struct {
		Versions []string `json:"versions"`
	}{}
	if err := json.NewDecoder(res.Body).Decode(readinessAPI); err != nil {
		logger.Log.Error("Failed to parse readiness response", logger.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	versions := readinessAPI.Versions
	sort.Sort(sort.Reverse(sort.StringSlice(versions)))

	err = ServerInfoTemplate.Execute(w, struct {
		Versions []string
	}{
		Versions: readinessAPI.Versions,
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
		logger.Log.Info("not found task", zap.Int("id", id))
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	reports, err := d.dao.TestReport.ListByTaskId(req.Context(), task.Id)
	if err != nil {
		logger.Log.Info("failed to get report", zap.Int32("task_id", task.Id))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	jobConf := &config.Job{}
	if task.JobConfiguration != nil && len(*task.JobConfiguration) > 0 {
		if err := config.UnmarshalJob([]byte(*task.JobConfiguration), jobConf); err != nil {
			logger.Log.Warn("Failed to parse job configuration", logger.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else if len(task.ParsedJobConfiguration) > 0 {
		if err := config.UnmarshalJob(task.ParsedJobConfiguration, jobConf); err != nil {
			logger.Log.Warn("Failed to parse job configuration by gob", logger.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	revUrl := ""
	if strings.Contains(task.Repository.Url, "https://github.com") {
		revUrl = task.Repository.Url + "/commit/" + task.Revision
	}
	err = DetailTemplate.Execute(w, struct {
		Task       *Task
		Job        *config.Job
		TestReport []*database.TestReport
		APIHost    template.JSStr
	}{
		Task: &Task{
			Task:        task,
			RevisionUrl: revUrl,
		},
		Job:        jobConf,
		TestReport: reports,
		APIHost:    template.JSStr(d.apiHost),
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
	defer r.Body.Close()
	io.Copy(w, r.Body)
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
