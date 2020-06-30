#!/bin/sh

set -xu

CREATE_RANDOM='cat /dev/urandom | LC_CTYPE=C tr -dc "[:alpha:]" | head -c 8'
PROJECT_NAME=${TALKDESK_JENKINS_BUILD_NAME:-$(eval ${CREATE_RANDOM} | tr '[:upper:]' '[:lower:]')}
BUILD_TAG=${TALKDESK_CI_BUILD_TASK:-$(eval ${CREATE_RANDOM} | tr '[:upper:]' '[:lower:]')}

docker build --force-rm -f build/Dockerfile.tests -t "${PROJECT_NAME}:${BUILD_TAG}" .
docker run "${PROJECT_NAME}:${BUILD_TAG}" $1

EXIT=$?

docker rmi -f "${PROJECT_NAME}:${BUILD_TAG}"

exit $EXIT
