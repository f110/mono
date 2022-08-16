#!/usr/bin/env bash

function should_run_under_bazel() {
  if [ -z "$BUILD_WORKSPACE_DIRECTORY" ]; then
      echo "Please execute via bazel"
      exit 1
  fi
}
