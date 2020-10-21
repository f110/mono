#!/usr/bin/env bash

function download_repository_from_github() {
    target_dir=$1
    owner=$2
    repo=$3
    commit=$4

    if [ -f "/tmp/vendor.tar.gz" ]; then
        rm -f /tmp/vendor.tar.gz
    fi

    curl -s -L -o /tmp/vendor.tar.gz https://github.com/${owner}/${repo}/archive/${commit}.tar.gz
    tar xfz /tmp/vendor.tar.gz --strip-components=1 --directory ${target_dir}
}

function clean_bazel_files() {
    target_dir=$1

    find "${target_dir}" -name BUILD -or -name BUILD.bazel | xargs rm -f
    find "${target_dir}" -name WORKSPACE -or -name WORKSPACE.bazel | xargs rm -f
}

function clean_git_files() {
    target_dir=$1

    if [ -f "${TARGET_DIR}/.gitignore" ]; then
        rm -f "${TARGET_DIR}/.gitignore"
    fi
}

function remove_unnecessary_go_files() {
    target_dir=$1

    find "${TARGET_DIR}" -name "*_test.go" -delete
    find "${TARGET_DIR}" -name "testdata" -type d | xargs rm -rf
}

function generate_build_file() {
    target_dir=$1
    importpath=$2

    dir_path=${target_dir##$BUILD_WORKSPACE_DIRECTORY}

    old_working_directory=$(pwd)

    cd "${target_dir}"
    cat <<EOS > BUILD.bazel
load("@dev_f110_rules_extras//go:vendor.bzl", "go_vendor")

# gazelle:prefix ${importpath}

go_vendor(name = "vendor")
EOS

    cd "${BUILD_WORKSPACE_DIRECTORY}"
    bazel run /${dir_path}:vendor

    cd "${old_working_directory}"
}