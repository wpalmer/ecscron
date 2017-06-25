#!/bin/bash
set -ex
go get
CGO_ENABLED=0 go build --ldflags '-extldflags "-static"' -buildmode exe -tags netgo
docker build .
