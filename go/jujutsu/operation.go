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

// RebaseSource is what to rebase.
type RebaseSource string

const (
	// RebaseRevisions rebases only the revisions (-r).
	RebaseRevisions RebaseSource = "revisions"
	// RebaseSourceDescendants rebases the revisions and the descendants (-s).
	RebaseSourceDescendants RebaseSource = "source"
	// RebaseBranch rebases the whole branch relative to the destination (-b).
	RebaseBranch RebaseSource = "branch"
)

// RebaseTarget is where to rebase.
type RebaseTarget string

const (
	RebaseOnto         RebaseTarget = "onto"
	RebaseInsertAfter  RebaseTarget = "insert-after"
	RebaseInsertBefore RebaseTarget = "insert-before"
)

func rebaseArgs(revs []string, source RebaseSource, destinations []string, target RebaseTarget) []string {
	args := []string{"rebase"}
	for _, v := range revs {
		args = append(args, "--"+string(source)+"="+v)
	}
	for _, v := range destinations {
		args = append(args, "--"+string(target)+"="+v)
	}
	return args
}

func (r *Repository) Rebase(ctx context.Context, revs []string, source RebaseSource, destinations []string, target RebaseTarget) (string, error) {
	return r.mutate(ctx, rebaseArgs(revs, source, destinations, target)...)
}

// squashArgs returns the arguments of jj squash. The description of the destination is kept if message is empty,
// otherwise message becomes the description. jj opens the editor without them if both have the descriptions.
func squashArgs(from []string, into, message string) []string {
	args := []string{"squash"}
	for _, v := range from {
		args = append(args, "--from="+v)
	}
	args = append(args, "--into="+into)
	if message == "" {
		return append(args, "--use-destination-message")
	}
	return append(args, "--message="+message)
}

func (r *Repository) Squash(ctx context.Context, from []string, into, message string) (string, error) {
	return r.mutate(ctx, squashArgs(from, into, message)...)
}

// splitArgs returns the arguments of jj split which moves the files into the new revision before the revision.
// message is the description of the new revision. The rest keeps the original description.
func splitArgs(rev, message string, files []string) []string {
	args := []string{"split", "--revision=" + rev, "--message=" + message, "--"}
	for _, f := range files {
		args = append(args, rootFile(f))
	}
	return args
}

// restoreArgs returns the arguments of jj restore which discards the changes of the files in the revision.
func restoreArgs(rev string, files []string) []string {
	args := []string{"restore", "--changes-in=" + rev, "--"}
	for _, f := range files {
		args = append(args, rootFile(f))
	}
	return args
}

func (r *Repository) Split(ctx context.Context, rev, message string, files []string) (string, error) {
	return r.mutate(ctx, splitArgs(rev, message, files)...)
}

func (r *Repository) Restore(ctx context.Context, rev string, files []string) (string, error) {
	return r.mutate(ctx, restoreArgs(rev, files)...)
}
