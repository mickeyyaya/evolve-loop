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
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
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

// ReorderArgs moves all flag tokens (starting with "-") ahead of positional
// tokens so Go's stdlib flag.Parse — which stops at the first positional —
// accepts flag-after-positional invocations like the bash predicates did
// (e.g. `probe foo --json`). Assumes flags are bool or use `--flag=value`
// form; space-separated `--flag value` is not handled (its callers use bool
// flags only). Extracted from cmd/evolve/args.go so the decomposed
// internal/cli/* command groups share one implementation.
func ReorderArgs(args []string) []string {
	flags := make([]string, 0, len(args))
	pos := make([]string, 0, len(args))
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
		} else {
			pos = append(pos, a)
		}
	}
	return append(flags, pos...)
}

// FilterEvolveEnv extracts the EVOLVE_* and BRIDGE_* variables from an environ
// slice ("KEY=VALUE" entries) into a map[KEY]VALUE — the env subset threaded
// into phase dispatch. Entries without '=' or with an empty key are skipped.
// Extracted from cmd_cycle.go so the decomposed internal/cli/* groups share one
// implementation.
func FilterEvolveEnv(environ []string) map[string]string {
	out := map[string]string{}
	for _, kv := range environ {
		eq := strings.IndexByte(kv, '=')
		if eq <= 0 {
			continue
		}
		k := kv[:eq]
		if strings.HasPrefix(k, "EVOLVE_") || strings.HasPrefix(k, "BRIDGE_") {
			out[k] = kv[eq+1:]
		}
	}
	return out
}

// NewPromptsLoader builds a prompts.Loader rooted at the EVOLVE_PROMPTS_DIR
// override when set, else at projectRoot. Extracted from the phase dispatch so
// cmd handlers and the cycle wiring share one loader-construction policy.
func NewPromptsLoader(projectRoot string) *prompts.Loader {
	if d := os.Getenv("EVOLVE_PROMPTS_DIR"); d != "" {
		return prompts.NewFromDir(d)
	}
	return prompts.NewFromDir(projectRoot)
}
