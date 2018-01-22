#!/usr/bin/env bats

load helpers

function setup(){
  _make create_environment
}

function teardown(){
  _make delete_environment
}

@test "send log message to logstash" {

  sample_message="this-is-one-logging-line"
  curl "http://${WEBAPPER_IP}:${WEBAPPER_PORT}/$sample_message" &>/dev/null
  sleep 2

  run docker logs logstash
  [[ "$status" -eq 0 ]]


  [[ "$(echo ${output} | _egrep "partial" "false")" -eq 0 ]]

  [[ "$(echo ${output} | _egrep 'containerName'       'webapper')"            -eq 0 ]]
  [[ "$(echo ${output} | _egrep 'containerImageName'  'rchicoli/webapper')"   -eq 0 ]]
  [[ "$(echo ${output} | _egrep 'source'              'stderr')"              -eq 0 ]]
  [[ "$(echo ${output} | _egrep 'message'             ".*$sample_message")"  -eq 0 ]]
  [[ "$(echo ${output} | _egrep 'containerID'         '[a-z0-9]*')"                   -eq 0 ]]
  [[ "$(echo ${output} | _egrep 'timestamp'           '([0-9]){4}-([0-9]){2}-([0-9]){2}T([0-9]){2}:([0-9]){2}:([0-9]){2}.([0-9]){9}Z')"    -eq 0 ]]

}
