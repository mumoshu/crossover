#!/usr/bin/env bash

# Exit on Ctrl-C
trap "exit" INT

# Stop all the sub-processes on exit
trap 'if [ ! -z "$(jobs -p)" ]; then echo Stopping all sub-processes 1>&2 ; kill $(jobs -p); fi' EXIT

set -xe

eval "$@"
