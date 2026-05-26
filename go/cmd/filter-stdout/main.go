// filter-stdout is an operator utility: feed it a workspace + phase and
// it writes the <phase>-stdout.clean.txt companion using the same
// logfilter.Process the runner calls. Useful for retroactively cleaning
// pre-v12.2 cycle workspaces.
package main

import (
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/logfilter"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: filter-stdout <workspace> <phase>")
		os.Exit(2)
	}
	if err := logfilter.Process(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintf(os.Stderr, "filter error: %v\n", err)
		os.Exit(1)
	}
}
