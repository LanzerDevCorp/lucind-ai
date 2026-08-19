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

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}
