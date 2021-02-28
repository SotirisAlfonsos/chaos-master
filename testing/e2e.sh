#!/bin/bash

verifyOK () {
  if ((!$#)); then
    printf "\n"%"s\n" "------------ E2e test FAILURE ------------"
    exit 1
  fi
}

startService () {
  echo $(curl -ss -X POST "http://127.0.0.1:8090/chaos/api/v1/service?action=start" \
  -H "Content-Type: application/json" \
  -d '{"job": '\""$1"\"', "serviceName": '\""$2"\"', "target": '\""$3"\"'}' | grep -E -i "$4")
}

stopService () {
  echo $(curl -ss -X POST "http://127.0.0.1:8090/chaos/api/v1/service?action=stop" \
  -H "Content-Type: application/json" \
  -d '{"job": '\""$1"\"', "serviceName": '\""$2"\"', "target": '\""$3"\"'}' | grep -E -i "$4")
}

startDocker () {
  echo $(curl -ss -X POST "http://127.0.0.1:8090/chaos/api/v1/docker?action=start" \
  -H "Content-Type: application/json" \
  -d '{"job": '\""$1"\"', "containerName": '\""$2"\"', "target": '\""$3"\"'}' | grep -E -i "$4")
}

stopDocker () {
  echo $(curl -ss -X POST "http://127.0.0.1:8090/chaos/api/v1/docker?action=stop" \
  -H "Content-Type: application/json" \
  -d '{"job": '\""$1"\"', "containerName": '\""$2"\"', "target": '\""$3"\"'}' | grep -E -i "$4")
}

startCPUInjection () {
  echo $(curl -ss -X POST "http://127.0.0.1:8090/chaos/api/v1/cpu?action=start" \
  -H "Content-Type: application/json" \
  -d '{"job": '\""$1"\"', "percentage": '"$2"', "target": '\""$3"\"'}' | grep -E -i "$4")
}

stopCPUInjection () {
  echo $(curl -ss -X POST "http://127.0.0.1:8090/chaos/api/v1/cpu?action=stop" \
  -H "Content-Type: application/json" \
  -d '{"job": '\""$1"\"', "target": '\""$2"\"'}' | grep -E -i "$3")
}

stopServer () {
  echo $(curl -ss -X POST "http://127.0.0.1:8090/chaos/api/v1/server?action=stop" \
  -H "Content-Type: application/json" \
  -d '{"job": '\""$1"\"', "target": '\""$2"\"'}' | grep -E -i "$3")
}

printf %"s\n" "------------ Init start service and container ------------"
out=$(startService "zookeeper service" "simple" "127.0.0.1:8081" '"status":200')
verifyOK $out

out=$(startService "zookeeper service" "simple" "127.0.0.1:8081" '"status":500')
verifyOK $out

out=$(startDocker "zookeeper docker" "zookeeper" "127.0.0.1:8081" '"status":200')
verifyOK $out

printf "\n"%"s\n" "------------ Start CPU failure ------------"
out=$(startCPUInjection "cpu_injection" 10 "127.0.0.1:8081" '"status":200')
verifyOK $out

sleep 1m

printf "\n"%"s\n" "------------ Start not existing service and container ------------"
out=$(startService "zookeeper_service" "test" "127.0.0.1:8081" '"status":500')
verifyOK $out

out=$(startDocker "zookeeper_docker" "test" "127.0.0.1:8081" '"status":500')
verifyOK $out


printf "\n"%"s\n" "------------ Stop service and container ------------"
out=$(stopService "zookeeper service" "simple" "127.0.0.1:8081" '"status":200')
verifyOK $out

out=$(stopDocker "zookeeper docker" "zookeeper" "127.0.0.1:8081" '"status":200')
verifyOK $out


printf "\n"%"s\n" "------------ Recover all ------------"
out=$(curl -ss -X POST "http://127.0.0.1:8090/chaos/api/v1/recover/alertmanager" \
-H "Content-Type: application/json" \
-d '{"alerts": [{"status": "firing", "labels": {"recoverAll": true}}]}' | grep -E -i '"status":200')
verifyOK $out


printf "\n"%"s\n" "------------ Stop docker ------------"
out=$(stopDocker "zookeeper docker" "zookeeper" "127.0.0.1:8081" '"status":200')
verifyOK $out

printf "\n"%"s\n" "------------ Stop CPU that has already recovered ------------"
out=$(stopCPUInjection "cpu_injection" "127.0.0.1:8081" '"status":500')
verifyOK $out


printf "\n"%"s\n" "------------ Recover job ------------"
out=$(curl -ss -X POST "http://127.0.0.1:8090/chaos/api/v1/recover/alertmanager" \
-H "Content-Type: application/json" \
-d '{"alerts": [{"status": "firing", "labels": {"recoverJob": "zookeeper docker"}}]}' | grep -E -i '"status":200')
verifyOK $out


printf "\n"%"s\n" "------------ Stop server ------------"
out=$(stopServer "server_injection" "127.0.0.1:8081" '"status":200')
verifyOK $out

printf "\n"%"s\n" "------------ Clean ------------"
out=$(stopService "zookeeper service" "simple" "127.0.0.1:8081" '"status":200')
verifyOK $out

out=$(stopDocker "zookeeper docker" "zookeeper" "127.0.0.1:8081" '"status":200')
verifyOK $out


printf "\n"%"s\n" "------------ E2e test SUCCESS ------------"