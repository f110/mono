#!/usr/bin/env bash
set -e

source "script/lib/bazel.sh"

VERSION="v3.21.1"
SOURCE_URL="https://github.com/protocolbuffers/protobuf/archive/refs/tags/${VERSION}.tar.gz"
BUCKET=mirror
PATH_PREFIX=github.com/protocolbuffers/protobuf/archive/refs/tags

should_run_under_bazel

cd "$BUILD_WORKSPACE_DIRECTORY"

filename=$(basename "$SOURCE_URL")
curl -s -L -O "$SOURCE_URL"
mc cp "$filename" incluster/${BUCKET}/${PATH_PREFIX}/${filename}
rm "$filename"
