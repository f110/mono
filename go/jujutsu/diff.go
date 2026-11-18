package jujutsu

import (
	"context"
	"strconv"
	"strings"

	"go.f110.dev/xerrors"
)

type FileStatus int

const (
	FileStatusModified FileStatus = iota
	FileStatusAdded
	FileStatusDeleted
	FileStatusRenamed
	FileStatusCopied
)

type DiffLineKind int

const (
	DiffLineContext DiffLineKind = iota
	DiffLineAdded
	DiffLineRemoved
)

type DiffLine struct {
	Kind DiffLineKind
	Text string
	// OldLine and NewLine are the line numbers in the old and the new file. 0 means the line doesn't exist in the file.
	OldLine int
	NewLine int
	// NoNewline is true if the line is the end of the file without the newline.
	NoNewline bool
}

type Hunk struct {
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	// Section is the text after the range of the header.
	Section string
	Lines   []*DiffLine
}

type FileDiff struct {
	Path string
	// OldPath is the path before the rename or the copy.
	OldPath string
	Status  FileStatus
	Binary  bool
	Hunks   []*Hunk
	Added   int
	Removed int
}

// diffArgs returns the arguments of jj diff in the git format. The prefixes of the paths are fixed because the parser
// depends on them.
func diffArgs(rev string) []string {
	return []string{"diff", "--git", "--color", "never", "--ignore-working-copy", "--config", "diff.git.show-path-prefix=true", "--revisions=" + rev}
}

func (r *Repository) Diff(ctx context.Context, rev string) ([]*FileDiff, error) {
	out, err := run(ctx, r.root, diffArgs(rev)...)
	if err != nil {
		return nil, err
	}
	return parseGitDiff(string(out))
}

// parseGitDiff parses the diff in the git format.
func parseGitDiff(out string) ([]*FileDiff, error) {
	var files []*FileDiff
	var file *FileDiff
	var hunk *Hunk
	var oldLine, newLine int
	for line := range strings.Lines(out) {
		line = strings.TrimSuffix(line, "\n")
		switch {
		case strings.HasPrefix(line, "diff --git "):
			oldPath, newPath, err := parseDiffHeader(line)
			if err != nil {
				return nil, err
			}
			file = &FileDiff{Path: newPath, OldPath: oldPath}
			files = append(files, file)
			hunk = nil
			continue
		case file == nil:
			continue
		case hunk != nil && (strings.HasPrefix(line, " ") || line == ""):
			hunk.Lines = append(hunk.Lines, &DiffLine{Kind: DiffLineContext, Text: strings.TrimPrefix(line, " "), OldLine: oldLine, NewLine: newLine})
			oldLine++
			newLine++
			continue
		case hunk != nil && strings.HasPrefix(line, "+"):
			hunk.Lines = append(hunk.Lines, &DiffLine{Kind: DiffLineAdded, Text: line[1:], NewLine: newLine})
			newLine++
			file.Added++
			continue
		case hunk != nil && strings.HasPrefix(line, "-"):
			hunk.Lines = append(hunk.Lines, &DiffLine{Kind: DiffLineRemoved, Text: line[1:], OldLine: oldLine})
			oldLine++
			file.Removed++
			continue
		case hunk != nil && strings.HasPrefix(line, `\`):
			if len(hunk.Lines) > 0 {
				hunk.Lines[len(hunk.Lines)-1].NoNewline = true
			}
			continue
		}

		switch {
		case strings.HasPrefix(line, "@@ "):
			h, err := parseHunkHeader(line)
			if err != nil {
				return nil, err
			}
			hunk = h
			file.Hunks = append(file.Hunks, hunk)
			oldLine, newLine = h.OldStart, h.NewStart
		case strings.HasPrefix(line, "new file mode"):
			file.Status = FileStatusAdded
		case strings.HasPrefix(line, "deleted file mode"):
			file.Status = FileStatusDeleted
		case strings.HasPrefix(line, "rename from "):
			file.Status = FileStatusRenamed
		case strings.HasPrefix(line, "copy from "):
			file.Status = FileStatusCopied
		case strings.HasPrefix(line, "Binary files "):
			file.Binary = true
		}
	}
	for _, v := range files {
		if v.Status != FileStatusRenamed && v.Status != FileStatusCopied {
			v.OldPath = ""
		}
	}
	return files, nil
}

// parseDiffHeader parses "diff --git a/<old> b/<new>". The paths may contain the spaces, so the header is split at
// " b/" which makes the old and the new path have the same length when the file is not renamed.
func parseDiffHeader(line string) (string, string, error) {
	rest := strings.TrimPrefix(line, "diff --git ")
	if !strings.HasPrefix(rest, "a/") {
		return "", "", xerrors.Newf("unexpected header of the diff: %s", line)
	}
	if len(rest)%2 == 1 {
		if half := (len(rest) - 1) / 2; rest[half:half+3] == " b/" && rest[2:half] == rest[half+3:] {
			return rest[2:half], rest[half+3:], nil
		}
	}
	i := strings.Index(rest, " b/")
	if i < 0 {
		return "", "", xerrors.Newf("unexpected header of the diff: %s", line)
	}
	return rest[2:i], rest[i+3:], nil
}

// parseHunkHeader parses "@@ -<start>,<lines> +<start>,<lines> @@ <section>". The number of the lines is omitted if it
// is 1.
func parseHunkHeader(line string) (*Hunk, error) {
	body, section, ok := strings.Cut(strings.TrimPrefix(line, "@@ "), " @@")
	if !ok {
		return nil, xerrors.Newf("unexpected header of the hunk: %s", line)
	}
	oldRange, newRange, ok := strings.Cut(body, " ")
	if !ok || !strings.HasPrefix(oldRange, "-") || !strings.HasPrefix(newRange, "+") {
		return nil, xerrors.Newf("unexpected header of the hunk: %s", line)
	}
	oldStart, oldLines, err := parseRange(oldRange[1:])
	if err != nil {
		return nil, err
	}
	newStart, newLines, err := parseRange(newRange[1:])
	if err != nil {
		return nil, err
	}
	return &Hunk{OldStart: oldStart, OldLines: oldLines, NewStart: newStart, NewLines: newLines, Section: strings.TrimSpace(section)}, nil
}

func parseRange(s string) (int, int, error) {
	start, lines, ok := strings.Cut(s, ",")
	if !ok {
		lines = "1"
	}
	a, err := strconv.Atoi(start)
	if err != nil {
		return 0, 0, xerrors.WithStack(err)
	}
	b, err := strconv.Atoi(lines)
	if err != nil {
		return 0, 0, xerrors.WithStack(err)
	}
	return a, b, nil
}
