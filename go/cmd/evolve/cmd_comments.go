package main

import (
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
)

func runComments(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return commentaudit.Main(args, stdout, stderr, commentaudit.ExecGit{})
}
