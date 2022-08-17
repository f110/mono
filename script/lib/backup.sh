#!/usr/bin/env bash

function backup_github_release() {
    repository=$1
    name=$2
    version=$3
    bucket=$4

    source_url="https://github.com/${repository}/releases/download/${version}/${name}"
    key="github.com/${repository}/releases/download/${version}/${name}"

    if ! mc stat "incluster/${bucket}/${key}" > /dev/null 2>&1; then
        echo "Download ${source_url} and put ${key}"
        curl -s -L -O "$source_url"
        mc cp "${name}" "incluster/${bucket}/${key}"
        rm "${name}"
    else
        echo "Skip to download because ${key} is already exist."
    fi
}

function backup_github_tags() {
    repository=$1
    version=$2
    ext=$3
    bucket=$4

    source_url="https://github.com/${repository}/archive/refs/tags/${version}.${ext}"
    key="github.com/${repository}/archive/refs/tags/${version}.${ext}"

    if ! mc stat "incluster/${bucket}/${key}" > /dev/null 2>&1; then
        echo "Download ${source_url} and put ${key}"
        filename="${version}.${ext}"
        curl -s -L -O "${source_url}"
        mc cp "$filename" "incluster/${bucket}/${key}"
        rm "$filename"
    else
        echo "Skip to download because ${key} is already exist"
    fi
}
