package jujutsu

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"go.f110.dev/xerrors"
)

type Operation struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	User        string `json:"user"`
	// Time is the unix time when the operation started.
	Time    int64 `json:"time,string"`
	Current bool  `json:"current"`
}

const operationTemplate = `"{" ++ ` +
	`"\"id\":" ++ json(self.id().short()) ++ ` +
	`",\"description\":" ++ json(self.description()) ++ ` +
	`",\"user\":" ++ json(self.user()) ++ ` +
	`",\"time\":" ++ json(self.time().start().format("%s")) ++ ` +
	`",\"current\":" ++ json(self.current_operation()) ++ ` +
	`"}\n"`

func operationsArgs(limit int) []string {
	args := []string{"op", "log", "--no-graph", "--ignore-working-copy", "-T", operationTemplate}
	if limit > 0 {
		args = append(args, "--limit", strconv.Itoa(limit))
	}
	return args
}

// Operations returns the operations from the newest.
func (r *Repository) Operations(ctx context.Context, limit int) ([]*Operation, error) {
	out, err := run(ctx, r.root, operationsArgs(limit)...)
	if err != nil {
		return nil, err
	}
	var ops []*Operation
	for line := range strings.Lines(string(out)) {
		op := &Operation{}
		if err := json.Unmarshal([]byte(line), op); err != nil {
			return nil, xerrors.WithMessagef(err, "failed to parse the operation: %s", line)
		}
		ops = append(ops, op)
	}
	return ops, nil
}

// OperationRevert creates the operation which reverts the operation.
func (r *Repository) OperationRevert(ctx context.Context, id string) (string, error) {
	return r.mutate(ctx, "op", "revert", "--", id)
}

// OperationRestore restores the repository to the state at the operation. The operations after it are discarded.
func (r *Repository) OperationRestore(ctx context.Context, id string) (string, error) {
	return r.mutate(ctx, "op", "restore", "--", id)
}

type EvologEntry struct {
	CommitID string `json:"commit_id"`
	// Timestamp is the unix time of the committer.
	Timestamp    int64    `json:"time,string"`
	Operation    string   `json:"operation"`
	Predecessors []string `json:"predecessors"`
	// Files is the changes which the version adds relative to its parents. It is not the diff from the predecessor, so
	// the rebase doesn't appear as the change.
	Files []*FileDiff `json:"-"`
}

// evologTemplate outputs the entry as JSON, the record separator and the diff in the git format. The entries are
// separated by the group separator.
const evologTemplate = `"{" ++ ` +
	`"\"commit_id\":" ++ json(commit.commit_id().short()) ++ ` +
	`",\"time\":" ++ json(commit.committer().timestamp().format("%s")) ++ ` +
	`",\"operation\":" ++ json(operation.description()) ++ ` +
	`",\"predecessors\":[" ++ predecessors.map(|p| json(p.commit_id().short())).join(",") ++ "]" ++ ` +
	`"}\x1E" ++ self.inter_diff().git() ++ "\x1D"`

func evologArgs(rev string) []string {
	return []string{"evolog", "--no-graph", "--color", "never", "--ignore-working-copy", "--config", "diff.git.show-path-prefix=true", "--revisions=" + rev, "-T", evologTemplate}
}

// Evolog returns the versions of the revision from the newest.
func (r *Repository) Evolog(ctx context.Context, rev string) ([]*EvologEntry, error) {
	out, err := run(ctx, r.root, evologArgs(rev)...)
	if err != nil {
		return nil, err
	}
	return parseEvolog(string(out))
}

func parseEvolog(out string) ([]*EvologEntry, error) {
	var entries []*EvologEntry
	for record := range strings.SplitSeq(out, "\x1D") {
		if strings.TrimSpace(record) == "" {
			continue
		}
		head, diff, _ := strings.Cut(record, "\x1E")
		e := &EvologEntry{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(head)), e); err != nil {
			return nil, xerrors.WithMessagef(err, "failed to parse the entry: %s", head)
		}
		files, err := parseGitDiff(diff)
		if err != nil {
			return nil, err
		}
		e.Files = files
		entries = append(entries, e)
	}
	return entries, nil
}
