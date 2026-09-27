package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
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

// Replay always uses the DEFAULT ship floor, since a per-cycle policy-floor
// override is not persisted; the run-set comparison stays floor-anchored.
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

func readJSONArtifact(path string, v any) bool {
	buf, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return json.Unmarshal(buf, v) == nil
}
