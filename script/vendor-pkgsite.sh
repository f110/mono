#!/usr/bin/env bash
set -e

source "script/lib/vendor.sh"

OWNER="golang"
REPO="pkgsite"
IMPORTPATH="golang.org/x/pkgsite"
COMMIT="aa5dca7496c53df88ed8bb5279c4d76c02c78bd3"

if [ -z "$BUILD_WORKSPACE_DIRECTORY" ]; then
    echo "Please exec via bazel"
    echo "bazel run //script:vendor_pkgsite"
    exit 1
fi

PATCH_DIR="${BUILD_WORKSPACE_DIRECTORY}/script/pkgsite"
THIRD_PARTY_DIR="${BUILD_WORKSPACE_DIRECTORY}/third_party"
TARGET_DIR="${THIRD_PARTY_DIR}/${REPO}"
if [ -d "${TARGET_DIR}" ]; then
  rm -rf "${TARGET_DIR}"
fi

download_repository_from_github "${TARGET_DIR}" "${OWNER}" "${REPO}" "${COMMIT}"

remove_unnecessary_files "${TARGET_DIR}"

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
