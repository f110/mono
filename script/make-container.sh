#!/usr/bin/env bash

PACKAGES=(
    # base
    base-files_base
    base-passwd_data
    ca-certificates_data
    tzdata_zoneinfo
    netbase_etc
    libc6_libs
    libssl3_libs

    # Basic bazel dependency package
    g++_bin
    zip_bin
    unzip_bin

    curl_bin
    git_bin
    python3.10-minimal_bin
    python3-distutils_libs
)
CONTAINER_NAME=new-container

# Start up local container registry
REGISTRY_OUTPUT=./registry.log
touch $REGISTRY_OUTPUT
crane registry serve 1> >(tee $REGISTRY_OUTPUT) 2>&1 &
crane_pid=$!
echo "$crane_pid" > crane.pid
sleep 1
port=$(cat $REGISTRY_OUTPUT | sed -nr 's/.+serving on port ([0-9]+)/\1/p')

REGISTRY="127.0.0.1:$port"

# Make container
work_dir=$(mktemp -d)
echo "Working directory: $work_dir"
chisel cut --release ./chisel-release --root $work_dir --arch amd64 ${PACKAGES[@]}

crane append -f <(tar -f - -c -C $work_dir ./) -t $REGISTRY/tmp
crane pull $REGISTRY/tmp:latest ./new-container.tar --format tarball

# Clean up
kill $crane_pid
rm -rf $work_dir
