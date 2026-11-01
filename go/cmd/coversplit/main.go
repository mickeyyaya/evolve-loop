// Command coversplit prints one go test argument line for each coverage run of the named packages.
// See docs/architecture/packages/cmd-coversplit.md.
package main

import (
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/coversplit"
)

var exitProcess = os.Exit

func main() {
	exitProcess(coversplit.Main(os.Args[1:], os.Stdout, os.Stderr))
}
