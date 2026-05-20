package releasewatcher

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/v85/github"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/ctxutil"
	"go.f110.dev/mono/go/logger/slogger"
)

// Builder is the subset of coordinator.BazelBuilder used to dispatch a task.
// Defined here to avoid a cyclic import with the coordinator package.
type Builder interface {
	Build(ctx context.Context, repo *database.SourceRepository, job *config.JobV2, revision, bazelVersion, command string, targets, platforms []string, via string, isMainBranch bool) ([]*database.Task, error)
}

// trigger is the in-memory representation of one external release trigger.
// It is derived from the cue file of the owning SourceRepository.
type trigger struct {
	ID                int32
	SourceRepoID      int32
	JobName           string
	Provider          string
	ExternalRepo      string
	Kind              config.ExternalReleaseSourceKind
	TagPattern        string
	IncludePrerelease bool

	pattern *regexp.Regexp
}

func (t *trigger) matches(item Item) bool {
	if t.pattern != nil && !t.pattern.MatchString(item.Tag) {
		return false
	}
	if t.Kind == config.ExternalReleaseKindRelease && item.Prerelease && !t.IncludePrerelease {
		return false
	}
	return true
}

type pollKey struct {
	provider     string
	externalRepo string
	kind         config.ExternalReleaseSourceKind
}

func (k pollKey) String() string {
	return fmt.Sprintf("%s:%s:%s", k.provider, k.externalRepo, k.kind)
}

// Manager polls external sources for new releases/tags and dispatches build
// tasks for matching triggers. Designed to run as a singleton inside the
// elected leader. Trigger definitions are read fresh from the DB on every
// tick; the api package owns the rows.
type Manager struct {
	dao          dao.Options
	builder      Builder
	source       ReleaseSource
	githubClient *github.Client
	interval     time.Duration
}

func NewManager(builder Builder, daoOpt dao.Options, githubClient *github.Client, source ReleaseSource, interval time.Duration) *Manager {
	if source == nil {
		source = NewGitHubSource(githubClient)
	}
	return &Manager{
		dao:          daoOpt,
		builder:      builder,
		source:       source,
		githubClient: githubClient,
		interval:     interval,
	}
}

// Start runs the polling loop until ctx is cancelled. Intended to be invoked
// in its own goroutine after leader election.
func (m *Manager) Start(ctx context.Context) {
	slogger.Log.Info("Start releasewatcher", slog.Duration("interval", m.interval))
	t := time.NewTicker(m.interval)
	defer t.Stop()

	m.tickWithTimeout(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.tickWithTimeout(ctx)
		}
	}
}

func (m *Manager) tickWithTimeout(parent context.Context) {
	ctx, cancel := ctxutil.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	m.tick(ctx)
}

func (m *Manager) tick(ctx context.Context) {
	rows, err := m.dao.ExternalReleaseTrigger.ListAll(ctx)
	if err != nil {
		slogger.Log.Warn("Failed to list external_release_trigger", slogger.E(err))
		return
	}
	groups := groupTriggers(rows)
	if len(groups) == 0 {
		return
	}

	repoCache := make(map[int32]*database.SourceRepository)

	for key, triggers := range groups {
		items, err := m.source.List(ctx, key.externalRepo, key.kind)
		if err != nil {
			slogger.Log.Warn("Failed to list external releases",
				slog.String("repo", key.externalRepo),
				slog.String("kind", string(key.kind)),
				slogger.E(err))
			continue
		}
		if len(items) == 0 {
			continue
		}
		// Process oldest -> newest so history rows are inserted in chronological
		// order if the source returns newest-first.
		for i := len(items) - 1; i >= 0; i-- {
			item := items[i]
			for _, t := range triggers {
				if !t.matches(item) {
					continue
				}
				if processed, err := m.dao.ExternalReleaseHistory.SelectProcessed(ctx, t.SourceRepoID, t.JobName, t.ExternalRepo, item.Tag); err != nil && !errors.Is(err, sql.ErrNoRows) {
					slogger.Log.Warn("Failed to query external_release_history",
						slog.Int("repo_id", int(t.SourceRepoID)),
						slog.String("job", t.JobName),
						slog.String("tag", item.Tag),
						slogger.E(err))
					continue
				} else if processed != nil {
					continue
				}
				repo, ok := repoCache[t.SourceRepoID]
				if !ok {
					r, err := m.dao.Repository.Select(ctx, t.SourceRepoID)
					if err != nil {
						slogger.Log.Warn("Failed to load source_repository",
							slog.Int("id", int(t.SourceRepoID)), slogger.E(err))
						continue
					}
					repoCache[t.SourceRepoID] = r
					repo = r
				}
				if err := m.dispatch(ctx, repo, t, item); err != nil {
					slogger.Log.Warn("Failed to dispatch external release task",
						slog.Int("repo_id", int(t.SourceRepoID)),
						slog.String("job", t.JobName),
						slog.String("tag", item.Tag),
						slogger.E(err))
				}
			}
		}
	}
}

func (m *Manager) dispatch(ctx context.Context, repo *database.SourceRepository, t *trigger, item Item) error {
	owner, repoName := ownerRepoFromURL(repo.Url)
	if owner == "" || repoName == "" {
		return xerrors.Definef("invalid repository URL: %s", repo.Url).WithStack()
	}
	conf, err := config.ReadFromRepository(ctx, m.githubClient, owner, repoName)
	if err != nil {
		return xerrors.WithStack(err)
	}
	var job *config.JobV2
	for _, j := range conf.Jobs {
		if j.Name == t.JobName && containsEvent(j.Event, config.EventExternalRelease) {
			job = j
			break
		}
	}
	if job == nil {
		return xerrors.Definef("job %s not found in repository %d", t.JobName, repo.Id).WithStack()
	}
	commit, _, err := m.githubClient.Repositories.GetCommit(ctx, owner, repoName, "HEAD", nil)
	if err != nil {
		return xerrors.WithStack(err)
	}
	revision := commit.GetSHA()

	history, err := m.dao.ExternalReleaseHistory.Create(ctx, &database.ExternalReleaseHistory{
		RepositoryId: repo.Id,
		JobName:      t.JobName,
		ExternalRepo: t.ExternalRepo,
		Tag:          item.Tag,
		ProcessedAt:  time.Now(),
	})
	if err != nil {
		return xerrors.WithStack(err)
	}

	jobCopy := job.Copy()
	if jobCopy.Env == nil {
		jobCopy.Env = make(map[string]any)
	} else {
		jobCopy.Env = maps.Clone(jobCopy.Env)
	}
	jobCopy.Env["EXTERNAL_RELEASE_TAG"] = item.Tag
	jobCopy.Env["EXTERNAL_RELEASE_NAME"] = item.Name
	jobCopy.Env["EXTERNAL_RELEASE_URL"] = item.URL
	jobCopy.Env["EXTERNAL_RELEASE_REPO"] = t.ExternalRepo
	jobCopy.Env["EXTERNAL_RELEASE_PRERELEASE"] = fmt.Sprintf("%t", item.Prerelease)

	tasks, err := m.builder.Build(ctx, repo, jobCopy, revision, conf.BazelVersion, jobCopy.Command, jobCopy.Targets, jobCopy.Platforms, "external_release", false)
	if err != nil {
		return xerrors.WithStack(err)
	}
	if len(tasks) > 0 {
		history.TaskId = tasks[0].Id
		if err := m.dao.ExternalReleaseHistory.Update(ctx, history); err != nil {
			slogger.Log.Warn("Failed to update external_release_history.task_id",
				slog.Int("history_id", int(history.Id)), slogger.E(err))
		}
	}
	slogger.Log.Info("Dispatched external release build",
		slog.String("external_repo", t.ExternalRepo),
		slog.String("tag", item.Tag),
		slog.String("job", t.JobName),
		slog.Int("repo_id", int(repo.Id)))
	return nil
}

func groupTriggers(rows []*database.ExternalReleaseTrigger) map[pollKey][]*trigger {
	out := make(map[pollKey][]*trigger)
	for _, r := range rows {
		t, err := triggerFromRow(r)
		if err != nil {
			slogger.Log.Warn("Skip invalid external_release_trigger", slog.Int("id", int(r.Id)), slogger.E(err))
			continue
		}
		k := pollKey{provider: t.Provider, externalRepo: t.ExternalRepo, kind: t.Kind}
		out[k] = append(out[k], t)
	}
	return out
}

func triggerFromRow(r *database.ExternalReleaseTrigger) (*trigger, error) {
	t := &trigger{
		ID:                r.Id,
		SourceRepoID:      r.RepositoryId,
		JobName:           r.JobName,
		Provider:          r.Provider,
		ExternalRepo:      r.ExternalRepo,
		Kind:              config.ExternalReleaseSourceKind(r.Kind),
		TagPattern:        r.TagPattern,
		IncludePrerelease: r.IncludePrerelease,
	}
	if t.TagPattern != "" {
		re, err := regexp.Compile(t.TagPattern)
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		t.pattern = re
	}
	return t, nil
}

func containsEvent(events []config.EventType, want config.EventType) bool {
	for _, e := range events {
		if e == want {
			return true
		}
	}
	return false
}

// ownerRepoFromURL parses an HTML URL of the shape https://github.com/<owner>/<repo>(.git)?
// into its owner and repo components. Returns empty strings if the URL is
// malformed; callers log + skip in that case.
func ownerRepoFromURL(repoURL string) (string, string) {
	repoURL = strings.TrimSuffix(repoURL, ".git")
	idx := strings.LastIndex(repoURL, "/")
	if idx < 0 {
		return "", ""
	}
	name := repoURL[idx+1:]
	rest := repoURL[:idx]
	idx = strings.LastIndex(rest, "/")
	if idx < 0 {
		return "", name
	}
	owner := rest[idx+1:]
	return owner, name
}
