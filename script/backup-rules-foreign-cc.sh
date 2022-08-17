#!/usr/bin/env bash
set -e

source "script/lib/bazel.sh"

VERSION="0.5.1"
SOURCE_URL="https://github.com/bazelbuild/rules_foreign_cc/archive/${VERSION}.tar.gz"
BUCKET=mirror
PATH_PREFIX=github.com/bazelbuild/rules_foreign_cc/archive

should_run_under_bazel

cd "$BUILD_WORKSPACE_DIRECTORY"

filename=$(basename "$SOURCE_URL")
curl -s -L -O "$SOURCE_URL"
mc cp "$filename" incluster/${BUCKET}/${PATH_PREFIX}/${filename}
rm "$filename"
