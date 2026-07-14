// Command apicover is a thin CLI wrapper over the importable internal/apicover
// package (folded there so the evolve binary — the `evolve apicover` subcommand
// and the audit's in-process API-gate — and this standalone binary all share one
// measurement implementation, never rebuilt at runtime). See package apicover
// for the flag and behavior documentation, plus
// docs/architecture/adr/0050-modularization-and-unified-phase-io.md.
package main

import (
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
)

func main() {
	os.Exit(apicover.Main(os.Args[1:], os.Stdout, os.Stderr))
}
