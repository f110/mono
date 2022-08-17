#!/usr/bin/env bash
set -e

source "script/lib/bazel.sh"
source "script/lib/backup.sh"

VERSION="0.7.0"
BUCKET=mirror

should_run_under_bazel

cd "$BUILD_WORKSPACE_DIRECTORY"

backup_github_release "bazelbuild/rules_pkg" "rules_pkg" "${VERSION}" "tar.gz" "${BUCKET}"
