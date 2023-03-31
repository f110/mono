#!/usr/bin/env bash
set -e

source "script/lib/vendor.sh"

OWNER="StackExchange"
REPO="dnscontrol"
IMPORTPATH="github.com/${OWNER}/${REPO}/v3"
TAG="v3.9.0"

if [ -z "$BUILD_WORKSPACE_DIRECTORY" ]; then
    echo "Please execute via bazel"
    echo "bazel run //script:vendor_dnscontrol"
    exit 1
fi
cd "$BUILD_WORKSPACE_DIRECTORY"

THIRD_PARTY_DIR="${BUILD_WORKSPACE_DIRECTORY}/third_party"
TARGET_DIR="${THIRD_PARTY_DIR}/${REPO}"

if [ -d "${TARGET_DIR}" ]; then
  rm -rf "${TARGET_DIR}"
fi
mkdir -p "${TARGET_DIR}"

download_release_from_github "${TARGET_DIR}" "${OWNER}" "${REPO}" "${TAG}"

remove_unnecessary_files "${TARGET_DIR}"
rm -rf "${TARGET_DIR}/docs"

generate_build_file "${TARGET_DIR}" "${IMPORTPATH}"
