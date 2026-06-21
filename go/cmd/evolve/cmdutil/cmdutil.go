// Package cmdutil holds tiny helpers shared by the `evolve` CLI's
// per-subcommand handlers. Deliberately minimal — this is not a CLI
// framework. The existing manual flag parsing in each cmd_*.go file is
// already idiomatic Go; cmdutil only extracts the 1-2 patterns that
// were verbatim-duplicated across 10+ files. It is the single shared home
// for these helpers, imported by both package main and the decomposed
// internal/cli/* command groups (so no group re-duplicates them).
package cmdutil

import (
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

// HasHelp reports whether args contains the conventional help flags
// ("-h" or "--help"). Use as an early-exit gate at the top of a
// subcommand's Run function:
//
//	if cmdutil.HasHelp(args) {
//	    fmt.Fprintln(stdout, "Usage: ...")
//	    return 0
//	}
//
// Replaces the verbatim 6-line `for _, a := range args { if a == "-h"
// || a == "--help" { ... } }` block that lived in 13 cmd_*.go files.
func HasHelp(args []string) bool {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return true
		}
	}
	return false
}

// EnvOrCwd returns the absolute root named by environment variable env when it
// is set — absolutized via paths.AbsoluteRoot so downstream paths are
// cwd-independent (the cycle-119 fix class) — otherwise the current working
// directory (already absolute), otherwise ".". Extracted from cmd_subagent.go
// so the decomposed internal/cli/* command groups share one implementation.
func EnvOrCwd(env string) string {
	if v := os.Getenv(env); v != "" {
		return paths.AbsoluteRoot(env, v, nil)
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}
