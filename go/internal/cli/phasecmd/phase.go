package phasecmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"

	// Blank imports register every built-in phase through its package init().
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/build"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/debugger"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/intent"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/retro"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/scout"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/tdd"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/triage"
)

// RunPhase implements `evolve phase <name>`: a PhaseRequest JSON on stdin, a PhaseResponse JSON on stdout.
func RunPhase(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintf(stderr, "evolve phase: missing phase name (%s)\n", strings.Join(registry.Names(), "|"))
		return 10
	}
	if strings.ToLower(args[0]) == "verify" {
		return runPhaseVerify(args[1:], stdout, stderr)
	}
	if strings.ToLower(args[0]) == "lint" {
		return runPhaseLint(args[1:], stdout, stderr)
	}
	name := strings.ToLower(args[0])
	factory, ok := registry.For(name)
	if !ok {
		fmt.Fprintf(stderr, "evolve phase: unknown phase %q (known: %s)\n", name, strings.Join(registry.Names(), ", "))
		return 10
	}

	var req core.PhaseRequest
	dec := json.NewDecoder(stdin)
	if err := dec.Decode(&req); err != nil {
		fmt.Fprintf(stderr, "evolve phase: parse stdin JSON: %v\n", err)
		return 11
	}

	runner := factory(req)
	resp, err := runner.Run(context.Background(), req)
	if err != nil {
		// Emit the partial response anyway so the parent can read its diagnostics.
		buf, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(stdout, string(buf))
		fmt.Fprintf(stderr, "evolve phase: %s: %v\n", name, err)
		return 1
	}
	buf, mErr := json.MarshalIndent(resp, "", "  ")
	if mErr != nil {
		fmt.Fprintf(stderr, "evolve phase: marshal response: %v\n", mErr)
		return 1
	}
	fmt.Fprintln(stdout, string(buf))
	return 0
}
