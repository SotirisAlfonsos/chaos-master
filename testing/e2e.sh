#!/bin/bash

printf %"s\n" "------------ Init start service and container ------------"
curl -X POST "http://127.0.0.1:8090/chaos/api/v1/service?action=start" \
-H "Content-Type: application/json" \
-d '{"job": "zookeeper service", "serviceName": "simple", "target": "127.0.0.1:8081"}'

curl -X POST "http://127.0.0.1:8090/chaos/api/v1/docker?action=start" \
-H "Content-Type: application/json" \
-d '{"job": "zookeeper docker", "containerName": "zookeeper", "target": "127.0.0.1:8081"}'

printf "\n"%"s\n" "------------ Stop service and container ------------"
curl -X POST "http://127.0.0.1:8090/chaos/api/v1/service?action=stop" \
-H "Content-Type: application/json" \
-d '{"job": "zookeeper service", "serviceName": "simple", "target": "127.0.0.1:8081"}'

curl -X POST "http://127.0.0.1:8090/chaos/api/v1/docker?action=stop" \
-H "Content-Type: application/json" \
-d '{"job": "zookeeper docker", "containerName": "zookeeper", "target": "127.0.0.1:8081"}'

printf "\n"%"s\n" "------------ Recover all ------------"
curl -X POST "http://127.0.0.1:8090/chaos/api/v1/recover/alertmanager" \
-H "Content-Type: application/json" \
-d '{"alerts": [{"status": "firing", "labels": {"recoverAll": true}}]}'

printf "\n"%"s\n" "------------ Stop docker ------------"
curl -X POST "http://127.0.0.1:8090/chaos/api/v1/docker?action=stop" \
-H "Content-Type: application/json" \
-d '{"job": "zookeeper docker", "containerName": "zookeeper", "target": "127.0.0.1:8081"}'

printf "\n"%"s\n" "------------ Recover job ------------"
curl -X POST "http://127.0.0.1:8090/chaos/api/v1/recover/alertmanager" \
-H "Content-Type: application/json" \
-d '{"alerts": [{"status": "firing", "labels": {"recoverJob": "zookeeper docker"}}]}'

printf "\n"%"s\n" "------------ Clean ------------"
curl -X POST "http://127.0.0.1:8090/chaos/api/v1/service?action=stop" \
-H "Content-Type: application/json" \
-d '{"job": "zookeeper service", "serviceName": "simple", "target": "127.0.0.1:8081"}'

curl -X POST "http://127.0.0.1:8090/chaos/api/v1/docker?action=stop" \
-H "Content-Type: application/json" \
-d '{"job": "zookeeper docker", "containerName": "zookeeper", "target": "127.0.0.1:8081"}'