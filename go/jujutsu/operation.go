package jujutsu

import (
	"context"
	"strings"
)

// The operations return the message of jj. jj writes the message (e.g. "Working copy (@) now at: ...") to stderr.
// The revisions are passed after "--" and the message is joined with "=" not to be interpreted as the options
// (e.g. "-r --config=x" is the option --config).

func newArgs(revs []string) []string {
	return append([]string{"new", "--"}, revs...)
}

func editArgs(rev string) []string {
	return []string{"edit", "--", rev}
}

func abandonArgs(revs []string) []string {
	return append([]string{"abandon", "--"}, revs...)
}

func describeArgs(rev, message string) []string {
	return []string{"describe", "--message=" + message, "--", rev}
}

func commitArgs(message string) []string {
	return []string{"commit", "--message=" + message}
}

func (r *Repository) New(ctx context.Context, revs []string) (string, error) {
	return r.mutate(ctx, newArgs(revs)...)
}

func (r *Repository) Edit(ctx context.Context, rev string) (string, error) {
	return r.mutate(ctx, editArgs(rev)...)
}

func (r *Repository) Abandon(ctx context.Context, revs []string) (string, error) {
	return r.mutate(ctx, abandonArgs(revs)...)
}

func (r *Repository) Describe(ctx context.Context, rev, message string) (string, error) {
	return r.mutate(ctx, describeArgs(rev, message)...)
}

// Commit describes the working copy and creates the new working copy on it.
func (r *Repository) Commit(ctx context.Context, message string) (string, error) {
	return r.mutate(ctx, commitArgs(message)...)
}

func (r *Repository) Undo(ctx context.Context) (string, error) {
	return r.mutate(ctx, "undo")
}

func (r *Repository) mutate(ctx context.Context, args ...string) (string, error) {
	_, stderr, err := runWithStderr(ctx, r.root, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(stderr)), nil
}
