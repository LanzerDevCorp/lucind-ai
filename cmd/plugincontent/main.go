// Command plugincontent is a repository maintenance tool for the shipped
// Claude Code plugin's version and its recorded skill-tree content hash
// (internal/packet/testdata/skill_content_hash.txt). It is intentionally
// NOT wired into `go test ./...`, `lucind-ai check`, or `make install`: a
// version bump must always be a deliberate, human-run action, never a side
// effect of an ordinary plugin/claude-code/skills/lucind-ai/** content edit.
// See internal/skillcontent for the shared logic and the incident this
// decouples from go test.
//
// Usage (normally invoked through the Makefile, not directly):
//
//	go run ./cmd/plugincontent verify
//	go run ./cmd/plugincontent bump-version <new-version>
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/LanzerDevCorp/lucind-ai/internal/skillcontent"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

const usage = `usage:
  plugincontent verify
  plugincontent bump-version <new-version>`

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "plugincontent: %v\n", err)
		return 1
	}

	switch args[0] {
	case "verify":
		if len(args) != 1 {
			fmt.Fprintln(stderr, usage)
			return 2
		}
		if err := skillcontent.Verify(repoRoot); err != nil {
			fmt.Fprintf(stderr, "plugincontent: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "plugincontent: plugin.json version and recorded skill-content hash match the current tree")
		return 0

	case "bump-version":
		if len(args) != 2 {
			fmt.Fprintln(stderr, usage)
			return 2
		}
		newVersion := args[1]
		if err := skillcontent.Bump(repoRoot, newVersion); err != nil {
			fmt.Fprintf(stderr, "plugincontent: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "plugincontent: bumped plugin.json, marketplace.json, and %s to version %s\n", skillcontent.HashRecordRelPath, newVersion)
		return 0

	default:
		fmt.Fprintln(stderr, usage)
		return 2
	}
}
