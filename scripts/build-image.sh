#!/bin/bash

export SHELLOPTS	# propagate set to children by default
IFS=$'\t\n'

# Check required commands are in place
command -v go >/dev/null 2>&1 || { echo 'please install go'; exit 1; }
command -v docker >/dev/null 2>&1 || { echo 'please install docker'; exit 1; }

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ./bin/rinha-backend .

image="flavio1110/rinha-backend"
tag="${tag:=local}"

if [ "$tag" = "stable" ]
then
  docker buildx create --use
  docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 --no-cache=true -t "${image}:${tag}" --push .
else
  docker build --no-cache=true -t "${image}:${tag}" .
fi