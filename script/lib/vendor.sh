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

function download_release_from_github() {
    target_dir=$1
    owner=$2
    repo=$3
    tag=$4
    sub_directory=$5

    if [ -f "/tmp/vendor.tar.gz" ]; then
        rm -f /tmp/vendor.tar.gz
    fi

    echo "Download the release file from github"
    curl --silent --location --output /tmp/vendor.tar.gz https://github.com/${owner}/${repo}/archive/refs/tags/${tag}.tar.gz
    if [ -n "$sub_directory" ]; then
        tmp_dir=$(mktemp -d)
        tar xfz /tmp/vendor.tar.gz --strip-components=1 --directory "$tmp_dir"
        rmdir "$target_dir"
        mv "$tmp_dir"/"$sub_directory" "$target_dir"/
        rm -rf "$tmp_dir"
    else
        mkdir -p "${target_dir}"
        tar xfz /tmp/vendor.tar.gz --strip-components=1 --directory ${target_dir}
    fi
}

function remove_unnecessary_files() {
    target_dir=$1

    remove_bazel_files "${target_dir}"
    remove_git_files "${target_dir}"
    remove_dev_files "${target_dir}"
    remove_unnecessary_go_files "${target_dir}"
}

function remove_bazel_files() {
    target_dir=$1

    echo "Remove bazel's files"
    find "${target_dir}" -name BUILD -or -name BUILD.bazel | xargs rm -f
    find "${target_dir}" -name WORKSPACE -or -name WORKSPACE.bazel | xargs rm -f
}

function remove_git_files() {
    target_dir=$1
    files=(
        ".gitignore"
        ".gitattributes"
    )

    echo "Remove git files"
    for file in "${files[@]}"; do
        find "${TARGET_DIR}" -name "$file" -delete
    done
}

function remove_dev_files() {
    target_dir=$1
    files=(
        ".editorconfig"
        ".prettierrc"
        ".travis.yml"
        "docker-compose.yaml"
        ".dockerignore"
    )
    dirs=(
        ".github"
    )

    echo "Remove files for development"
    for file in "${files[@]}"; do
        find "${TARGET_DIR}" -name "$file" -delete
    done
    for dir in "${dirs[@]}"; do
        rm -rf "$TARGET_DIR"/"${dir}"
    done
}

function remove_unnecessary_go_files() {
    target_dir=$1

    echo "Remove test files and data"
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

    echo "bazel run //${dir_path}:vendor"
    cd "${BUILD_WORKSPACE_DIRECTORY}"
    bazel run //${dir_path}:vendor

    cd "${old_working_directory}"
}