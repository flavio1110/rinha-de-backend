#!/bin/bash

export LOCAL_ENV=true
export HTTP_PORT=9999
export DB_URL="postgres://user:super-secret@localhost:5432/people?sslmode=disable"

CGO_ENABLED=0 go build  -gcflags="all=-N -l"  -o ./bin/rinha-backend .

./bin/rinha-backend
