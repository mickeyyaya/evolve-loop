package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/phasecmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
)

// runCompose runs the named phases in order through the orchestrator's
// factories without consulting the state machine. It exits 1 when any phase
// fails, 2 for ship without --ship-anyway, and 10 on bad arguments.
func runCompose(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("compose", flag.ContinueOnError)
	fs.SetOutput(stderr)
	phasesArg := fs.String("phases", "", "comma-separated phase names to run in order (e.g., scout,audit)")
	shipAnyway := fs.Bool("ship-anyway", false, "permit 'ship' in the composition (otherwise refused early)")
	dryRun := fs.Bool("dry-run", false, "print the planned phase sequence; do not execute")
	cycle := fs.Int("cycle", 0, "derive the request from .evolve/runs/cycle-N/cycle-state.json instead of stdin")
	projectRoot := fs.String("project-root", "", "project root for --cycle (default EVOLVE_PROJECT_ROOT or cwd)")
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
	if rc := refuseComposePhases(phases, *shipAnyway, stderr); rc != 0 {
		return rc
	}

	req, rc := composeRequest(*cycle, *projectRoot, stdin, stderr)
	if rc != 0 {
		return rc
	}

	// In compose mode the kernel phase guard warns instead of blocking.
	req.ComposePhases = true

	fmt.Fprintf(stdout, "[compose] sequence: %s\n", strings.Join(phases, " -> "))
	if *dryRun {
		fmt.Fprintln(stdout, "[compose] DRY-RUN; no phases will execute")
		return 0
	}
	if err := installRootRouter(req.ProjectRoot, stderr); err != nil {
		fmt.Fprintf(stderr, "evolve compose: %v\n", err)
		return exitRoutingRefused
	}

	overall := 0
	for i, p := range phases {
		factory, _ := registry.For(p)
		runner := factory(req)
		fmt.Fprintf(stdout, "[compose] %d/%d running %s\n", i+1, len(phases), p)
		resp, runErr := runner.Run(context.Background(), req)
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

func composeRequest(cycle int, projectRoot string, stdin io.Reader, stderr io.Writer) (core.PhaseRequest, int) {
	if cycle != 0 {
		req, err := phasecmd.RequestForCycle(projectRoot, cycle, stdin)
		if err != nil {
			fmt.Fprintf(stderr, "evolve compose: %v\n", err)
			return core.PhaseRequest{}, phasecmd.CycleRequestExitCode(err)
		}
		return req, 0
	}
	body, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "evolve compose: read stdin: %v\n", err)
		return core.PhaseRequest{}, 1
	}
	var req core.PhaseRequest
	if len(strings.TrimSpace(string(body))) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			fmt.Fprintf(stderr, "evolve compose: parse stdin JSON: %v\n", err)
			return core.PhaseRequest{}, 10
		}
	}
	return req, 0
}

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

func refuseComposePhases(phases []string, shipAnyway bool, stderr io.Writer) int {
	known := registry.Names()
	for _, p := range phases {
		if !slices.Contains(known, p) {
			fmt.Fprintf(stderr, "evolve compose: unknown phase %q (known: %s)\n", p, joinNames(known))
			return 10
		}
	}
	if slices.Contains(phases, string(core.PhaseShip)) && !shipAnyway {
		fmt.Fprintln(stderr, "evolve compose: refusing to compose 'ship' without --ship-anyway")
		return 2
	}
	return 0
}
