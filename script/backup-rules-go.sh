#!/usr/bin/env bash
set -ex

source "script/lib/bazel.sh"

VERSION="v0.34.0"
SOURCE_URL="https://github.com/bazelbuild/rules_go/releases/download/${VERSION}/rules_go-${VERSION}.zip"
BUCKET=mirror
PATH_PREFIX=rules/rules_go

should_run_under_bazel

cd "$BUILD_WORKSPACE_DIRECTORY"

filename=$(basename "$SOURCE_URL")
curl -s -L -O "$SOURCE_URL"
mc cp "$filename" incluster/${BUCKET}/${PATH_PREFIX}/${filename}
