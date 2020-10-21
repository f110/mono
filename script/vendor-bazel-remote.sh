#!/usr/bin/env bash
set -e

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

if [ -f "/tmp/vendor.tar.gz" ]; then
  rm -f /tmp/vendor.tar.gz
fi

curl -s -L -o /tmp/vendor.tar.gz https://github.com/${OWNER}/${REPO}/archive/${COMMIT}.tar.gz
tar xfz /tmp/vendor.tar.gz --strip-components=1 --directory ${TARGET_DIR}
find "${TARGET_DIR}" -name BUILD -or -name BUILD.bazel | xargs rm -f
find "${TARGET_DIR}" -name WORKSPACE -or -name WORKSPACE.bazel | xargs rm -f
find "${TARGET_DIR}" -name "*_test.go" -delete
find "${TARGET_DIR}" -name "testdata" -type d | xargs rm -rf
if [ -f "${TARGET_DIR}/.gitignore" ]; then
  rm -f "${TARGET_DIR}/.gitignore"
fi

cd "${TARGET_DIR}"
echo $COMMIT > COMMIT

cat <<EOS > BUILD.bazel
load("@dev_f110_rules_extras//go:vendor.bzl", "go_vendor")

# gazelle:prefix ${IMPORTPATH}

go_vendor(name = "vendor")
EOS

cd ../../
bazel run //third_party/${REPO}:vendor