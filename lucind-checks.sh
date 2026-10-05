#!/bin/sh
set -e
CGO_ENABLED=0 go build ./...
golangci-lint run
go test ./... -race -count=1
