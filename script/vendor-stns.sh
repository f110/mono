#!/usr/bin/env bash
set -e

source "script/lib/vendor.sh"

OWNER="STNS"
REPO="STNS"
IMPORTPATH="github.com/${OWNER}/${REPO}/v2"
TAG="v2.2.10"
SUBDIRECTORY="v2"

if [ -z "$BUILD_WORKSPACE_DIRECTORY" ]; then
    echo "Please execute via bazel"
    echo "bazel run //script:vendor_stns"
    exit 1
fi
cd "$BUILD_WORKSPACE_DIRECTORY"

THIRD_PARTY_DIR="${BUILD_WORKSPACE_DIRECTORY}/third_party"
TARGET_DIR="${THIRD_PARTY_DIR}/${OWNER}/${REPO}"

if [ -d "${TARGET_DIR}" ]; then
  rm -rf "${TARGET_DIR}"
fi

download_release_from_github "${TARGET_DIR}" "${OWNER}" "${REPO}" "${TAG}" "${SUBDIRECTORY}"

remove_unnecessary_files "${TARGET_DIR}"
rm -rf "${TARGET_DIR}/docs"

generate_build_file "${TARGET_DIR}" "${IMPORTPATH}"