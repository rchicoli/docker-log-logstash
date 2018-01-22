#!/bin/bash

LOGSTASH_IP="172.31.0.2"
LOGSTASH_PORT="9200"

WEBAPPER_IP="172.31.0.3"
WEBAPPER_PORT="8080"

# this is required for the makefile
export BASE_DIR="$BATS_TEST_DIRNAME/.."

DOCKER_COMPOSE_FILE="${BASE_DIR}/docker/docker-compose.yml"
MAKEFILE="${BASE_DIR}/Makefile"
MAKE="make -f ${MAKEFILE} "

# make wrapper
function _make() {
  run make -f "$MAKEFILE" "$@"
}

function _expr() {
  expr "$(cat /dev/stdin)" : "$@"
}

function _egrep() {
  local field="$1"
  local message="$2"
  grep -Eoq "\"${field}\" => \"?${message}\"?"
  echo $?
}
