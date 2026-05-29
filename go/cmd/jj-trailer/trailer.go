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

// appendTrailer appends "key: value" to desc unconditionally. When desc's last
// paragraph already looks like a trailer block (every line matches
// "Key: value"), the new trailer is appended to it; otherwise a blank line is
// inserted before the trailer to start a new block.
func appendTrailer(desc, key, value string) string {
	body := strings.TrimRight(desc, "\n")
	newTrailer := fmt.Sprintf("%s: %s", key, value)
	if body == "" {
		return newTrailer
	}

	paragraphs := strings.Split(body, "\n\n")
	last := paragraphs[len(paragraphs)-1]
	isTrailerBlock := true
	for _, line := range strings.Split(last, "\n") {
		if !trailerLineRe.MatchString(line) {
			isTrailerBlock = false
			break
		}
	}

	if isTrailerBlock {
		return body + "\n" + newTrailer
	}
	return body + "\n\n" + newTrailer
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
