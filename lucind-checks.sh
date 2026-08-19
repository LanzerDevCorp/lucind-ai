#!/bin/sh
set -e
CGO_ENABLED=0 go build ./...
go test ./... -race -count=1
