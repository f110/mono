package jujutsu

import (
	"testing"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestParseBookmarks(t *testing.T) {
	// The output of jj bookmark list by bookmarksArgs.
	out := `{"name":"main","remote":"","tracked":false,"conflict":false,"synced":false,"target":"abc","ahead":0,"behind":0,"description":"local"}
{"name":"main","remote":"git","tracked":true,"conflict":false,"synced":true,"target":"abc","ahead":0,"behind":0,"description":"local"}
{"name":"main","remote":"origin","tracked":true,"conflict":false,"synced":false,"target":"def","ahead":2,"behind":1,"description":"remote"}
`
	bookmarks, err := parseBookmarks([]byte(out))
	assertion.MustNoError(t, err)
	assertion.MustLen(t, bookmarks, 2)
	assertion.Equal(t, *bookmarks[0], Bookmark{Name: "main", Target: "abc", Description: "local"})
	assertion.Equal(t, *bookmarks[1], Bookmark{Name: "main", Remote: "origin", Tracked: true, Target: "def", Ahead: 2, Behind: 1, Description: "remote"})
}

func TestBookmarkArgs(t *testing.T) {
	// The names are exact and the values are not the options.
	assertion.Equal(t, bookmarkSetArgs("--foo", "--config=x"), []string{"bookmark", "set", "--revision=--config=x", "--", "--foo"})
	assertion.Equal(t, bookmarkMoveArgs("feat-*", "abc"), []string{"bookmark", "move", "--to=abc", "--allow-backwards", "--", "exact:feat-*"})
	assertion.Equal(t, bookmarkDeleteArgs("feat-*"), []string{"bookmark", "delete", "--", "exact:feat-*"})
	assertion.Equal(t, bookmarkTrackArgs("main", "origin"), []string{"bookmark", "track", "--remote=exact:origin", "--", "exact:main"})
	assertion.Equal(t, gitPushArgs(""), []string{"git", "push"})
	assertion.Equal(t, gitPushArgs("feat-*"), []string{"git", "push", "--bookmark=exact:feat-*"})
}
