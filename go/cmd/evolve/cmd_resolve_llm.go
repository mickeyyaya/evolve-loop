package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
)

// runResolveLLM is the `evolve resolve-llm <role> [config_path]` subcommand.
// Ports legacy/scripts/dispatch/resolve-llm.sh. Emits a single JSON line.
func runResolveLLM(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var role, configPath string
	for _, a := range args {
		switch a {
		case "--help", "-h":
			fmt.Fprintln(stdout, "Usage: evolve resolve-llm <role> [config_path]")
			fmt.Fprintln(stdout, "Emits: {\"cli\":...,\"model\"|\"model_tier\":...,\"source\":...}")
			return 0
		default:
			if len(a) >= 2 && a[:2] == "--" {
				fmt.Fprintf(stderr, "[resolve-llm] unknown flag: %s\n", a)
				return 2
			}
			if role == "" {
				role = a
			} else if configPath == "" {
				configPath = a
			} else {
				fmt.Fprintln(stderr, "[resolve-llm] too many arguments")
				return 2
			}
		}
	}
	if role == "" {
		fmt.Fprintln(stderr, "[resolve-llm] usage: resolve-llm.sh <role> [config_path]")
		return 2
	}
	r, err := resolvellm.Resolve(role, resolvellm.Options{ConfigPath: configPath})
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
