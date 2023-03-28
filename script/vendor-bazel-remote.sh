#!/usr/bin/env bash
set -e

source "script/lib/vendor.sh"

OWNER="buchgr"
REPO="bazel-remote"
IMPORTPATH="github.com/buchgr/bazel-remote"
COMMIT="c5bf6e13938aa89923c637b5a4f01c2203a3c9f8" # v2.4.1

if [ -z "$BUILD_WORKSPACE_DIRECTORY" ]; then
    echo "Please execute via bazel"
    echo "bazel run //script:vendor-bazel-remote"
    exit 1
fi
cd "$BUILD_WORKSPACE_DIRECTORY"

THIRD_PARTY_DIR="${BUILD_WORKSPACE_DIRECTORY}/third_party"
TARGET_DIR="${THIRD_PARTY_DIR}/${REPO}"

if [ -d "${TARGET_DIR}" ]; then
  rm -rf "${TARGET_DIR}"
fi
mkdir -p "${TARGET_DIR}"

download_repository_from_github "${TARGET_DIR}" "${OWNER}" "${REPO}" "${COMMIT}"

remove_git_files "${TARGET_DIR}"
remove_bazel_files "${TARGET_DIR}"
remove_unnecessary_go_files "${TARGET_DIR}"

cd "${TARGET_DIR}"
echo $COMMIT > COMMIT

generate_build_file "${TARGET_DIR}" "${IMPORTPATH}"
