package main

import (
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
)

// runApicover shares apicover.Main with the standalone binary, so a deployed
// cycle runs the API gate without building a second executable.
func runApicover(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return apicover.Main(args, stdout, stderr)
}
