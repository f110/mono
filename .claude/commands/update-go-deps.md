---
description: Update direct Go module dependencies one by one, with a commit per module
---

# Goal

Update each direct dependency in `go.mod` (`Indirect = false`) to its latest version, regenerate `BUILD.bazel` via `make update-deps`, verify no regressions with the CI-equivalent bazel tests, then commit at the granularity of one module per commit.

# Pre-flight

1. Verify the working tree is clean:
   ```
   git status
   ```
   If there are uncommitted changes, confirm with the user before stashing.

2. List candidates to update:
   ```
   go list -m -u -f '{{if not .Indirect}}{{if .Update}}{{.Path}} {{.Version}} -> {{.Update.Version}}{{end}}{{end}}' all
   ```

# Per-module procedure

Process the list top-down. For each module:

1. **Re-check the latest version** (it may already have been bumped transitively by a previous update):
   ```
   go list -m -u <module path>
   ```
   If already up to date, skip to the next module.

2. **Update the dependency**:
   ```
   go get <module path>@<latest version>
   ```

3. **Regenerate BUILD.bazel**:
   ```
   make update-deps
   ```

4. **Run bazel tests** (mirroring CI's target list):
   ```
   bazel test -- //... -//containers/zoekt-indexer/... -//containers/zoekt-webserver/... -//py/...
   ```
   **Important**: the `--` separator is required because negative patterns follow.

5. **Inspect failures**:
   - Build errors usually mean a breaking API change — fix the call sites (example: in `aws-sdk-go-v2 v1.41.7`, `Size`/`ContentLength` switched from `int64` to `*int64`; wrap with `aws.ToInt64()`).
   - If `querylog` flakes, retry with `bazel test //go/database/querylog:querylog_test --runs_per_test=2`.
   - For unfixable incompatibilities (module rename, upstream behavioral regression, etc.), confirm with the user and skip.

6. **Commit**:
   ```
   git add go.mod go.sum [plus any code changes needed]
   git commit -m "Bump up <module path> to <new version>"
   ```
   Bundle any follow-up code changes into the same commit.

# Known modules to skip

These currently break when updated. Confirm with the user and leave them pinned:

- `github.com/go-sql-driver/mysql v1.10.0`: incompatible with MariaDB 11.4 — `Stmt.Exec("SELECT ...")` hangs.
- `github.com/minio/minio-operator v0.4.0`: module path was renamed to `github.com/minio/operator`.

# Wrap-up

Final check:
```
go list -m -u -f '{{if not .Indirect}}{{if .Update}}{{.Path}} {{.Version}} -> {{.Update.Version}}{{end}}{{end}}' all
```
Confirm that only the "known skipped" entries remain. Run `git log --oneline <pre-work HEAD>..HEAD` and show the commit list to the user before signing off.

# Notes

- A single `go get` can bump other direct deps transitively (e.g. updating `cloud.google.com/go/storage` also pulls newer `golang.org/x/crypto`). That's fine — include the extra bumps in that commit. When you later try to `go get` those modules individually and see no diff, just skip them.
- Distinguish flaky tests from real regressions carefully: persistent failures = regression, passing on retry = flake.
- If you find code that's already broken on master (e.g. a half-finished migration), land a fixing commit first before starting the dependency updates.
