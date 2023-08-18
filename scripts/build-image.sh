#!/bin/bash

export SHELLOPTS	# propagate set to children by default
IFS=$'\t\n'

# Check required commands are in place
command -v go >/dev/null 2>&1 || { echo 'please install go'; exit 1; }
command -v docker >/dev/null 2>&1 || { echo 'please install docker'; exit 1; }

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ./bin/rinha-backend .

image="flavio1110/rinha-backend"
tag="local"

docker build --no-cache=true -t "${image}:${tag}" .