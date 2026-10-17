// Command e2elist prints the anchored go test -run pattern of the tests that only the e2e build tag compiles.
// See docs/architecture/packages/cmd-e2elist.md.
package main

import (
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
)

var exitProcess = os.Exit

func main() {
	exitProcess(ciparity.E2EMain(os.Args[1:], os.Stdout, os.Stderr))
}
