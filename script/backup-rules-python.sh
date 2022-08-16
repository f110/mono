#!/usr/bin/env bash
set -ex

source "script/lib/bazel.sh"

VERSION="0.9.0"
SOURCE_URL="https://github.com/bazelbuild/rules_python/archive/refs/tags/${VERSION}.tar.gz"
BUCKET=mirror
PATH_PREFIX=github.com/bazelbuild/rules_python/archive/refs/tags

should_run_under_bazel

cd "$BUILD_WORKSPACE_DIRECTORY"

filename=$(basename "$SOURCE_URL")
curl -s -L -O "$SOURCE_URL"
mc cp "$filename" incluster/${BUCKET}/${PATH_PREFIX}/${filename}
rm "$filename"
