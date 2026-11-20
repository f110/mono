// Package review provides the reviews of the changes of jj. The review is saved as a file per review and the comments
// follow the lines when the revisions are rewritten.
package review

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/jujutsu"
)

var (
	ErrNotFound        = xerrors.Define("not found")
	ErrInvalidArgument = xerrors.Define("invalid argument")
	ErrInvalidState    = xerrors.Define("invalid state")
)

type State string

const (
	StateOpen      State = "open"
	StateFinished  State = "finished"
	StateApproved  State = "approved"
	StateDiscarded State = "discarded"
)

// Side is the side of the diff which the lines of the comment belong to.
type Side string

const (
	// SideNew is the file at the commit of the change.
	SideNew Side = "new"
	// SideOld is the file at the parent of the commit of the change.
	SideOld Side = "old"
)

const (
	AuthorUser  = "user"
	AuthorAgent = "agent"
)

type Review struct {
	ID         string `json:"id"`
	Repository string `json:"repository"`
	// Changes are the change IDs under review.
	Changes  []string   `json:"changes"`
	State    State      `json:"state"`
	Rounds   []*Round   `json:"rounds"`
	Comments []*Comment `json:"comments"`
	// Viewed is the viewed marks of the files. The file without the mark is not viewed.
	Viewed    []*ViewedFile `json:"viewed,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// Round is the commits of the changes at the start of the round. The number of the round is the index + 1.
type Round struct {
	// Commits maps the change ID to the commit ID. The change which disappeared (e.g. abandoned) is absent.
	Commits   map[string]string `json:"commits"`
	CreatedAt time.Time         `json:"created_at"`
}

// Comment is the comment on the review. The target is the whole review if ChangeID is empty. Otherwise the target is
// the file of Path, or the description of the commit if Path is empty. The target is the lines of it if StartLine is
// not 0. The lines of the description are always SideNew.
type Comment struct {
	ID        string `json:"id"`
	ChangeID  string `json:"change_id,omitempty"`
	Path      string `json:"path,omitempty"`
	Side      Side   `json:"side,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	// Anchor is the content of the lines. It is used to find the lines after the file is changed.
	Anchor string `json:"anchor,omitempty"`
	// Drifted is true if the anchor is not found in the latest round.
	Drifted bool `json:"drifted,omitempty"`
	// Round is the number of the round in which the comment was written.
	Round     int       `json:"round"`
	Body      string    `json:"body"`
	Author    string    `json:"author"`
	Resolved  bool      `json:"resolved,omitempty"`
	Replies   []*Reply  `json:"replies,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ViewedFile is the viewed mark of the file of the change. Viewed false is the explicit mark of not viewed.
type ViewedFile struct {
	ChangeID string `json:"change_id"`
	Path     string `json:"path"`
	Viewed   bool   `json:"viewed"`
}

type Reply struct {
	ID        string    `json:"id"`
	Body      string    `json:"body"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
}

// Repository is the operations of jj used by the review. *jujutsu.Repository implements it.
type Repository interface {
	Root() string
	Snapshot(ctx context.Context) error
	Log(ctx context.Context, revset string, limit int) ([]*jujutsu.LogRow, error)
	File(ctx context.Context, rev, path string) ([]byte, error)
	Diff(ctx context.Context, rev string) ([]*jujutsu.FileDiff, error)
	DiffRange(ctx context.Context, from, to string) ([]*jujutsu.FileDiff, error)
	Show(ctx context.Context, rev string) (*jujutsu.Commit, error)
	Interdiff(ctx context.Context, from, to string) ([]*jujutsu.FileDiff, error)
}

var changeIDPattern = regexp.MustCompile(`^[k-z]+$`)

// New returns the review of the changes. The first round is the current commits of the changes. The changed files
// which match autoViewed are marked as viewed.
func New(ctx context.Context, repo Repository, changes, autoViewed []string, now time.Time) (*Review, error) {
	if len(changes) == 0 {
		return nil, xerrors.WithMessage(ErrInvalidArgument, "changes are required")
	}
	commits, err := currentCommits(ctx, repo, changes)
	if err != nil {
		return nil, err
	}
	for _, v := range changes {
		if _, ok := commits[v]; !ok {
			return nil, xerrors.WithMessagef(ErrNotFound, "change %s", v)
		}
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}
	r := &Review{
		ID:         id,
		Repository: repo.Root(),
		Changes:    slices.Clone(changes),
		State:      StateOpen,
		Rounds:     []*Round{{Commits: commits, CreatedAt: now}},
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := r.markAutoViewed(ctx, repo, autoViewed); err != nil {
		return nil, err
	}
	return r, nil
}

// currentCommits returns the commit IDs of the changes. The working copy is snapshotted to include the changes of the
// files which are not snapshotted yet.
func currentCommits(ctx context.Context, repo Repository, changes []string) (map[string]string, error) {
	revsets := make([]string, 0, len(changes))
	for _, v := range changes {
		if !changeIDPattern.MatchString(v) {
			return nil, xerrors.WithMessagef(ErrInvalidArgument, "%q is not a change ID", v)
		}
		revsets = append(revsets, "change_id("+v+")")
	}
	if err := repo.Snapshot(ctx); err != nil {
		return nil, err
	}
	rows, err := repo.Log(ctx, strings.Join(revsets, "|"), 0)
	if err != nil {
		return nil, err
	}
	return resolveCommits(changes, rows)
}

// resolveCommits maps the change IDs to the commit IDs. The change ID in changes may be the prefix of the change ID of
// the commit.
func resolveCommits(changes []string, rows []*jujutsu.LogRow) (map[string]string, error) {
	commits := make(map[string]string, len(changes))
	for _, row := range rows {
		for _, v := range changes {
			if !strings.HasPrefix(row.Commit.ChangeID, v) {
				continue
			}
			if _, ok := commits[v]; ok {
				return nil, xerrors.WithMessagef(ErrInvalidState, "change %s is divergent", v)
			}
			commits[v] = row.Commit.CommitID
		}
	}
	return commits, nil
}

// LatestRound returns the number of the latest round.
func (r *Review) LatestRound() int {
	return len(r.Rounds)
}

// Commit returns the commit ID of the change at the round. The number of the round starts from 1.
func (r *Review) Commit(round int, changeID string) (string, bool) {
	if round < 1 || round > len(r.Rounds) {
		return "", false
	}
	v, ok := r.Rounds[round-1].Commits[changeID]
	return v, ok
}

// NextRound starts the next round with the current commits of the changes and moves the comments to the lines of the
// anchors. The viewed marks of the files changed from the previous round are removed, and then the changed files which
// match autoViewed and have no mark are marked as viewed. The review is opened again.
func (r *Review) NextRound(ctx context.Context, repo Repository, autoViewed []string, now time.Time) error {
	if r.State == StateApproved || r.State == StateDiscarded {
		return xerrors.WithMessagef(ErrInvalidState, "the review is %s", r.State)
	}
	commits, err := currentCommits(ctx, repo, r.Changes)
	if err != nil {
		return err
	}
	for change, commit := range commits {
		prev, ok := r.Commit(r.LatestRound(), change)
		if !ok || prev == commit {
			continue
		}
		files, err := repo.DiffRange(ctx, prev, commit)
		if err != nil {
			return err
		}
		r.Viewed = slices.DeleteFunc(r.Viewed, func(v *ViewedFile) bool {
			return v.ChangeID == change && slices.ContainsFunc(files, func(f *jujutsu.FileDiff) bool { return f.Path == v.Path || f.OldPath == v.Path })
		})
	}
	r.Rounds = append(r.Rounds, &Round{Commits: commits, CreatedAt: now})
	if err := r.markAutoViewed(ctx, repo, autoViewed); err != nil {
		return err
	}

	files := make(map[string][]string)
	for _, c := range r.Comments {
		if c.ChangeID == "" || c.StartLine == 0 {
			continue
		}
		commit, ok := commits[c.ChangeID]
		if !ok {
			c.Drifted = true
			continue
		}
		key := revision(commit, c.Side) + "\x00" + c.Path
		lines, ok := files[key]
		if !ok {
			v, err := targetLines(ctx, repo, commit, c.Path, c.Side)
			if _, jjErr := errors.AsType[*jujutsu.Error](err); err != nil && !jjErr {
				return err
			}
			// The file doesn't exist at the revision if jj fails.
			lines = v
			files[key] = lines
		}
		c.Realign(lines)
	}
	r.State = StateOpen
	r.UpdatedAt = now
	return nil
}

// markAutoViewed marks the changed files of the latest round which match the patterns and have no mark as viewed.
func (r *Review) markAutoViewed(ctx context.Context, repo Repository, patterns []string) error {
	if len(patterns) == 0 {
		return nil
	}
	for _, change := range r.Changes {
		commit, ok := r.Commit(r.LatestRound(), change)
		if !ok {
			continue
		}
		files, err := repo.Diff(ctx, commit)
		if err != nil {
			return err
		}
		for _, f := range files {
			if r.viewedFile(change, f.Path) != nil || !slices.ContainsFunc(patterns, func(p string) bool { return matchPattern(f.Path, p) }) {
				continue
			}
			r.Viewed = append(r.Viewed, &ViewedFile{ChangeID: change, Path: f.Path, Viewed: true})
		}
	}
	return nil
}

func (r *Review) viewedFile(changeID, path string) *ViewedFile {
	for _, v := range r.Viewed {
		if v.ChangeID == changeID && v.Path == path {
			return v
		}
	}
	return nil
}

// SetViewed marks the file of the change as viewed or not viewed. The explicit mark of not viewed prevents the file
// from being marked by the patterns.
func (r *Review) SetViewed(changeID, path string, viewed bool, now time.Time) error {
	if !slices.Contains(r.Changes, changeID) {
		return xerrors.WithMessagef(ErrNotFound, "change %s", changeID)
	}
	if path == "" {
		return xerrors.WithMessage(ErrInvalidArgument, "path is required")
	}
	if v := r.viewedFile(changeID, path); v != nil {
		v.Viewed = viewed
	} else {
		r.Viewed = append(r.Viewed, &ViewedFile{ChangeID: changeID, Path: path, Viewed: viewed})
	}
	r.UpdatedAt = now
	return nil
}

// matchPattern reports whether the path matches the pattern of auto_viewed_patterns. The patterns are the same as crit:
// "dir/" matches the files under the directory at any depth, the pattern with the slash matches the whole path, and
// the other patterns match the name of the file.
func matchPattern(filePath, pattern string) bool {
	switch {
	case pattern == "":
		return false
	case strings.HasSuffix(pattern, "/"):
		return strings.HasPrefix(filePath, pattern) || strings.Contains(filePath, "/"+pattern)
	case strings.Contains(pattern, "/"):
		ok, _ := path.Match(pattern, filePath)
		return ok
	default:
		ok, _ := path.Match(pattern, path.Base(filePath))
		return ok
	}
}

// AddComment adds the comment. The target is the same as Comment. The lines are the lines of the file or the
// description at the latest round.
func (r *Review) AddComment(ctx context.Context, repo Repository, changeID, path string, side Side, startLine, endLine int, body, author string, now time.Time) (*Comment, error) {
	if body == "" {
		return nil, xerrors.WithMessage(ErrInvalidArgument, "body is required")
	}
	if r.State == StateApproved || r.State == StateDiscarded {
		return nil, xerrors.WithMessagef(ErrInvalidState, "the review is %s", r.State)
	}
	c := &Comment{ChangeID: changeID, Round: r.LatestRound(), Body: body, Author: author, CreatedAt: now, UpdatedAt: now}
	if changeID != "" {
		commit, ok := r.Commit(r.LatestRound(), changeID)
		if !ok {
			return nil, xerrors.WithMessagef(ErrNotFound, "change %s", changeID)
		}
		c.Path = path
		if startLine != 0 {
			if side != SideNew && (side != SideOld || path == "") {
				return nil, xerrors.WithMessagef(ErrInvalidArgument, "side %q", side)
			}
			if startLine < 1 || endLine < startLine {
				return nil, xerrors.WithMessagef(ErrInvalidArgument, "lines %d-%d", startLine, endLine)
			}
			lines, err := targetLines(ctx, repo, commit, path, side)
			if err != nil {
				return nil, err
			}
			if endLine > len(lines) {
				return nil, xerrors.WithMessagef(ErrInvalidArgument, "the target has %d lines", len(lines))
			}
			c.Side, c.StartLine, c.EndLine = side, startLine, endLine
			c.Anchor = strings.Join(lines[startLine-1:endLine], "\n")
		}
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}
	c.ID = id
	r.Comments = append(r.Comments, c)
	r.UpdatedAt = now
	return c, nil
}

// targetLines returns the lines of the file of the side of the commit, or the lines of the description of the commit if
// path is empty.
func targetLines(ctx context.Context, repo Repository, commit, path string, side Side) ([]string, error) {
	if path == "" {
		c, err := repo.Show(ctx, commit)
		if err != nil {
			return nil, err
		}
		return splitLines([]byte(c.FullDescription)), nil
	}
	content, err := repo.File(ctx, revision(commit, side), path)
	if err != nil {
		return nil, err
	}
	return splitLines(content), nil
}

// revision returns the revision of the file of the side of the change.
func revision(commit string, side Side) string {
	if side == SideOld {
		return commit + "-"
	}
	return commit
}

// Comment returns the comment.
func (r *Review) Comment(id string) (*Comment, error) {
	for _, v := range r.Comments {
		if v.ID == id {
			return v, nil
		}
	}
	return nil, xerrors.WithMessagef(ErrNotFound, "comment %s", id)
}

func (r *Review) EditComment(id, body string, now time.Time) (*Comment, error) {
	if body == "" {
		return nil, xerrors.WithMessage(ErrInvalidArgument, "body is required")
	}
	c, err := r.Comment(id)
	if err != nil {
		return nil, err
	}
	c.Body = body
	c.UpdatedAt = now
	r.UpdatedAt = now
	return c, nil
}

func (r *Review) DeleteComment(id string, now time.Time) error {
	if _, err := r.Comment(id); err != nil {
		return err
	}
	r.Comments = slices.DeleteFunc(r.Comments, func(c *Comment) bool { return c.ID == id })
	r.UpdatedAt = now
	return nil
}

func (r *Review) Reply(commentID, body, author string, now time.Time) (*Comment, error) {
	if body == "" {
		return nil, xerrors.WithMessage(ErrInvalidArgument, "body is required")
	}
	c, err := r.Comment(commentID)
	if err != nil {
		return nil, err
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}
	c.Replies = append(c.Replies, &Reply{ID: id, Body: body, Author: author, CreatedAt: now})
	c.UpdatedAt = now
	r.UpdatedAt = now
	return c, nil
}

func (r *Review) Resolve(commentID string, resolved bool, now time.Time) (*Comment, error) {
	c, err := r.Comment(commentID)
	if err != nil {
		return nil, err
	}
	c.Resolved = resolved
	c.UpdatedAt = now
	r.UpdatedAt = now
	return c, nil
}

// Unresolved reports whether the review has the unresolved comments.
func (r *Review) Unresolved() bool {
	return slices.ContainsFunc(r.Comments, func(c *Comment) bool { return !c.Resolved })
}

// Finish finishes the round. The review is approved if all comments are resolved. Otherwise the agent addresses the
// comments and starts the next round.
func (r *Review) Finish(now time.Time) error {
	if r.State != StateOpen {
		return xerrors.WithMessagef(ErrInvalidState, "the review is %s", r.State)
	}
	if r.Unresolved() {
		r.State = StateFinished
	} else {
		r.State = StateApproved
	}
	r.UpdatedAt = now
	return nil
}

// Discard ends the review without the approval. The comments and the rounds are kept, and the review can't be
// finished or start the next round anymore.
func (r *Review) Discard(now time.Time) error {
	if r.State != StateOpen && r.State != StateFinished {
		return xerrors.WithMessagef(ErrInvalidState, "the review is %s", r.State)
	}
	r.State = StateDiscarded
	r.UpdatedAt = now
	return nil
}

// Realign moves the comment to the lines of the anchor in lines. The lines at the same position are preferred and the
// nearest lines are used if the anchor appears multiple times. If the anchor is not found, the comment is drifted and
// stays at the same position. lines is nil if the file doesn't exist.
func (c *Comment) Realign(lines []string) {
	if c.StartLine == 0 {
		return
	}
	anchor := strings.Split(c.Anchor, "\n")
	match := func(start int) bool {
		return start >= 1 && start+len(anchor)-1 <= len(lines) && slices.Equal(lines[start-1:start-1+len(anchor)], anchor)
	}
	if match(c.StartLine) {
		c.Drifted = false
		return
	}
	found := 0
	for i := 1; i+len(anchor)-1 <= len(lines); i++ {
		if match(i) && (found == 0 || abs(i-c.StartLine) < abs(found-c.StartLine)) {
			found = i
		}
	}
	if found == 0 {
		c.Drifted = true
		return
	}
	c.StartLine, c.EndLine = found, found+len(anchor)-1
	c.Drifted = false
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// splitLines splits the content into the lines without the newlines.
func splitLines(content []byte) []string {
	s := strings.TrimSuffix(string(content), "\n")
	if s == "" {
		return []string{}
	}
	return strings.Split(s, "\n")
}

func newID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", xerrors.WithStack(err)
	}
	return hex.EncodeToString(buf), nil
}

// Approvals finds the commits approved in the reviews. The comparisons of the commits are cached because the commit ID
// is the hash of the content.
type Approvals struct {
	mu   sync.Mutex
	same map[[2]string]bool
}

func NewApprovals() *Approvals {
	return &Approvals{same: make(map[[2]string]bool)}
}

// Commits returns the commit IDs of the changes approved in the reviews. The commit is approved if the change of it is
// the same as the commit in the latest round of the approved review including the description. The commit rebased after
// the approval is approved because the difference of the parents is ignored.
func (a *Approvals) Commits(ctx context.Context, repo Repository, reviews []*Review) ([]string, error) {
	approved := make(map[string][]string)
	for _, r := range reviews {
		if r.State != StateApproved || len(r.Rounds) == 0 {
			continue
		}
		for change, commit := range r.Rounds[len(r.Rounds)-1].Commits {
			approved[change] = append(approved[change], commit)
		}
	}
	if len(approved) == 0 {
		return nil, nil
	}
	revsets := make([]string, 0, len(approved))
	for change := range approved {
		if !changeIDPattern.MatchString(change) {
			return nil, xerrors.WithMessagef(ErrInvalidArgument, "%q is not a change ID", change)
		}
		revsets = append(revsets, "change_id("+change+")")
	}
	slices.Sort(revsets)
	rows, err := repo.Log(ctx, strings.Join(revsets, "|"), 0)
	if err != nil {
		return nil, err
	}

	var commits []string
	for _, row := range rows {
		for _, v := range approved[row.Commit.ChangeID] {
			same, err := a.sameChange(ctx, repo, v, row.Commit.CommitID)
			if err != nil {
				return nil, err
			}
			if same {
				commits = append(commits, row.Commit.CommitID)
				break
			}
		}
	}
	slices.Sort(commits)
	return commits, nil
}

func (a *Approvals) sameChange(ctx context.Context, repo Repository, from, to string) (bool, error) {
	if from == to {
		return true, nil
	}
	key := [2]string{from, to}
	a.mu.Lock()
	same, ok := a.same[key]
	a.mu.Unlock()
	if ok {
		return same, nil
	}
	files, err := repo.Interdiff(ctx, from, to)
	if err != nil {
		return false, err
	}
	same = len(files) == 0
	a.mu.Lock()
	a.same[key] = same
	a.mu.Unlock()
	return same, nil
}

var idPattern = regexp.MustCompile(`^[0-9a-f]+$`)

// Store saves the reviews in the directory. A review is a file named <id>.json. The changes are serialized.
type Store struct {
	dir string
	mu  sync.Mutex
}

// DefaultDir returns ~/.config/codexel/reviews.
func DefaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", xerrors.WithStack(err)
	}
	return filepath.Join(home, ".config", "codexel", "reviews"), nil
}

func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

func (s *Store) Get(id string) (*Review, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.get(id)
}

// List returns the reviews of the repository. All reviews are returned if repository is empty.
func (s *Store) List(repository string) ([]*Review, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	var reviews []*Review
	for _, v := range entries {
		id, ok := strings.CutSuffix(v.Name(), ".json")
		if !ok || !idPattern.MatchString(id) {
			continue
		}
		r, err := s.get(id)
		if err != nil {
			return nil, err
		}
		if repository == "" || r.Repository == repository {
			reviews = append(reviews, r)
		}
	}
	slices.SortFunc(reviews, func(a, b *Review) int { return b.CreatedAt.Compare(a.CreatedAt) })
	return reviews, nil
}

func (s *Store) Create(r *Review) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.write(r)
}

// Update applies fn to the review and writes it. The review is not written if fn returns the error.
func (s *Store) Update(id string, fn func(*Review) error) (*Review, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, err := s.get(id)
	if err != nil {
		return nil, err
	}
	if err := fn(r); err != nil {
		return nil, err
	}
	if err := s.write(r); err != nil {
		return nil, err
	}
	return r, nil
}

func (s *Store) get(id string) (*Review, error) {
	if !idPattern.MatchString(id) {
		return nil, xerrors.WithMessagef(ErrNotFound, "review %s", id)
	}
	buf, err := os.ReadFile(filepath.Join(s.dir, id+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, xerrors.WithMessagef(ErrNotFound, "review %s", id)
	}
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	r := &Review{}
	if err := json.Unmarshal(buf, r); err != nil {
		return nil, xerrors.WithMessagef(err, "failed to parse the review %s", id)
	}
	return r, nil
}

func (s *Store) write(r *Review) error {
	if !idPattern.MatchString(r.ID) {
		return xerrors.WithMessagef(ErrInvalidArgument, "id %q", r.ID)
	}
	buf, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return xerrors.WithStack(err)
	}
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return xerrors.WithStack(err)
	}
	tmp, err := os.CreateTemp(s.dir, ".review-*.json")
	if err != nil {
		return xerrors.WithStack(err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(buf, '\n')); err != nil {
		tmp.Close()
		return xerrors.WithStack(err)
	}
	if err := tmp.Close(); err != nil {
		return xerrors.WithStack(err)
	}
	if err := os.Rename(tmp.Name(), filepath.Join(s.dir, r.ID+".json")); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}

// MessageDiff returns the diff of the descriptions of the commits. The diff is a hunk which has all lines because the
// description is short. The lines are the context if the descriptions are the same.
func MessageDiff(oldMessage, newMessage string) *jujutsu.FileDiff {
	dmp := diffmatchpatch.New()
	a, b, lines := dmp.DiffLinesToRunes(withNewline(oldMessage), withNewline(newMessage))
	f := &jujutsu.FileDiff{Status: jujutsu.FileStatusModified}
	h := &jujutsu.Hunk{}
	oldLine, newLine := 0, 0
	for _, d := range dmp.DiffCharsToLines(dmp.DiffMainRunes(a, b, false), lines) {
		for _, text := range splitLines([]byte(d.Text)) {
			l := &jujutsu.DiffLine{Text: text}
			switch d.Type {
			case diffmatchpatch.DiffEqual:
				oldLine++
				newLine++
				l.Kind, l.OldLine, l.NewLine = jujutsu.DiffLineContext, oldLine, newLine
			case diffmatchpatch.DiffDelete:
				oldLine++
				l.Kind, l.OldLine = jujutsu.DiffLineRemoved, oldLine
				f.Removed++
			case diffmatchpatch.DiffInsert:
				newLine++
				l.Kind, l.NewLine = jujutsu.DiffLineAdded, newLine
				f.Added++
			}
			h.Lines = append(h.Lines, l)
		}
	}
	if len(h.Lines) == 0 {
		return f
	}
	h.OldStart, h.OldLines, h.NewStart, h.NewLines = min(1, oldLine), oldLine, min(1, newLine), newLine
	f.Hunks = []*jujutsu.Hunk{h}
	return f
}

// withNewline terminates the last line by the newline so that the last lines are compared in the same form.
func withNewline(s string) string {
	if s == "" || strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}
