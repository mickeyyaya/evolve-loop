package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
)

// runResolveLLM is the `evolve resolve-llm <role>` subcommand. Resolves the
// role's CLI + model tier from its profile. Emits a single JSON line.
// (Step 9 removed llm_config.json, so the legacy optional [config_path] arg is
// gone.)
func runResolveLLM(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var role string
	for _, a := range args {
		switch a {
		case "--help", "-h":
			fmt.Fprintln(stdout, "Usage: evolve resolve-llm <role>")
			fmt.Fprintln(stdout, "Emits: {\"cli\":...,\"model_tier\":...,\"source\":\"profile\"}")
			return 0
		default:
			if len(a) >= 2 && a[:2] == "--" {
				fmt.Fprintf(stderr, "[resolve-llm] unknown flag: %s\n", a)
				return 2
			}
			if role == "" {
				role = a
			} else {
				fmt.Fprintln(stderr, "[resolve-llm] too many arguments")
				return 2
			}
		}
	}
	if role == "" {
		fmt.Fprintln(stderr, "[resolve-llm] usage: evolve resolve-llm <role>")
		return 2
	}
	r, err := resolvellm.Resolve(role, resolvellm.Options{})
	if err != nil {
		if errors.Is(err, resolvellm.ErrProfileNotFound) {
			fmt.Fprintf(stderr, "[resolve-llm] ERROR: profile not found for role '%s'\n", role)
			return 1
		}
		fmt.Fprintf(stderr, "[resolve-llm] ERROR: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, r.JSON())
	return 0
}
