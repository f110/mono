package main

import (
	"strings"
	"testing"

	"go.f110.dev/mono/go/testing/assertion"
)

func TestParseSeed(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		wantRows int
		check    func(t *testing.T, rows []map[string]any)
	}{
		{
			name: "sequence",
			in: `- id: 1
  url: https://github.com/example/public-app
  private: false
- id: 2
  url: https://github.com/example/private-app
  private: true
`,
			wantRows: 2,
			check: func(t *testing.T, rows []map[string]any) {
				assertion.Equal(t, rows[0]["id"], any(1))
				assertion.Equal(t, rows[0]["url"], any("https://github.com/example/public-app"))
				assertion.Equal(t, rows[0]["private"], any(false))
				assertion.Equal(t, rows[1]["private"], any(true))
			},
		},
		{
			name: "stream",
			in: `id: 1
url: https://github.com/example/public-app
---
id: 2
url: https://github.com/example/private-app
`,
			wantRows: 2,
			check: func(t *testing.T, rows []map[string]any) {
				assertion.Equal(t, rows[0]["id"], any(1))
				assertion.Equal(t, rows[1]["id"], any(2))
			},
		},
		{
			name:     "empty",
			in:       "",
			wantRows: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := parseSeed(strings.NewReader(tt.in))
			assertion.MustNoError(t, err)
			assertion.Equal(t, len(rows), tt.wantRows)
			if tt.check != nil {
				tt.check(t, rows)
			}
		})
	}
}

func TestParseSeedError(t *testing.T) {
	_, err := parseSeed(strings.NewReader("- just a scalar\n"))
	assertion.MustError(t, err)
}

func TestBuildUpsertQuery(t *testing.T) {
	rows := []map[string]any{
		{"id": 1, "name": "octocat", "github_id": 583231},
		{"id": 2, "name": "hubot"},
	}
	query, cols := buildUpsertQuery("trusted_user", rows)

	assertion.Equal(t, cols, []string{"github_id", "id", "name"})
	want := "INSERT INTO `trusted_user` (`github_id`, `id`, `name`) VALUES (?, ?, ?) " +
		"ON DUPLICATE KEY UPDATE `github_id` = VALUES(`github_id`), `id` = VALUES(`id`), `name` = VALUES(`name`)"
	assertion.Equal(t, query, want)
}
