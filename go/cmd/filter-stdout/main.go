// filter-stdout is an operator utility: feed it a workspace + phase and
// it writes the <phase>-stdout.clean.txt companion using the same
// logfilter.Process the runner calls. Useful for retroactively cleaning
// pre-v12.2 cycle workspaces.
package main

import (
	"fmt"
	"os"

	"github.com/mickeyyaya/evolveloop/go/internal/logfilter"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: filter-stdout <workspace> <phase>")
		return 2
	}
	if err := logfilter.Process(args[0], args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "filter error: %v\n", err)
		return 1
	}
	return 0
}
