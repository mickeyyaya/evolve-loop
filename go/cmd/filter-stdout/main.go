// filter-stdout writes the <phase>-stdout.clean.txt companion of a workspace.
// It runs the same logfilter that the runner uses.
package main

import (
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/logfilter"
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
