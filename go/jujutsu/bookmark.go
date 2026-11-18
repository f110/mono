package jujutsu

import (
	"context"
	"encoding/json"
	"strings"

	"go.f110.dev/xerrors"
)

type Bookmark struct {
	Name string `json:"name"`
	// Remote is empty for the local bookmark.
	Remote   string `json:"remote"`
	Tracked  bool   `json:"tracked"`
	Conflict bool   `json:"conflict"`
	// Synced is true if the local and the tracked remote bookmarks point to the same commit.
	Synced bool `json:"synced"`
	// Target is the commit ID which the bookmark points to. It is empty for the conflicted bookmark.
	Target string `json:"target"`
	// Ahead and Behind are set to the tracked remote bookmark. Ahead is the number of the commits of the remote bookmark
	// which the local one doesn't have, and Behind is the number of the commits of the local one which the remote one
	// doesn't have.
	Ahead       int    `json:"ahead"`
	Behind      int    `json:"behind"`
	Description string `json:"description"`
}

const bookmarkTemplate = `"{" ++ ` +
	`"\"name\":" ++ json(name) ++ ` +
	`",\"remote\":" ++ json(if(remote, remote, "")) ++ ` +
	`",\"tracked\":" ++ json(tracked) ++ ` +
	`",\"conflict\":" ++ json(conflict) ++ ` +
	`",\"synced\":" ++ json(synced) ++ ` +
	`",\"target\":" ++ json(if(normal_target, normal_target.commit_id().short(), "")) ++ ` +
	`",\"ahead\":" ++ if(tracked, self.tracking_ahead_count().lower(), "0") ++ ` +
	`",\"behind\":" ++ if(tracked, self.tracking_behind_count().lower(), "0") ++ ` +
	`",\"description\":" ++ json(if(normal_target, normal_target.description().first_line(), "")) ++ ` +
	`"}\n"`

func bookmarksArgs() []string {
	return []string{"bookmark", "list", "--all-remotes", "--ignore-working-copy", "-T", bookmarkTemplate}
}

// Bookmarks returns the local and the remote bookmarks. The remote "git" of the colocated repository is excluded.
func (r *Repository) Bookmarks(ctx context.Context) ([]*Bookmark, error) {
	out, err := run(ctx, r.root, bookmarksArgs()...)
	if err != nil {
		return nil, err
	}
	return parseBookmarks(out)
}

func parseBookmarks(out []byte) ([]*Bookmark, error) {
	var bookmarks []*Bookmark
	for line := range strings.Lines(string(out)) {
		b := &Bookmark{}
		if err := json.Unmarshal([]byte(line), b); err != nil {
			return nil, xerrors.WithMessagef(err, "failed to parse the bookmark: %s", line)
		}
		if b.Remote == "git" {
			continue
		}
		bookmarks = append(bookmarks, b)
	}
	return bookmarks, nil
}

// The names of the bookmarks are glob patterns for most of the commands, so the name is passed with "exact:".

func bookmarkSetArgs(name, rev string) []string {
	return []string{"bookmark", "set", "--revision=" + rev, "--", name}
}

func bookmarkMoveArgs(name, rev string) []string {
	return []string{"bookmark", "move", "--to=" + rev, "--allow-backwards", "--", "exact:" + name}
}

func bookmarkDeleteArgs(name string) []string {
	return []string{"bookmark", "delete", "--", "exact:" + name}
}

func bookmarkForgetArgs(name string) []string {
	return []string{"bookmark", "forget", "--", "exact:" + name}
}

func bookmarkTrackArgs(name, remote string) []string {
	return []string{"bookmark", "track", "--remote=exact:" + remote, "--", "exact:" + name}
}

func bookmarkUntrackArgs(name, remote string) []string {
	return []string{"bookmark", "untrack", "--remote=exact:" + remote, "--", "exact:" + name}
}

// BookmarkSet creates the bookmark or moves it to the revision.
func (r *Repository) BookmarkSet(ctx context.Context, name, rev string) (string, error) {
	return r.mutate(ctx, bookmarkSetArgs(name, rev)...)
}

// BookmarkMove moves the bookmark to the revision even if it moves backwards or sideways.
func (r *Repository) BookmarkMove(ctx context.Context, name, rev string) (string, error) {
	return r.mutate(ctx, bookmarkMoveArgs(name, rev)...)
}

// BookmarkDelete deletes the bookmark. The tracked remote bookmarks are deleted on the next push.
func (r *Repository) BookmarkDelete(ctx context.Context, name string) (string, error) {
	return r.mutate(ctx, bookmarkDeleteArgs(name)...)
}

// BookmarkForget forgets the bookmark without deleting the remote bookmarks.
func (r *Repository) BookmarkForget(ctx context.Context, name string) (string, error) {
	return r.mutate(ctx, bookmarkForgetArgs(name)...)
}

func (r *Repository) BookmarkTrack(ctx context.Context, name, remote string) (string, error) {
	return r.mutate(ctx, bookmarkTrackArgs(name, remote)...)
}

func (r *Repository) BookmarkUntrack(ctx context.Context, name, remote string) (string, error) {
	return r.mutate(ctx, bookmarkUntrackArgs(name, remote)...)
}

func gitFetchArgs() []string {
	return []string{"git", "fetch"}
}

// gitPushArgs returns the arguments of jj git push. It pushes the bookmarks of revsets.git-push if bookmark is empty.
func gitPushArgs(bookmark string) []string {
	if bookmark == "" {
		return []string{"git", "push"}
	}
	return []string{"git", "push", "--bookmark=exact:" + bookmark}
}

func (r *Repository) GitFetch(ctx context.Context) (string, error) {
	return r.mutate(ctx, gitFetchArgs()...)
}

func (r *Repository) GitPush(ctx context.Context, bookmark string) (string, error) {
	return r.mutate(ctx, gitPushArgs(bookmark)...)
}
