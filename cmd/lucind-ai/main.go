// Command lucind-ai is the entry point for the lucind-ai delegated-execution
// binary. This file is pure wiring: it builds run.Deps from the outside
// world (git, the filesystem, a real clock, a real executor) and hands them
// to run.Execute, which is where the actual flow lives and is actually
// tested. Nothing here deserves a test of its own beyond the wiring and
// failure paths — see main_test.go.
package main

import (
	"context"
	"os"
)

// version is the build identifier reported by `lucind-ai --version` / `-v`.
// It is overridden at build time via:
//
//	go install -ldflags "-X main.version=$(git describe --tags --always --dirty)" ./cmd/lucind-ai
//
// (see the `install` target in the repository Makefile). The zero value
// "dev" marks a binary built without that flag, so an ad-hoc `go build`
// output is never silently mistaken for a real, traceable build.
var version = "dev"

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}
