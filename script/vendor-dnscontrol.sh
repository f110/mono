#!/usr/bin/env bash
set -e

REPOSITORY_URL="https://github.com/StackExchange/dnscontrol.git"
VERSION="v3.9.0"

THIRD_PARTY_DIR=$(cd $(dirname $0)/../third_party; pwd)
TARGET_NAME="${THIRD_PARTY_DIR}/dnscontrol"

if [ -d "${TARGET_NAME}" ]; then
  rm -rf "${TARGET_NAME}"
fi

cd "${THIRD_PARTY_DIR}"
git clone --depth 1 "${REPOSITORY_URL}" -b "$VERSION"

cd "${TARGET_NAME}"
find . -name "*_test.go" -delete
find . -name "testdata" -type d | xargs rm -rf
find . -name ".*" -maxdepth 1 | grep -v "^.$" | xargs rm -rf {} +

cat <<EOS > BUILD.bazel
load("@bazel_gazelle//:def.bzl", "gazelle")

# gazelle:proto disable_global
# gazelle:prefix github.com/StackExchange/dnscontrol/v3

gazelle(name = "gazelle")
EOS

bazel run //third_party/dnscontrol:gazelle