#!/usr/bin/env bash
set -e

source "script/lib/vendor.sh"

OWNER="universal-ctags"
REPO="ctags"
COMMIT="40603a68c1f3b14dc1db4671111096733f6d2485"

if [ -z "$BUILD_WORKSPACE_DIRECTORY" ]; then
    echo "Please execute via bazel"
    echo "bazel run //script:vendor_universal_ctags"
    exit 1
fi
cd "$BUILD_WORKSPACE_DIRECTORY"

THIRD_PARTY_DIR="${BUILD_WORKSPACE_DIRECTORY}/third_party"
TARGET_DIR="${THIRD_PARTY_DIR}/${OWNER}/${REPO}"

if [ -d "${TARGET_DIR}" ]; then
  rm -rf "${TARGET_DIR}"
fi

download_repository_from_github "${TARGET_DIR}" "${OWNER}" "${REPO}" "${COMMIT}"

remove_unnecessary_files "${TARGET_DIR}"
rm -rf "${TARGET_DIR}/doc"
