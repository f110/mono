#!/usr/bin/env bash
set -e

OWNER="golang"
REPO="pkgsite"
IMPORTPATH="golang.org/x/pkgsite"
COMMIT="5ae7386a8ff7d9caec4c44f8ce68f402793f3954"

PATCH_DIR=$(cd $(dirname $0)/${REPO}; pwd)
THIRD_PARTY_DIR=$(cd $(dirname $0)/../third_party; pwd)
TARGET_DIR="${THIRD_PARTY_DIR}/${REPO}"

if [ -d "${TARGET_DIR}" ]; then
  rm -rf "${TARGET_DIR}"
fi
mkdir -p "${TARGET_DIR}"

if [ -f "/tmp/vendor.tar.gz" ]; then
  rm -f /tmp/vendor.tar.gz
fi

curl -L -o /tmp/vendor.tar.gz https://github.com/${OWNER}/${REPO}/archive/${COMMIT}.tar.gz
tar xfz /tmp/vendor.tar.gz --strip-components=1 --directory ${TARGET_DIR}
find "${TARGET_DIR}" -name "*_test.go" -delete
find "${TARGET_DIR}" -name "testdata" -type d | xargs rm -rf

cd "${TARGET_DIR}"
echo $COMMIT > COMMIT

cat <<EOS > BUILD.bazel
load("@dev_f110_rules_extras//go:vendor.bzl", "go_vendor")

# gazelle:prefix ${IMPORTPATH}

go_vendor(name = "vendor")
EOS

bazel run //third_party/${REPO}:vendor

for i in $(find $PATCH_DIR -type f)
do
  file=${i#"$PATCH_DIR/"}
  if [ -f "$TARGET_DIR/$file" ]; then
    cat "$PATCH_DIR/$file" >> "$TARGET_DIR/$file"
  else
    cp "$PATCH_DIR/$file" "$TARGET_DIR/$file"
  fi
done