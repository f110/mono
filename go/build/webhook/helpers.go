package webhook

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/go-github/v85/github"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/build/config"
	"go.f110.dev/mono/go/build/database"
	"go.f110.dev/mono/go/build/database/dao"
	"go.f110.dev/mono/go/logger/slogger"
)

// unmarshalPayload deserializes the raw webhook payload into the supplied
// event struct. Errors are wrapped so the scheduler can log a clean message.
func unmarshalPayload(ev *database.GithubEvent, out any) error {
	if err := json.Unmarshal(ev.Payload, out); err != nil {
		return xerrors.WithMessagef(err, "failed to decode %s payload", ev.EventType)
	}
	return nil
}

// fetchConfig reads the build configuration for owner/repoName at revision.
// useCommittedConfig=true reads the config of revision itself (used for
// pushes/releases); =false reads HEAD of the default branch (used for pull
// requests where revision lives outside the trusted branch). Returns
// (nil, nil) when the repository has no jobs configured.
func (r *PushReconciler) fetchConfig(ctx context.Context, owner, repoName, revision string) (*config.Config, error) {
	return fetchBuildConfig(ctx, r.githubClient, owner, repoName, revision, true)
}

func fetchBuildConfig(ctx context.Context, gh *github.Client, owner, repoName, revision string, useCommittedConfig bool) (*config.Config, error) {
	var sha string
	if useCommittedConfig {
		c, _, err := gh.Repositories.GetCommit(ctx, owner, repoName, revision, nil)
		if err != nil {
			return nil, xerrors.WithMessagef(err, "failed to get the commit: %s", revision)
		}
		sha = c.GetCommit().GetTree().GetSHA()
	} else {
		c, _, err := gh.Repositories.GetCommit(ctx, owner, repoName, "HEAD", nil)
		if err != nil {
			return nil, xerrors.WithMessage(err, "failed to get HEAD commit")
		}
		sha = c.GetCommit().GetTree().GetSHA()
	}
	conf, err := config.ReadFromSpecifiedCommit(ctx, gh, owner, repoName, sha)
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	if len(conf.Jobs) == 0 {
		return nil, nil
	}
	return conf, nil
}

// dispatchBuilds runs the legacy a.build loop: filters jobs to known commands
// and asks the Builder to schedule each one. Returns the tasks the builder
// produced so the caller can checkpoint them.
func dispatchBuilds(ctx context.Context, builder Builder, owner, repoName string, repo *database.SourceRepository, jobs []*config.JobV2, bazelVersion, revision, via string, isMainBranch bool) ([]*database.Task, error) {
	if repo == nil {
		return nil, nil
	}
	var dispatched []*database.Task
	for _, v := range jobs {
		switch v.Command {
		case "build", "test", "run":
		default:
			slogger.Log.Warn("Skip creating job", slog.String("command", v.Command))
			continue
		}
		tasks, err := builder.Build(ctx, repo, v, revision, bazelVersion, v.Command, v.Targets, v.Platforms, via, isMainBranch)
		if err != nil {
			return dispatched, xerrors.WithMessagef(err, "failed to start job %s for %s/%s", v.Name, owner, repoName)
		}
		dispatched = append(dispatched, tasks...)
	}
	return dispatched, nil
}

// reconcileExternalReleaseTriggers writes the external_release_trigger rows
// for repo to match the external_release jobs in the freshly-parsed config.
// Lifted from api/server.go with no behavioural change; the leader's
// releasewatcher reads these rows on every tick.
func reconcileExternalReleaseTriggers(ctx context.Context, daos dao.Options, repo *database.SourceRepository, jobs []*config.JobV2) error {
	want := make(map[string]*config.JobV2)
	for _, j := range jobs {
		hasEvent := false
		for _, e := range j.Event {
			if e == config.EventExternalRelease {
				hasEvent = true
				break
			}
		}
		if !hasEvent {
			continue
		}
		if j.ExternalSource == nil {
			slogger.Log.Warn("external_release job without external_source", slog.String("job", j.Name))
			continue
		}
		want[j.Name] = j
	}

	existing, err := daos.ExternalReleaseTrigger.ListByRepositoryId(ctx, repo.Id)
	if err != nil {
		return xerrors.WithStack(err)
	}
	existingByName := make(map[string]*database.ExternalReleaseTrigger, len(existing))
	for _, e := range existing {
		existingByName[e.JobName] = e
	}

	for name, e := range existingByName {
		if _, keep := want[name]; keep {
			continue
		}
		if err := daos.ExternalReleaseTrigger.Delete(ctx, e.Id); err != nil {
			return xerrors.WithStack(err)
		}
	}

	for name, j := range want {
		src := j.ExternalSource
		kind := src.Kind
		if kind == "" {
			kind = config.ExternalReleaseKindRelease
		}
		if e, ok := existingByName[name]; ok {
			e.Provider = src.Provider
			e.ExternalRepo = src.Repo
			e.Kind = string(kind)
			e.TagPattern = src.TagPattern
			e.IncludePrerelease = src.IncludePrerelease
			if err := daos.ExternalReleaseTrigger.Update(ctx, e); err != nil {
				return xerrors.WithStack(err)
			}
		} else {
			_, err := daos.ExternalReleaseTrigger.Create(ctx, &database.ExternalReleaseTrigger{
				RepositoryId:      repo.Id,
				JobName:           name,
				Provider:          src.Provider,
				ExternalRepo:      src.Repo,
				Kind:              string(kind),
				TagPattern:        src.TagPattern,
				IncludePrerelease: src.IncludePrerelease,
			})
			if err != nil {
				return xerrors.WithStack(err)
			}
		}
	}
	return nil
}

// stringPtr is a small helper used when posting strings to the GitHub API
// (which takes *string fields).
func stringPtr(s string) *string { return &s }
