package main

import (
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/cloudcmd"
)

func runCloudCommand(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cloudcmd.Run(args, stdout, stderr)
}
