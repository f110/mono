#!/usr/bin/env bash
set -e

source "script/lib/vendor.sh"

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

download_repository_from_github "${TARGET_DIR}" "${OWNER}" "${REPO}" "${COMMIT}"

remove_git_files "${TARGET_DIR}"
remove_bazel_files "${TARGET_DIR}"
remove_unnecessary_go_files "${TARGET_DIR}"

cd "${TARGET_DIR}"
echo $COMMIT > COMMIT

generate_build_file "${TARGET_DIR}" "${IMPORTPATH}"

for i in $(find $PATCH_DIR -type f)
do
  file=${i#"$PATCH_DIR/"}
  if [ -f "$TARGET_DIR/$file" ]; then
    cat "$PATCH_DIR/$file" >> "$TARGET_DIR/$file"
  else
    cp "$PATCH_DIR/$file" "$TARGET_DIR/$file"
  fi
done