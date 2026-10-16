// Command apicover is the standalone binary over internal/apicover, the measurement the evolve subcommand shares.
// See docs/architecture/packages/cmd-apicover.md.
package main

import (
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
)

func main() {
	os.Exit(apicover.Main(os.Args[1:], os.Stdout, os.Stderr))
}
