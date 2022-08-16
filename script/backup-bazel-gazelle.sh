#!/usr/bin/env bash
set -e

source "script/lib/bazel.sh"

VERSION="v0.26.0"
SOURCE_URL="https://github.com/bazelbuild/bazel-gazelle/releases/download/${VERSION}/bazel-gazelle-${VERSION}.tar.gz"
BUCKET=mirror
PATH_PREFIX=github.com/bazelbuild/bazel-gazelle/releases/download/${VERSION}

should_run_under_bazel

cd "$BUILD_WORKSPACE_DIRECTORY"

filename=$(basename "$SOURCE_URL")
curl -s -L -O "$SOURCE_URL"
mc cp "$filename" incluster/${BUCKET}/${PATH_PREFIX}/${filename}
rm "$filename"
