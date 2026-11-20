package main

import (
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/landcmd"
)

func runCI(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return landcmd.RunCI(args, stdin, stdout, stderr, landDeps())
}
