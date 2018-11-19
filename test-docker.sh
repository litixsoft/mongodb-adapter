#!/usr/bin/env bash

docker pull golang:1.11
docker pull mongo:4.0

docker-compose -f ./compose.test.yaml build
docker-compose -f ./compose.test.yaml run adapter
docker-compose -f ./compose.test.yaml down
docker system prune --volumes -f
