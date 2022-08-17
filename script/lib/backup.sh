#!/usr/bin/env bash

function backup_github_release() {
    repository=$1
    name=$2
    version=$3
    bucket=$4

    source_url="https://github.com/${repository}/releases/download/${version}/${name}"
    key="github.com/${repository}/releases/download/${version}/${name}"

    if ! mc stat "incluster/${bucket}/${key}" > /dev/null 2>&1; then
        curl -s -L -O "$source_url"
        mc cp "${name}" "incluster/${bucket}/${key}"
        rm "${name}"
    else
        echo "Skip to download because ${key} is already exists."
    fi
}
