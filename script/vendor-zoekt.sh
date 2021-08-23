#!/usr/bin/env bash
set -e

source "script/lib/vendor.sh"

OWNER="google"
REPO="zoekt"
IMPORTPATH="github.com/${OWNER}/${REPO}"
COMMIT="fcc0c9ab67c5e237fa886ef7a105d96c1b264d27"

if [ -z "$BUILD_WORKSPACE_DIRECTORY" ]; then
    echo "Please execute via bazel"
    echo "bazel run //script:vendor_zoekt"
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

generate_build_file "${TARGET_DIR}" "${IMPORTPATH}"