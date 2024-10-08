#!/usr/bin/env bash

GO=@@GO@@
GAZELLE_PATH=@@GAZELLE@@
DIR=@@DIR@@
ARGS=@@ARGS@@
PATCHES=@@PATCHES@@

RUNFILES=$(pwd)
GO_RUNTIME="$RUNFILES"/"$GO"
GAZELLE="$RUNFILES"/"$GAZELLE_PATH"

cd "$BUILD_WORKSPACE_DIRECTORY"/"$DIR"
"$GO_RUNTIME" mod tidy
"$GO_RUNTIME" mod vendor
find vendor -name BUILD.bazel -delete
find vendor -name BUILD -delete
for p in "${PATCHES[@]}"
do
    patch --remove-empty-files --strip 1 < "$RUNFILES"/"$p"
done
"$GAZELLE" update "${ARGS[@]}"
