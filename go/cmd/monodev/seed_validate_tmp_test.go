package main

import (
	"os"
	"sort"
	"testing"
)

func TestTmpValidateBuildSeedFiles(t *testing.T) {
	// NOT NULL columns of the `task` table that a seed row must provide (no
	// default). Kept in sync with go/build/database/schema.sql.
	taskNotNull := []string{
		"repository_id", "job_name", "parsed_job_configuration", "revision",
		"is_trunk", "bazel_version", "success", "log_file", "command", "target",
		"targets", "platform", "via", "config_name", "node", "job_object_name",
		"manifest", "container", "executed_tests_count", "succeeded_tests_count",
		"created_at",
	}
	artifactNotNull := []string{
		"task_id", "type", "hash", "name", "repository", "reference", "size",
		"created_at",
	}

	check := func(path string, notNull []string) {
		f, err := os.Open(path)
		if err != nil {
			t.Fatalf("open %s: %v", path, err)
		}
		defer f.Close()
		rows, err := parseSeed(f)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if len(rows) == 0 {
			t.Fatalf("%s: no rows", path)
		}
		_, cols := buildUpsertQuery("t", rows)
		has := make(map[string]bool)
		for _, c := range cols {
			has[c] = true
		}
		// Every row must carry every column of the union, otherwise the shared
		// prepared statement inserts NULL into a column some row omitted.
		for i, r := range rows {
			for _, c := range cols {
				if _, ok := r[c]; !ok {
					t.Errorf("%s row %d missing column %q present in others", path, i, c)
				}
			}
		}
		for _, c := range notNull {
			if !has[c] {
				t.Errorf("%s: NOT NULL column %q not seeded", path, c)
			}
		}
		sort.Strings(cols)
		t.Logf("%s: %d rows, columns=%v", path, len(rows), cols)
	}

	check("../../build/dev/seed/task.yaml", taskNotNull)
	check("../../build/dev/seed/artifact.yaml", artifactNotNull)
}
