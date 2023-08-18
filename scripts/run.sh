#!/bin/bash

export LOCAL_ENV=true
CGO_ENABLED=0 go build  -gcflags="all=-N -l"  -o ./bin/rinha-backend .

./bin/rinha-backend
