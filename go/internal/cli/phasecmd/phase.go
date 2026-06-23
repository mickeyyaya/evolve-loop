// `evolve phase <name>` is the universal phase-subprocess entrypoint.
//
// Protocol (pkg/phaseproto, plan §1 decision #6):
//
//	stdin:  PhaseRequest JSON (one object)
//	stdout: PhaseResponse JSON (one object)
//	stderr: human-readable diagnostics
//	exit:   0 on success, 1 on internal error
//
// The same binary serves as both the in-process runner (called by the
// orchestrator) AND as the per-phase subprocess override via
// EVOLVE_PHASE_<NAME>_BIN. The subprocess override lets third parties
// implement a phase in any language as long as it speaks this protocol.
package phasecmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/phases/registry"

	// Blank imports drive the init()-time registry.Register calls for
	// every built-in phase. Adding a new phase = new package + import
	// line here; no edit to a dispatch switch (OCP). Every built-in phase
	// — ship and retro included — self-registers in its own package init().
	_ "github.com/mickeyyaya/evolveloop/go/internal/phases/audit"
	_ "github.com/mickeyyaya/evolveloop/go/internal/phases/build"
	_ "github.com/mickeyyaya/evolveloop/go/internal/phases/debugger"
	_ "github.com/mickeyyaya/evolveloop/go/internal/phases/intent"
	_ "github.com/mickeyyaya/evolveloop/go/internal/phases/retro"
	_ "github.com/mickeyyaya/evolveloop/go/internal/phases/scout"
	_ "github.com/mickeyyaya/evolveloop/go/internal/phases/ship"
	_ "github.com/mickeyyaya/evolveloop/go/internal/phases/tdd"
	_ "github.com/mickeyyaya/evolveloop/go/internal/phases/triage"
)

// runPhase implements `evolve phase <name>`.
func RunPhase(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintf(stderr, "evolve phase: missing phase name (%s)\n", strings.Join(registry.Names(), "|"))
		return 10
	}
	// `evolve phase verify ...` is the deliverable-contract self-check (ADR-0034),
	// not an in-process phase run — it reads no stdin JSON.
	if strings.ToLower(args[0]) == "verify" {
		return runPhaseVerify(args[1:], stdout, stderr)
	}
	// `evolve phase lint <name>` validates a phase descriptor against the unified
	// schema (ADR-0035). Fail-open: warnings only, never blocks.
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
		// Still emit the partial response so the parent can read
		// diagnostics; exit 1 marks the error.
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
