package main

// cmd_routing.go — ADR-0052 WS3-S4: `evolve routing explain --cycle N` renders
// a recorded routing decision (the clamped plan, the integrity-floor clamps,
// and the OTel decision span) for debugging WHY a cycle ran the phases it did.
// WS3-S5 adds the `replay` subcommand. Pure reader: no state/ledger/registry
// mutation — safe to run mid-batch.

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/router"
)

func runRouting(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "evolve routing: usage: routing explain --cycle N [--project-root P]")
		return 10
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "explain":
		return runRoutingExplain(rest, stdout, stderr)
	case "replay":
		return runRoutingReplay(rest, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve routing: unknown subcommand %q (want: explain | replay)\n", sub)
		return 10
	}
}

// routingCycleRoot resolves the per-cycle workspace dir for the shared --cycle/
// --project-root flags both subcommands take. Returns ("", code) on a usage or
// cwd error (caller returns code); otherwise ("<workspace>", 0).
func routingCycleRoot(fs *flag.FlagSet, cycle *int, root *string, args []string, name string, stderr io.Writer) (string, int) {
	if err := fs.Parse(args); err != nil {
		return "", 10
	}
	if *cycle <= 0 {
		fmt.Fprintf(stderr, "evolve routing %s: --cycle N is required (N>=1)\n", name)
		return "", 10
	}
	pr := *root
	if pr == "" {
		pr = os.Getenv("EVOLVE_PROJECT_ROOT")
	}
	if pr == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "evolve routing %s: %v\n", name, err)
			return "", 1
		}
		pr = cwd
	}
	return cycleWorkspace(pr, *cycle), 0
}

func runRoutingExplain(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("routing explain", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cycle := fs.Int("cycle", 0, "cycle number to explain (required, >=1)")
	root := fs.String("project-root", "", "project root (default: cwd or EVOLVE_PROJECT_ROOT)")
	ws, code := routingCycleRoot(fs, cycle, root, args, "explain", stderr)
	if ws == "" {
		return code
	}
	fmt.Fprintf(stdout, "Routing decision — cycle %d\n  workspace: %s\n\n", *cycle, ws)
	explainPlan(stdout, ws)
	explainClamps(stdout, ws)
	explainSpan(stdout, ws)
	return 0
}

// runRoutingReplay reparses the captured advisor response through the live
// parse+clamp (core.ReplayPlanFromResponse) and compares its RUN-SET to the
// recorded phase-plan.json. MATCH (exit 0) ⇒ the capture still reproduces the
// recorded plan; MISMATCH (exit 3) ⇒ it diverged — a corrupted/tampered
// capture, or (in WS4) a prompt/model regression. Replay uses the DEFAULT ship
// floor; a per-cycle policy-floor override is not persisted, so the run-set
// comparison is the robust, floor-anchored invariant (it's exactly what WS4-S2
// locks: never schedules a forbidden phase / ship-without-audit).
func runRoutingReplay(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("routing replay", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cycle := fs.Int("cycle", 0, "cycle number to replay (required, >=1)")
	root := fs.String("project-root", "", "project root (default: cwd or EVOLVE_PROJECT_ROOT)")
	ws, code := routingCycleRoot(fs, cycle, root, args, "replay", stderr)
	if ws == "" {
		return code
	}
	fmt.Fprintf(stdout, "Replay — cycle %d (default ship floor)\n  workspace: %s\n\n", *cycle, ws)

	raw, err := os.ReadFile(filepath.Join(ws, "advisor-response-plan.txt"))
	if err != nil {
		fmt.Fprintln(stdout, "(no captured advisor-response-plan.txt to replay)")
		return 0
	}
	clamped, _, err := core.ReplayPlanFromResponse(string(raw), router.RouteInput{}, router.DefaultShipFloor())
	if err != nil {
		fmt.Fprintf(stdout, "MISMATCH: captured response no longer parses (corrupted/tampered): %v\n", err)
		return 3
	}
	replayed := runSet(clamped.Entries)
	fmt.Fprintf(stdout, "replayed run-set: %v\n", replayed)

	var recorded []router.PhasePlanEntry
	if !readJSONArtifact(filepath.Join(ws, "phase-plan.json"), &recorded) {
		fmt.Fprintln(stdout, "(no recorded phase-plan.json to compare against)")
		return 0
	}
	recordedSet := runSet(recorded)
	fmt.Fprintf(stdout, "recorded run-set: %v\n\n", recordedSet)

	if slices.Equal(replayed, recordedSet) {
		fmt.Fprintln(stdout, "MATCH: replayed run-set equals recorded phase-plan.json")
		return 0
	}
	fmt.Fprintln(stdout, "MISMATCH: replayed run-set diverges from recorded phase-plan.json")
	return 3
}

// runSet returns the sorted names of the phases an entry list runs (Run==true).
func runSet(entries []router.PhasePlanEntry) []string {
	var out []string
	for _, e := range entries {
		if e.Run {
			out = append(out, e.Phase)
		}
	}
	slices.Sort(out)
	return out
}

// explainPlan renders the clamped whole-cycle plan (phase-plan.json) as one
// RUN/SKIP line per phase with its justification. A missing/unreadable plan is
// a clean message, not an error — a partially-recorded cycle still explains.
func explainPlan(w io.Writer, ws string) {
	var entries []router.PhasePlanEntry
	if !readJSONArtifact(filepath.Join(ws, "phase-plan.json"), &entries) || len(entries) == 0 {
		fmt.Fprintln(w, "Plan: (no phase plan recorded)")
		fmt.Fprintln(w)
		return
	}
	fmt.Fprintln(w, "Plan:")
	for _, e := range entries {
		verb := "SKIP"
		if e.Run {
			verb = "RUN "
		}
		fmt.Fprintf(w, "  %s %s", verb, e.Phase)
		if e.Mint != nil {
			fmt.Fprint(w, " [minted]")
		}
		if e.Justification != "" {
			fmt.Fprintf(w, " — %s", e.Justification)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w)
}

// explainClamps aggregates the integrity-floor clamps recorded across the
// cycle's routing-decision-*.json artifacts (RouterDecision.Clamps), in stable
// file order. No clamps ⇒ a clean message (the advisor's plan passed the floor
// untouched, which is the healthy case).
func explainClamps(w io.Writer, ws string) {
	files, _ := filepath.Glob(filepath.Join(ws, "routing-decision-*.json"))
	slices.Sort(files)
	var clamps []router.Clamp
	for _, f := range files {
		var dec router.RouterDecision
		if readJSONArtifact(f, &dec) {
			clamps = append(clamps, dec.Clamps...)
		}
	}
	if len(clamps) == 0 {
		fmt.Fprintln(w, "Clamps: (none — advisory plan passed the integrity floor unchanged)")
		fmt.Fprintln(w)
		return
	}
	fmt.Fprintln(w, "Clamps:")
	for _, c := range clamps {
		fmt.Fprintf(w, "  %s (%s → %s)\n", c.Rule, c.Proposed, c.Forced)
	}
	fmt.Fprintln(w)
}

// explainSpan renders the OTel-GenAI decision span (advisor-span-plan.json).
func explainSpan(w io.Writer, ws string) {
	var span core.AdvisorSpan
	if !readJSONArtifact(filepath.Join(ws, "advisor-span-plan.json"), &span) {
		fmt.Fprintln(w, "Span: (no decision span recorded)")
		return
	}
	fmt.Fprintln(w, "Span:")
	fmt.Fprintf(w, "  model:        %s\n", span.Model)
	fmt.Fprintf(w, "  system:       %s\n", span.System)
	fmt.Fprintf(w, "  duration_ms:  %d\n", span.DurationMS)
	fmt.Fprintf(w, "  prompt_sha:   %s\n", span.PromptSHA)
	fmt.Fprintf(w, "  response_sha: %s\n", span.ResponseSHA)
}

// readJSONArtifact reads+unmarshals path into v, returning false on any
// absence/read/parse error (the caller renders a clean "not recorded" line —
// explain is read-only and best-effort by contract).
func readJSONArtifact(path string, v any) bool {
	buf, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return json.Unmarshal(buf, v) == nil
}
