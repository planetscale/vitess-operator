#!/bin/bash

cd "$(dirname "$0")"

function usage() {
  echo "Usage: docker.sh <command>"
  echo "command = build | run"
}

function run_build() {
  # clean up image of old builds
  docker rmi "vtop:go"

  # build image
  docker build --no-cache -t "vtop:go" -f Dockerfile.integrationgo ../
}

function run_run() {
  docker run -it --rm vtop:go /bin/bash
}

if [ "$1" != "" ]; then
  case $1 in
    build )
      run_build
    ;;
    run )
      run_run
    ;;
    * ) usage
        exit 1
  esac
else
  usage
  exit 1
fi
