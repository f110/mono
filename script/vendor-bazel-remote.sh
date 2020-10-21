#!/usr/bin/env bash
set -e

source "script/lib/vendor.sh"

OWNER="buchgr"
REPO="bazel-remote"
IMPORTPATH="github.com/buchgr/bazel-remote"
COMMIT="d165aec7aec6b3909001bbeaf2a9482698f5cb1c"

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

clean_git_files "${TARGET_DIR}"
clean_bazel_files "${TARGET_DIR}"
remove_unnecessary_go_files "${TARGET_DIR}"

cd "${TARGET_DIR}"
echo $COMMIT > COMMIT

generate_build_file "${TARGET_DIR}" "${IMPORTPATH}"