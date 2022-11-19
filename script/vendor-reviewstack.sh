#!/usr/bin/env bash
set -e

source "script/lib/vendor.sh"

OWNER="facebook"
REPO="sapling"
RELEASE="20221116-203146-2c1a971a"

if [ -z "$BUILD_WORKSPACE_DIRECTORY" ]; then
    echo "Please exec via bazel"
    echo "bazel run //script:vendor_reviewstack"
    exit 1
fi

WORK_DIR=$(mktemp -d)
THIRD_PARTY_DIR="${BUILD_WORKSPACE_DIRECTORY}/third_party"
TARGET_DIR="${THIRD_PARTY_DIR}/reviewstack"
if [ -d "${TARGET_DIR}" ]; then
    echo rm -rf "${TARGET_DIR}"
fi

download_release_from_github "${WORK_DIR}" "${OWNER}" "${REPO}" "${RELEASE}"
cd "${WORK_DIR}/addons"
yarn
cd "${WORK_DIR}/addons/reviewstack"
yarn codegen
cd "${WORK_DIR}/addons/reviewstack.dev"
yarn build
cp -r "${WORK_DIR}/addons/reviewstack.dev/build" "${TARGET_DIR}"
cp "${BUILD_WORKSPACE_DIRECTORY}/script/BUILD.reviewstack.bazel "${TARGET_DIR}/BUILD.bazel"
