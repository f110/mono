#!/usr/bin/env bash

function backup_github_release() {
    repository=$1
    name=$2
    version=$3
    bucket=$4

    source_url="https://github.com/${repository}/releases/download/${version}/${name}"

    curl -s -L -O "$source_url"
    mc cp "${name}" incluster/${bucket}/github.com/${repository}/releases/download/${version}/${name}
    rm "${name}"
}
