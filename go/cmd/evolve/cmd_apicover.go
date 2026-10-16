package main

import (
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
)

func runApicover(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return apicover.Main(args, stdout, stderr)
}
