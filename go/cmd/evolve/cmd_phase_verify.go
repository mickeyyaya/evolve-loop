package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// runPhaseVerify implements `evolve phase verify <phase> --workspace DIR
// [--worktree DIR] [--evolve-dir DIR] [--json]`. It is the agent-callable
// self-check (the Deliverable Contract block tells each agent to run it before
// finishing) and shares its verifier with the host-side contract gate so the
// two run byte-identical logic. ADR-0034.
//
// Exit codes:
//
//	0  — deliverable well-formed
//	1  — confirmed contract violation (agent must fix)
//	10 — usage error (missing/unknown phase)
//	2  — ambiguity/infra (e.g. unreadable dir) — caller should fail OPEN
func runPhaseVerify(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("phase verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	workspace := fs.String("workspace", "", "per-cycle workspace dir (.evolve/runs/cycle-N)")
	worktree := fs.String("worktree", "", "isolated build worktree (optional)")
	evolveDir := fs.String("evolve-dir", "", "project .evolve dir (for orchestrator deliverables)")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	if err := fs.Parse(reorderArgs(args)); err != nil {
		return 10
	}
	rest := fs.Args()
	if len(rest) < 1 {
		fmt.Fprintf(stderr, "evolve phase verify: missing phase name\n")
		return 10
	}
	phase := strings.ToLower(rest[0])
	if _, ok := phasecontract.For(phase); !ok {
		fmt.Fprintf(stderr, "evolve phase verify: unknown phase %q\n", phase)
		return 10
	}

	roots := phasecontract.Roots{Workspace: *workspace, Worktree: *worktree, EvolveDir: *evolveDir}
	res, err := deliverable.Verify(phase, roots)
	if err != nil {
		// Ambiguity/infra — fail OPEN at the call site.
		fmt.Fprintf(stderr, "evolve phase verify: %v\n", err)
		return 2
	}

	if *asJSON {
		buf, _ := json.MarshalIndent(res, "", "  ")
		fmt.Fprintln(stdout, string(buf))
	} else if res.OK {
		fmt.Fprintf(stdout, "OK: %s deliverable well-formed at %s\n", phase, res.ArtifactPath)
	} else {
		fmt.Fprintf(stderr, "FAIL: %s deliverable has %d violation(s):\n", phase, len(res.Violations))
		for _, v := range res.Violations {
			fmt.Fprintf(stderr, "  - [%s] %s\n", v.Code, v.Message)
		}
	}
	if res.OK {
		return 0
	}
	return 1
}
