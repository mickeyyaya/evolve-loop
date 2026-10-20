// Command darwinset prints the go/.darwin-set file: the packages whose code or tests differ on macOS.
// See docs/architecture/packages/cmd-darwinset.md.
package main

import (
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
)

var exitProcess = os.Exit

func main() {
	exitProcess(ciparity.DarwinMain(os.Args[1:], os.Stdout, os.Stderr))
}
