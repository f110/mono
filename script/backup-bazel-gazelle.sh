#!/usr/bin/env bash
set -e

source "script/lib/bazel.sh"
source "script/lib/backup.sh"

VERSION="v0.26.0"
BUCKET=mirror

should_run_under_bazel
cd "$BUILD_WORKSPACE_DIRECTORY"

backup_github_release "bazelbuild/bazel-gazelle" "bazel-gazelle" "${VERSION}" "tar.gz" "${BUCKET}"
