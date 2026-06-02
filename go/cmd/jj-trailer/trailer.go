package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.f110.dev/xerrors"
)

var trailerLineRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]*:\s`)

// hasTrailer reports whether desc already contains a trailer with the given
// key (case-insensitive).
func hasTrailer(desc, key string) bool {
	keyPrefix := strings.ToLower(key) + ":"
	for _, line := range strings.Split(desc, "\n") {
		if strings.HasPrefix(strings.ToLower(line), keyPrefix) {
			return true
		}
	}
	return false
}

// appendTrailer appends "key: value" to desc unconditionally. The contiguous
// run of "Key: value" lines at the end of desc is treated as the trailer
// block (whether or not it was separated from the body by a blank line); the
// new trailer joins that block, and exactly one blank line is guaranteed
// between the body and the block.
func appendTrailer(desc, key, value string) string {
	body := strings.TrimRight(desc, "\n")
	newTrailer := fmt.Sprintf("%s: %s", key, value)
	if body == "" {
		return newTrailer
	}

	lines := strings.Split(body, "\n")
	trailerStart := len(lines)
	for i := len(lines) - 1; i >= 1; i-- {
		if lines[i] == "" || !trailerLineRe.MatchString(lines[i]) {
			break
		}
		trailerStart = i
	}

	if trailerStart == len(lines) {
		return body + "\n\n" + newTrailer
	}

	bodyLines := lines[:trailerStart]
	for len(bodyLines) > 0 && bodyLines[len(bodyLines)-1] == "" {
		bodyLines = bodyLines[:len(bodyLines)-1]
	}
	trailerLines := append(lines[trailerStart:], newTrailer)
	return strings.Join(bodyLines, "\n") + "\n\n" + strings.Join(trailerLines, "\n")
}

// addTrailer is like appendTrailer but skips when a line with the same key
// (case-insensitive) already exists anywhere in desc. The returned bool
// reports whether desc was changed.
func addTrailer(desc, key, value string) (string, bool) {
	if hasTrailer(desc, key) {
		return desc, false
	}
	return appendTrailer(desc, key, value), true
}

const modelsCacheFile = "models.json"

func defaultCachePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", xerrors.WithStack(err)
	}
	return filepath.Join(dir, "jj-trailer", modelsCacheFile), nil
}

// loadModelsCache reads cached models from path. Returns (nil, nil) when the
// cache file does not exist, so callers can treat absence as a soft miss.
func loadModelsCache(path string) ([]anthropicModel, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, xerrors.WithStack(err)
	}
	var models []anthropicModel
	if err := json.Unmarshal(buf, &models); err != nil {
		return nil, xerrors.WithStack(err)
	}
	return models, nil
}

func writeModelsCache(path string, models []anthropicModel) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return xerrors.WithStack(err)
	}
	tmp, err := os.CreateTemp(dir, ".models-*.json")
	if err != nil {
		return xerrors.WithStack(err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := json.NewEncoder(tmp).Encode(models); err != nil {
		tmp.Close()
		return xerrors.WithStack(err)
	}
	if err := tmp.Close(); err != nil {
		return xerrors.WithStack(err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return xerrors.WithStack(err)
	}
	return nil
}
