// Command commentaudit ranks packages by comment load and proves an edit
// touched only comments. See docs/conventions/code-comments.md.
package main

import (
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
)

func main() {
	os.Exit(commentaudit.Main(os.Args[1:], os.Stdout, os.Stderr, commentaudit.ExecGit{}))
}
