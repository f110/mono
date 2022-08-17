#!/usr/bin/env bash
set -e

source "script/lib/bazel.sh"
source "script/lib/backup.sh"

VERSION="v0.34.0"
BUCKET=mirror

should_run_under_bazel
cd "$BUILD_WORKSPACE_DIRECTORY"

backup_github_release "bazelbuild/rules_go" "rules_go" "${VERSION}" "zip" "${BUCKET}"
