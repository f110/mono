#!/usr/bin/env bash

BIN=@@BIN@@
KIND=@@KIND@@
ARGS=@@ARGS@@

$BIN --kind=$KIND "${ARGS[@]}"