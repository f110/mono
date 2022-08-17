#!/usr/bin/env bash

function backup_github_release() {
    repository=$1
    name=$2
    version=$3
    ext=$4
    bucket=$5

    source_url="https://github.com/${repository}/releases/download/${version}/${name}-${version}.${ext}"

    filename=${name}-${version}.${ext}
    curl -s -L -O "$source_url"
    mc cp "$filename" incluster/${bucket}/github.com/${repository}/releases/download/${version}/${filename}
    rm "$filename"
}
