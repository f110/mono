#!/usr/bin/env bash
set -e

source "script/lib/vendor.sh"

OWNER="minio"
REPO="minio"
IMPORTPATH="github.com/${OWNER}/${REPO}"
COMMIT="1aa8896ad69a3e23b0f439cb83f873956ed620a7"

if [ -z "$BUILD_WORKSPACE_DIRECTORY" ]; then
    echo "Please execute via bazel"
    echo "bazel run //script:vendor_minio"
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

remove_unnecessary_files "${TARGET_DIR}"
rm -rf "${TARGET_DIR}/docs"
rm -rf "${TARGET_DIR}/helm"
rm -rf "${TARGET_DIR}/helm-releases"

generate_build_file "${TARGET_DIR}" "${IMPORTPATH}"
