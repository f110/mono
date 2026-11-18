package review

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.f110.dev/mono/go/jujutsu"
	"go.f110.dev/mono/go/testing/assertion"
)

// fakeRepository returns the commits of the changes in commits and the files in files keyed by "<rev>:<path>".
type fakeRepository struct {
	commits map[string]string
	files   map[string]string
	// diffs is the changed files of the revision and the range "<from>..<to>".
	diffs map[string][]string
	// descriptions is the whole description of the commit.
	descriptions map[string]string
	snapshots    int
	revsets      []string
}

func (f *fakeRepository) Root() string { return "/repo" }

func (f *fakeRepository) Snapshot(_ context.Context) error {
	f.snapshots++
	return nil
}

func (f *fakeRepository) Log(_ context.Context, revset string, _ int) ([]*jujutsu.LogRow, error) {
	f.revsets = append(f.revsets, revset)
	var rows []*jujutsu.LogRow
	for change, commit := range f.commits {
		if strings.Contains(revset, "change_id("+change+")") {
			rows = append(rows, &jujutsu.LogRow{Commit: &jujutsu.Commit{ChangeID: change, CommitID: commit}})
		}
	}
	return rows, nil
}

func (f *fakeRepository) Show(_ context.Context, rev string) (*jujutsu.Commit, error) {
	return &jujutsu.Commit{CommitID: rev, FullDescription: f.descriptions[rev]}, nil
}

func (f *fakeRepository) File(_ context.Context, rev, path string) ([]byte, error) {
	v, ok := f.files[rev+":"+path]
	if !ok {
		return nil, &jujutsu.Error{Args: []string{"file"}, Stderr: "No such path"}
	}
	return []byte(v), nil
}

func (f *fakeRepository) Diff(_ context.Context, rev string) ([]*jujutsu.FileDiff, error) {
	return f.fileDiffs(rev), nil
}

func (f *fakeRepository) DiffRange(_ context.Context, from, to string) ([]*jujutsu.FileDiff, error) {
	return f.fileDiffs(from + ".." + to), nil
}

func (f *fakeRepository) fileDiffs(key string) []*jujutsu.FileDiff {
	var files []*jujutsu.FileDiff
	for _, v := range f.diffs[key] {
		files = append(files, &jujutsu.FileDiff{Path: v})
	}
	return files
}

func TestReviewRounds(t *testing.T) {
	ctx := t.Context()
	now := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		commits: map[string]string{"kkkk": "c1", "llll": "d1"},
		files: map[string]string{
			"c1:a.go":  "package a\n\nfunc A() {\n}\n",
			"c1-:a.go": "package a\n",
			"d1:b.go":  "package b\n",
		},
	}

	r, err := New(ctx, repo, []string{"kkkk", "llll"}, nil, now)
	assertion.MustNoError(t, err)
	assertion.Equal(t, r.Repository, "/repo")
	assertion.Equal(t, r.State, StateOpen)
	assertion.Equal(t, r.Rounds, []*Round{{Commits: map[string]string{"kkkk": "c1", "llll": "d1"}, CreatedAt: now}})
	assertion.Equal(t, repo.snapshots, 1)
	assertion.Equal(t, repo.revsets, []string{"change_id(kkkk)|change_id(llll)"})

	moved, err := r.AddComment(ctx, repo, "kkkk", "a.go", SideNew, 3, 4, "rename", AuthorUser, now)
	assertion.MustNoError(t, err)
	assertion.Equal(t, moved.Anchor, "func A() {\n}")
	assertion.Equal(t, moved.Round, 1)
	old, err := r.AddComment(ctx, repo, "kkkk", "a.go", SideOld, 1, 1, "old side", AuthorUser, now)
	assertion.MustNoError(t, err)
	assertion.Equal(t, old.Anchor, "package a")
	gone, err := r.AddComment(ctx, repo, "llll", "b.go", SideNew, 1, 1, "abandoned", AuthorUser, now)
	assertion.MustNoError(t, err)
	file, err := r.AddComment(ctx, repo, "kkkk", "a.go", "", 0, 0, "file", AuthorUser, now)
	assertion.MustNoError(t, err)
	whole, err := r.AddComment(ctx, repo, "", "", "", 0, 0, "overall", AuthorUser, now)
	assertion.MustNoError(t, err)
	assertion.Equal(t, whole.Path, "")

	_, err = r.AddComment(ctx, repo, "kkkk", "a.go", SideNew, 4, 5, "out of range", AuthorUser, now)
	assertion.MustError(t, err)
	_, err = r.AddComment(ctx, repo, "mmmm", "a.go", SideNew, 1, 1, "unknown change", AuthorUser, now)
	assertion.MustError(t, err)

	assertion.MustNoError(t, r.Finish(now))
	assertion.MustError(t, r.Finish(now))
	_, err = r.Reply(moved.ID, "done", AuthorAgent, now)
	assertion.MustNoError(t, err)

	repo.commits = map[string]string{"kkkk": "c2"}
	repo.files["c2:a.go"] = "package a\n\nimport \"fmt\"\n\nfunc A() {\n}\n"
	repo.files["c2-:a.go"] = "package a\n"
	later := now.Add(time.Hour)
	assertion.MustNoError(t, r.NextRound(ctx, repo, nil, later))
	assertion.Equal(t, r.State, StateOpen)
	assertion.Equal(t, r.LatestRound(), 2)
	commit, ok := r.Commit(2, "kkkk")
	assertion.True(t, ok)
	assertion.Equal(t, commit, "c2")
	_, ok = r.Commit(2, "llll")
	assertion.False(t, ok)

	assertion.Equal(t, [2]int{moved.StartLine, moved.EndLine}, [2]int{5, 6})
	assertion.False(t, moved.Drifted)
	assertion.Equal(t, moved.Replies[0].Author, AuthorAgent)
	assertion.Equal(t, [2]int{old.StartLine, old.EndLine}, [2]int{1, 1})
	assertion.False(t, old.Drifted)
	assertion.True(t, gone.Drifted)
	assertion.False(t, file.Drifted)

	assertion.MustNoError(t, r.Approve(later))
	assertion.MustError(t, r.NextRound(ctx, repo, nil, later))
	_, err = r.AddComment(ctx, repo, "", "", "", 0, 0, "after approval", AuthorUser, later)
	assertion.MustError(t, err)
}

func TestNewRejectsInvalidChange(t *testing.T) {
	repo := &fakeRepository{commits: map[string]string{"kkkk": "c1"}}

	_, err := New(t.Context(), repo, []string{"kkkk|all()"}, nil, time.Now())
	assertion.MustError(t, err)
	assertion.Len(t, repo.revsets, 0)

	_, err = New(t.Context(), repo, []string{"llll"}, nil, time.Now())
	assertion.MustError(t, err)
}

func TestResolveCommits(t *testing.T) {
	rows := []*jujutsu.LogRow{
		{Commit: &jujutsu.Commit{ChangeID: "kkkkkkkk", CommitID: "c1"}},
		{Commit: &jujutsu.Commit{ChangeID: "llllllll", CommitID: "d1"}},
	}
	commits, err := resolveCommits([]string{"kkkk", "llllllll"}, rows)
	assertion.MustNoError(t, err)
	assertion.Equal(t, commits, map[string]string{"kkkk": "c1", "llllllll": "d1"})

	divergent := append(rows, &jujutsu.LogRow{Commit: &jujutsu.Commit{ChangeID: "kkkkkkkk", CommitID: "c2"}})
	_, err = resolveCommits([]string{"kkkk"}, divergent)
	assertion.MustError(t, err)
}

func TestRealign(t *testing.T) {
	cases := []struct {
		name      string
		start     int
		end       int
		anchor    string
		lines     []string
		wantStart int
		wantEnd   int
		drifted   bool
	}{
		{name: "same position", start: 2, end: 2, anchor: "b", lines: []string{"a", "b", "c"}, wantStart: 2, wantEnd: 2},
		{name: "moved", start: 1, end: 2, anchor: "a\nb", lines: []string{"x", "y", "a", "b"}, wantStart: 3, wantEnd: 4},
		{name: "nearest", start: 5, end: 5, anchor: "}", lines: []string{"}", "x", "x", "x", "x", "x", "}"}, wantStart: 7, wantEnd: 7},
		{name: "not found", start: 2, end: 2, anchor: "b", lines: []string{"a", "c"}, wantStart: 2, wantEnd: 2, drifted: true},
		{name: "file deleted", start: 1, end: 1, anchor: "a", lines: nil, wantStart: 1, wantEnd: 1, drifted: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Comment{StartLine: tc.start, EndLine: tc.end, Anchor: tc.anchor}
			c.Realign(tc.lines)
			assertion.Equal(t, [2]int{c.StartLine, c.EndLine}, [2]int{tc.wantStart, tc.wantEnd})
			assertion.Equal(t, c.Drifted, tc.drifted)
		})
	}
}

func TestCommitMessageComments(t *testing.T) {
	ctx := t.Context()
	now := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		commits:      map[string]string{"kkkk": "c1"},
		descriptions: map[string]string{"c1": "Add A\n\nA is the first.\n"},
	}
	r, err := New(ctx, repo, []string{"kkkk"}, nil, now)
	assertion.MustNoError(t, err)

	line, err := r.AddComment(ctx, repo, "kkkk", "", SideNew, 3, 3, "why", AuthorUser, now)
	assertion.MustNoError(t, err)
	assertion.Equal(t, line.Path, "")
	assertion.Equal(t, line.Anchor, "A is the first.")
	whole, err := r.AddComment(ctx, repo, "kkkk", "", "", 0, 0, "subject", AuthorUser, now)
	assertion.MustNoError(t, err)
	assertion.Equal(t, whole.Anchor, "")
	_, err = r.AddComment(ctx, repo, "kkkk", "", SideNew, 4, 4, "out of range", AuthorUser, now)
	assertion.MustError(t, err)
	_, err = r.AddComment(ctx, repo, "kkkk", "", SideOld, 1, 1, "parent", AuthorUser, now)
	assertion.MustError(t, err)

	repo.commits = map[string]string{"kkkk": "c2"}
	repo.descriptions["c2"] = "Add A\n\nThe details.\n\nA is the first.\n"
	assertion.MustNoError(t, r.NextRound(ctx, repo, nil, now))
	assertion.Equal(t, [2]int{line.StartLine, line.EndLine}, [2]int{5, 5})
	assertion.False(t, line.Drifted)

	repo.commits = map[string]string{"kkkk": "c3"}
	repo.descriptions["c3"] = "Add A\n"
	assertion.MustNoError(t, r.NextRound(ctx, repo, nil, now))
	assertion.True(t, line.Drifted)
}

func TestMessageDiff(t *testing.T) {
	line := func(kind jujutsu.DiffLineKind, text string, oldLine, newLine int) *jujutsu.DiffLine {
		return &jujutsu.DiffLine{Kind: kind, Text: text, OldLine: oldLine, NewLine: newLine}
	}

	same := MessageDiff("Add A\n\nbody\n", "Add A\n\nbody\n")
	assertion.Equal(t, same.Added, 0)
	assertion.Equal(t, same.Removed, 0)
	assertion.MustLen(t, same.Hunks, 1)
	assertion.Equal(t, same.Hunks[0].Lines, []*jujutsu.DiffLine{
		line(jujutsu.DiffLineContext, "Add A", 1, 1),
		line(jujutsu.DiffLineContext, "", 2, 2),
		line(jujutsu.DiffLineContext, "body", 3, 3),
	})

	changed := MessageDiff("Add A\n\nbody\n", "Add A\n\nnew body\nmore")
	assertion.Equal(t, changed.Added, 2)
	assertion.Equal(t, changed.Removed, 1)
	assertion.MustLen(t, changed.Hunks, 1)
	h := changed.Hunks[0]
	assertion.Equal(t, [4]int{h.OldStart, h.OldLines, h.NewStart, h.NewLines}, [4]int{1, 3, 1, 4})
	assertion.Equal(t, h.Lines, []*jujutsu.DiffLine{
		line(jujutsu.DiffLineContext, "Add A", 1, 1),
		line(jujutsu.DiffLineContext, "", 2, 2),
		line(jujutsu.DiffLineRemoved, "body", 3, 0),
		line(jujutsu.DiffLineAdded, "new body", 0, 3),
		line(jujutsu.DiffLineAdded, "more", 0, 4),
	})

	assertion.Len(t, MessageDiff("", "").Hunks, 0)
}

func TestStore(t *testing.T) {
	s := NewStore(t.TempDir())
	now := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)

	_, err := s.Get("0123")
	assertion.MustError(t, err)
	_, err = s.Get("../config")
	assertion.MustError(t, err)
	reviews, err := s.List("")
	assertion.MustNoError(t, err)
	assertion.Len(t, reviews, 0)

	assertion.MustNoError(t, s.Create(&Review{ID: "01", Repository: "/a", State: StateOpen, CreatedAt: now}))
	assertion.MustNoError(t, s.Create(&Review{ID: "02", Repository: "/b", State: StateOpen, CreatedAt: now.Add(time.Hour)}))

	updated, err := s.Update("01", func(r *Review) error { return r.Finish(now) })
	assertion.MustNoError(t, err)
	assertion.Equal(t, updated.State, StateFinished)
	_, err = s.Update("01", func(r *Review) error { return r.Finish(now) })
	assertion.MustError(t, err)

	got, err := s.Get("01")
	assertion.MustNoError(t, err)
	assertion.Equal(t, got.State, StateFinished)

	reviews, err = s.List("")
	assertion.MustNoError(t, err)
	assertion.Len(t, reviews, 2)
	assertion.Equal(t, reviews[0].ID, "02")
	reviews, err = s.List("/a")
	assertion.MustNoError(t, err)
	assertion.Len(t, reviews, 1)
	assertion.Equal(t, reviews[0].ID, "01")
}

func TestViewed(t *testing.T) {
	ctx := t.Context()
	now := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		commits: map[string]string{"kkkk": "c1"},
		diffs:   map[string][]string{"c1": {"a.go", "b.go", "go.sum"}},
	}
	patterns := []string{"*.sum", "gen/"}

	r, err := New(ctx, repo, []string{"kkkk"}, patterns, now)
	assertion.MustNoError(t, err)
	assertion.Equal(t, r.Viewed, []*ViewedFile{{ChangeID: "kkkk", Path: "go.sum", Viewed: true}})

	assertion.MustNoError(t, r.SetViewed("kkkk", "a.go", true, now))
	assertion.MustNoError(t, r.SetViewed("kkkk", "b.go", true, now))
	assertion.MustNoError(t, r.SetViewed("kkkk", "go.sum", false, now))
	assertion.MustError(t, r.SetViewed("llll", "a.go", true, now))
	assertion.MustError(t, r.SetViewed("kkkk", "", true, now))

	repo.commits = map[string]string{"kkkk": "c2"}
	repo.diffs["c2"] = []string{"a.go", "b.go", "go.sum", "gen/x.go"}
	repo.diffs["c1..c2"] = []string{"b.go", "go.sum", "gen/x.go"}
	assertion.MustNoError(t, r.NextRound(ctx, repo, patterns, now))
	assertion.Equal(t, r.Viewed, []*ViewedFile{
		{ChangeID: "kkkk", Path: "a.go", Viewed: true},
		{ChangeID: "kkkk", Path: "go.sum", Viewed: true},
		{ChangeID: "kkkk", Path: "gen/x.go", Viewed: true},
	})
}

func TestMatchPattern(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		match   bool
	}{
		{pattern: "*.lock", path: "a/b.lock", match: true},
		{pattern: "*.lock", path: "a/b.lock.go", match: false},
		{pattern: "gen/", path: "gen/a.go", match: true},
		{pattern: "gen/", path: "a/gen/b.go", match: true},
		{pattern: "gen/", path: "agen/b.go", match: false},
		{pattern: "go/*.pb.go", path: "go/a.pb.go", match: true},
		{pattern: "go/*.pb.go", path: "go/x/a.pb.go", match: false},
		{pattern: "PLAN.md", path: "docs/PLAN.md", match: true},
		{pattern: "docs/PLAN.md", path: "docs/PLAN.md", match: true},
		{pattern: "PLAN.md", path: "PLAN.mdx", match: false},
	}
	for _, tc := range cases {
		t.Run(tc.pattern+" "+tc.path, func(t *testing.T) {
			assertion.Equal(t, matchPattern(tc.path, tc.pattern), tc.match)
		})
	}
}
