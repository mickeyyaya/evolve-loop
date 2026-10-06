package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
)

func runResolveLLM(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	role, code, ok := resolveLLMRole(args, stdout, stderr)
	if !ok {
		return code
	}
	router, routed := rootRouter(paths.ResolveFromEnv(), "[resolve-llm] ERROR", io.Discard, stderr)
	if !routed {
		return exitRoutingRefused
	}
	r, err := router.ResolveRole(role, resolvellm.Options{})
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

func resolveLLMRole(args []string, stdout, stderr io.Writer) (string, int, bool) {
	var role string
	for _, a := range args {
		switch {
		case a == "--help" || a == "-h":
			fmt.Fprintln(stdout, "Usage: evolve resolve-llm <role>")
			fmt.Fprintln(stdout, "Emits: {\"cli\":...,\"model_tier\":...,\"source\":\"profile\"}")
			return "", 0, false
		case len(a) >= 2 && a[:2] == "--":
			fmt.Fprintf(stderr, "[resolve-llm] unknown flag: %s\n", a)
			return "", 2, false
		case role != "":
			fmt.Fprintln(stderr, "[resolve-llm] too many arguments")
			return "", 2, false
		}
		role = a
	}
	if role == "" {
		fmt.Fprintln(stderr, "[resolve-llm] usage: evolve resolve-llm <role>")
		return "", 2, false
	}
	return role, 0, true
}
