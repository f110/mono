#!/usr/bin/env bash
set -e

source "script/lib/bazel.sh"
source "script/lib/backup.sh"

VERSION="v0.17.0"
SOURCE_URL="https://github.com/bazelbuild/rules_docker/releases/download/${VERSION}/rules_docker-${VERSION}.tar.gz"
BUCKET=mirror
PATH_PREFIX=github.com/bazelbuild/rules_docker/releases/download/${VERSION}

should_run_under_bazel
cd "$BUILD_WORKSPACE_DIRECTORY"

backup_github_release "bazelbuild/rules_docker" "rules_docker" "${VERSION}" "tar.gz" "${BUCKET}"

filename=$(basename "$SOURCE_URL")
curl -s -L -O "$SOURCE_URL"
mc cp "$filename" incluster/${BUCKET}/${PATH_PREFIX}/${filename}
rm "$filename"
