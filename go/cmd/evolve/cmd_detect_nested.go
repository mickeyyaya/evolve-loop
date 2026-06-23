package main

import (
	"fmt"
	"io"

	"github.com/mickeyyaya/evolveloop/go/internal/detectnested"
)

// runDetectNested is the `evolve detect-nested-claude` subcommand.
// Ports legacy/scripts/dispatch/detect-nested-claude.sh. Prints "nested"
// or "standalone". --quiet suppresses stdout (rc still 0).
func runDetectNested(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	quiet := false
	for _, a := range args {
		switch a {
		case "--quiet", "-q":
			quiet = true
		case "--help", "-h":
			fmt.Fprintln(stdout, "Usage: evolve detect-nested-claude [--quiet]")
			return 0
		default:
			fmt.Fprintf(stderr, "evolve detect-nested-claude: unknown arg %q\n", a)
			return 0 // bash version exits 0 even on unknown args
		}
	}
	r := detectnested.Detect(detectnested.Options{})
	if !quiet {
		fmt.Fprintln(stdout, r)
	}
	return 0
}
