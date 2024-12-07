#!/usr/bin/env bash

# Copyright 2021 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

source devtools/docker.sh || { echo "Are you at repo root?"; exit 1; }

set -e

# Script for running a nodejs docker image.
# It passes env variables for e2e tests,
# mounts the pwd into a volume in the container at /pkgsite,
# and sets the working directory in the container to /pkgsite.
dockercompose run --rm nodejs $@
