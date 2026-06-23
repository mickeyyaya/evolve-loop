package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolveloop/go/internal/bedrock"
)

// runBedrock is the `evolve build-invocation-context <role>` subcommand.
// Ports legacy/scripts/dispatch/build-invocation-context.sh.
// Output is byte-identical per role for prompt-cache reuse.
func runBedrock(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var role string
	for _, a := range args {
		switch a {
		case "--help", "-h":
			fmt.Fprintln(stdout, "Usage: evolve build-invocation-context <role>")
			fmt.Fprintln(stdout, "Roles: scout, builder, auditor, retrospective, "+
				"triage, plan-reviewer, tdd-engineer, intent, memo, inspirer,")
			fmt.Fprintln(stdout, "       evaluator, orchestrator (others emit common bedrock only)")
			return 0
		default:
			if role == "" {
				role = a
			} else {
				fmt.Fprintf(stderr, "[build-invocation-context] too many arguments\n")
				return 2
			}
		}
	}
	out, err := bedrock.Emit(role)
	if err != nil {
		if errors.Is(err, bedrock.ErrMissingRole) {
			fmt.Fprintln(stderr, "usage: build-invocation-context.sh <role>")
			return 2
		}
		fmt.Fprintf(stderr, "[build-invocation-context] FATAL: %v\n", err)
		return 1
	}
	fmt.Fprint(stdout, out)
	return 0
}
