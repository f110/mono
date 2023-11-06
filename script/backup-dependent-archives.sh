#!/usr/bin/env bash
set -e

source "script/lib/bazel.sh"
source "script/lib/backup.sh"

GAZELLE_VERSION="v0.26.0"
MIGRATE_VERSION="v4.11.0"
PROTOBUF_VERSION="v3.21.1"
RULES_GO_VERSION="v0.34.0"
RULES_PKG_VERSION="0.7.0"
RULES_PYTHON_VERSION="0.9.0"
RULES_FOREIGN_CC_VERSION="0.5.1"

BUCKET=mirror

should_run_under_bazel

cd "$BUILD_WORKSPACE_DIRECTORY"

backup_github_release "bazelbuild/bazel-gazelle" "bazel-gazelle-${GAZELLE_VERSION}.tar.gz" "${GAZELLE_VERSION}" "${BUCKET}"
backup_github_release "bazelbuild/rules_go" "rules_go-${RULES_GO_VERSION}.zip" "${RULES_GO_VERSION}" "${BUCKET}"
backup_github_release "bazelbuild/rules_pkg" "rules_pkg-${RULES_PKG_VERSION}.tar.gz" "${RULES_PKG_VERSION}" "${BUCKET}"
backup_github_release "golang-migrate/migrate" "migrate.linux-amd64.tar.gz" "${MIGRATE_VERSION}" "${BUCKET}"

backup_github_tags "bazelbuild/rules_python" "${RULES_PYTHON_VERSION}" "tar.gz" "${BUCKET}"
backup_github_tags "bazelbuild/rules_foreign_cc" "${RULES_FOREIGN_CC_VERSION}" "tar.gz" "${BUCKET}"
backup_github_tags "protocolbuffers/protobuf" "${PROTOBUF_VERSION}" "tar.gz" "${BUCKET}"
