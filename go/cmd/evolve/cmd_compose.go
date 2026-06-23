package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/phases/registry"
)

// runCompose implements `evolve compose --phases <list>`. v12.1
// Capability 2: ad-hoc phase composition that bypasses
// core.StateMachine.CanTransition for re-audit / mix-and-match runs.
//
// Each phase in --phases is run sequentially via the same factory the
// orchestrator uses. The state machine is NOT consulted; the kernel
// `evolve guard phase` hook downgrades from BLOCK to WARN when
// PhaseRequest.ComposePhases is true (set automatically by this subcommand).
//
// Ship safety: if "ship" appears in --phases without --ship-anyway,
// refuse early (the ship gate still enforces at the OS layer, but
// catching it here gives a friendlier error).
//
// Exit codes:
//   - 0  every phase PASS
//   - 1  at least one phase FAIL
//   - 2  invalid composition (e.g., ship without --ship-anyway)
//   - 10 bad args
func runCompose(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("compose", flag.ContinueOnError)
	fs.SetOutput(stderr)
	phasesArg := fs.String("phases", "", "comma-separated phase names to run in order (e.g., scout,audit)")
	shipAnyway := fs.Bool("ship-anyway", false, "permit 'ship' in the composition (otherwise refused early)")
	dryRun := fs.Bool("dry-run", false, "print the planned phase sequence; do not execute")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if *phasesArg == "" {
		fmt.Fprintf(stderr, "evolve compose: missing --phases (known: %s)\n",
			joinNames(registry.Names()))
		return 10
	}
	phases := splitNonEmptyPhases(*phasesArg)
	if len(phases) == 0 {
		fmt.Fprintln(stderr, "evolve compose: --phases produced empty list after trimming")
		return 10
	}
	// Validate every phase name is registered.
	known := registry.Names()
	knownSet := map[string]bool{}
	for _, n := range known {
		knownSet[n] = true
	}
	for _, p := range phases {
		if !knownSet[p] {
			fmt.Fprintf(stderr, "evolve compose: unknown phase %q (known: %s)\n",
				p, joinNames(known))
			return 10
		}
	}
	// Ship-safety guard.
	for _, p := range phases {
		if p == string(core.PhaseShip) && !*shipAnyway {
			fmt.Fprintln(stderr, "evolve compose: refusing to compose 'ship' without --ship-anyway")
			return 2
		}
	}

	// Parse the phase-request envelope from stdin (same shape as
	// `evolve phase`).
	body, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "evolve compose: read stdin: %v\n", err)
		return 1
	}
	var req core.PhaseRequest
	if len(strings.TrimSpace(string(body))) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			fmt.Fprintf(stderr, "evolve compose: parse stdin JSON: %v\n", err)
			return 10
		}
	}

	// Signal compose mode via the DI field so the kernel guard can soft-WARN instead of BLOCK.
	req.ComposePhases = true

	fmt.Fprintf(stdout, "[compose] sequence: %s\n", strings.Join(phases, " -> "))
	if *dryRun {
		fmt.Fprintln(stdout, "[compose] DRY-RUN; no phases will execute")
		return 0
	}

	overall := 0
	for i, p := range phases {
		factory, _ := registry.For(p)
		runner := factory(req)
		fmt.Fprintf(stdout, "[compose] %d/%d running %s\n", i+1, len(phases), p)
		resp, runErr := runner.Run(context.Background(), req)
		// Marshal the per-phase response for operator inspection.
		out, _ := json.MarshalIndent(resp, "  ", "  ")
		fmt.Fprintf(stdout, "  %s\n", out)
		if runErr != nil {
			fmt.Fprintf(stderr, "[compose] %s ERROR: %v\n", p, runErr)
			overall = 1
		}
		if resp.Verdict != "" && resp.Verdict != core.VerdictPASS && resp.Verdict != core.VerdictSKIPPED {
			fmt.Fprintf(stderr, "[compose] %s verdict=%s (composition continues)\n", p, resp.Verdict)
			overall = 1
		}
	}
	if overall == 0 {
		fmt.Fprintln(stdout, "[compose] all phases PASS")
	} else {
		fmt.Fprintln(stdout, "[compose] at least one phase did not PASS")
	}
	return overall
}

// splitNonEmptyPhases trims each element and drops empties so the
// caller can be loose with whitespace.
func splitNonEmptyPhases(csv string) []string {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}
