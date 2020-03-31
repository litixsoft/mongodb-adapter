#!/usr/bin/env bash

export MONGO_VERSION=4.2
export GOLANG_VERSION=1.14
docker pull golang:${GOLANG_VERSION}
docker pull mongo:${MONGO_VERSION}

docker-compose -f ./compose.test.yaml build
docker-compose -f ./compose.test.yaml run adapter
docker-compose -f ./compose.test.yaml down
