#!/usr/bin/env bash
set -e

source "script/lib/bazel.sh"
source "script/lib/backup.sh"

VERSION="v4.11.0"
BUCKET=mirror

should_run_under_bazel
cd "$BUILD_WORKSPACE_DIRECTORY"

backup_github_release "golang-migrate/migrate" "migrate.linux-amd64.tar.gz" "${VERSION}" "${BUCKET}"
