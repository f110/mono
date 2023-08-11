package nixery

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type GitSource struct {
	Repository string
}

// Regex to determine whether a git reference is a commit hash or
// something else (branch/tag).
//
// Used to check whether a git reference is cacheable, and to pass the
// correct git structure to Nix.
//
// Note: If a user creates a branch or tag with the name of a commit
// and references it intentionally, this heuristic will fail.
var commitRegex = regexp.MustCompile(`^[0-9a-f]{40}$`)

func (g *GitSource) Render(tag string) (string, string) {
	args := map[string]string{
		"url": g.Repository,
	}

	// The 'git' source requires a tag to be present. If the user
	// has not specified one, it is assumed that the default
	// 'master' branch should be used.
	if tag == "latest" || tag == "" {
		tag = "master"
	}

	if commitRegex.MatchString(tag) {
		args["rev"] = tag
	} else {
		args["ref"] = tag
	}

	j, _ := json.Marshal(args)

	return "git", string(j)
}

func (g *GitSource) CacheKey(pkgs []string, tag string) string {
	// Only full commit hashes can be used for caching, as
	// everything else is potentially a moving target.
	if !commitRegex.MatchString(tag) {
		return ""
	}

	unhashed := strings.Join(pkgs, "") + tag
	hashed := fmt.Sprintf("%x", sha1.Sum([]byte(unhashed)))

	return hashed
}
