package releasewatcher

import (
	"context"
	"strings"

	"github.com/google/go-github/v85/github"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/build/config"
)

// Item represents one observed release or tag from an external source.
type Item struct {
	Tag        string
	Name       string
	URL        string
	Prerelease bool
}

// ReleaseSource fetches the recent set of releases or tags from a third-party
// repository.
type ReleaseSource interface {
	List(ctx context.Context, externalRepo string, kind config.ExternalReleaseSourceKind) ([]Item, error)
}

type githubSource struct {
	client *github.Client
}

// NewGitHubSource builds a ReleaseSource backed by the GitHub REST API.
func NewGitHubSource(client *github.Client) ReleaseSource {
	return &githubSource{client: client}
}

func (s *githubSource) List(ctx context.Context, externalRepo string, kind config.ExternalReleaseSourceKind) ([]Item, error) {
	owner, repo, ok := splitRepo(externalRepo)
	if !ok {
		return nil, xerrors.Definef("invalid external_repo: %s", externalRepo).WithStack()
	}

	switch kind {
	case config.ExternalReleaseKindTag:
		tags, _, err := s.client.Repositories.ListTags(ctx, owner, repo, &github.ListOptions{PerPage: 30})
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		items := make([]Item, 0, len(tags))
		for _, t := range tags {
			items = append(items, Item{
				Tag: t.GetName(),
				URL: "https://github.com/" + externalRepo + "/releases/tag/" + t.GetName(),
			})
		}
		return items, nil
	default: // ExternalReleaseKindRelease (default)
		releases, _, err := s.client.Repositories.ListReleases(ctx, owner, repo, &github.ListOptions{PerPage: 30})
		if err != nil {
			return nil, xerrors.WithStack(err)
		}
		items := make([]Item, 0, len(releases))
		for _, r := range releases {
			if r.GetDraft() {
				continue
			}
			items = append(items, Item{
				Tag:        r.GetTagName(),
				Name:       r.GetName(),
				URL:        r.GetHTMLURL(),
				Prerelease: r.GetPrerelease(),
			})
		}
		return items, nil
	}
}

func splitRepo(s string) (owner, repo string, ok bool) {
	owner, repo, ok = strings.Cut(s, "/")
	if !ok || owner == "" || repo == "" {
		return "", "", false
	}
	return owner, repo, true
}
