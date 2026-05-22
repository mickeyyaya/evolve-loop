// Package main is the evolve CLI entrypoint.
//
// Phase 1 subcommands: version (this file), doctor, guard, ledger, acs.
// Phase 2: loop, cycle, worktree, phase.
package main

import (
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/pkg/version"
)

const usage = `evolve — autonomous improvement loop (Go port)

Usage:
  evolve <command> [arguments]

Commands:
  version    Print build version and exit
  doctor     Probe environment (Phase 1 task #17)
  guard      Run a trust-kernel guard (Phase 1 tasks #13-14)
  ledger     Verify or tail the ledger (Phase 1 task #17)
  acs        Run ACS predicates for a cycle (Phase 1 tasks #16-17)

Phase 1 build — many subcommands still wired in subsequent tasks.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Println(version.Get())
	case "help", "--help", "-h":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "evolve: unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
}
