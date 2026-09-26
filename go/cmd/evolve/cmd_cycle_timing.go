package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

// runCycleTiming renders a cycle's per-phase wall-clock table and archetype
// roll-up from its phase-timing.json; --json emits the phasetiming.Summary.
func runCycleTiming(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve cycle timing", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		projectRoot string
		evolveDir   string
		jsonOut     bool
		concurrency int
	)
	fs.StringVar(&projectRoot, "project-root", ".", "absolute path to the project root (default cwd)")
	fs.StringVar(&evolveDir, "evolve-dir", "", "path to .evolve/ state directory (default <project-root>/.evolve)")
	fs.BoolVar(&jsonOut, "json", false, "emit the cycle timing Summary as JSON")
	fs.IntVar(&concurrency, "concurrency", 2, "concurrency for the shadow parallel-evaluate projection")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	projectRoot = paths.AbsoluteRoot("--project-root", projectRoot, func(m string) {
		fmt.Fprintf(stderr, "evolve cycle timing: WARN: %s\n", m)
	})
	if evolveDir == "" {
		evolveDir = filepath.Join(projectRoot, ".evolve")
	}
	runsDir := filepath.Join(evolveDir, "runs")

	workspace, cycleLabel := resolveCycleWorkspace(runsDir, fs.Args())
	if workspace == "" {
		fmt.Fprintf(stderr, "evolve cycle timing: no cycle timing logs under %s\n", runsDir)
		return 10
	}
	entries, err := phasetiming.Read(workspace)
	if err != nil {
		fmt.Fprintf(stderr, "evolve cycle timing: read %s: %v\n", phasetiming.Path(workspace), err)
		return 10
	}

	summary := phasetiming.Rollup(entries)
	if jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(summary); err != nil {
			fmt.Fprintf(stderr, "evolve cycle timing: encode: %v\n", err)
			return 10
		}
		return 0
	}
	renderTiming(stdout, cycleLabel, entries, summary)
	renderParallelProjection(stdout, phasetiming.ProjectParallelSaving(entries, concurrency))
	return 0
}

// renderParallelProjection prints the shadow estimate of parallelizing the
// independent checking phases, only when it saves measurable time.
func renderParallelProjection(w io.Writer, p phasetiming.ParallelProjection) {
	if p.SavingMS == 0 {
		return
	}
	fmt.Fprintf(w, "Parallel-evaluate projection (shadow, C=%d): %s — %s → %s, would save %s\n",
		p.Concurrency, strings.Join(p.GroupPhases, ", "),
		phasetiming.HumanMS(p.SequentialMS), phasetiming.HumanMS(p.ProjectedParallelMS), phasetiming.HumanMS(p.SavingMS))
}

// resolveCycleWorkspace returns the requested cycle's workspace and label,
// defaulting to the highest-numbered cycle with a timing log.
func resolveCycleWorkspace(runsDir string, rest []string) (workspace, label string) {
	if len(rest) > 0 {
		label = "cycle-" + rest[0]
		return filepath.Join(runsDir, label), label
	}
	label = latestCycleDir(runsDir)
	if label == "" {
		return "", ""
	}
	return filepath.Join(runsDir, label), label
}

// latestCycleDir returns the highest-numbered cycle dir holding a timing log. A
// reset-suffixed dir (cycle-N.reset-…) parses on its leading integer, so a
// completed re-run beats an older sealed attempt.
func latestCycleDir(runsDir string) string {
	matches, _ := filepath.Glob(filepath.Join(runsDir, "cycle-*", phasetiming.FileName))
	best, bestN := "", -1
	for _, m := range matches {
		dir := filepath.Base(filepath.Dir(m))
		if n := cycleNumber(dir); n > bestN {
			bestN, best = n, dir
		}
	}
	return best
}

// cycleNumber extracts the leading integer from a "cycle-N[.suffix]" dir name.
func cycleNumber(dir string) int {
	s := strings.TrimPrefix(dir, "cycle-")
	if dot := strings.IndexByte(s, '.'); dot != -1 {
		s = s[:dot]
	}
	n, _ := strconv.Atoi(s)
	return n
}

func renderTiming(w io.Writer, cycleLabel string, entries []phasetiming.Entry, s phasetiming.Summary) {
	fmt.Fprintf(w, "Cycle timing — %s\n\n", cycleLabel)
	fmt.Fprintf(w, "%-22s %-9s %-8s %-8s %-3s %-4s %s\n", "PHASE", "ARCHETYPE", "START", "END", "ATT", "RETRY", "DURATION  VERDICT")
	for _, e := range entries {
		retry := ""
		if e.AttemptCount > 1 {
			retry = "↻"
		}
		fmt.Fprintf(w, "%-22s %-9s %-8s %-8s %-3d %-4s %-9s %s\n",
			truncate(e.Phase, 22), e.Archetype,
			clockOf(e.StartedAt), clockOf(e.EndedAt),
			e.AttemptCount, retry, phasetiming.HumanMS(e.DurationMS), e.Verdict)
	}
	fmt.Fprintln(w, strings.Repeat("─", 72))
	fmt.Fprintf(w, "Total: %s across %d phases (%d retried)\n", phasetiming.HumanMS(s.TotalMS), s.PhaseCount, s.RetriedCount)
	if s.LongestPhase != "" {
		fmt.Fprintf(w, "Longest: %s %s\n", s.LongestPhase, phasetiming.HumanMS(s.LongestMS))
	}
	fmt.Fprintf(w, "By archetype: %s\n", archetypeBreakdown(s))
}

// archetypeBreakdown renders the per-archetype share, highest first.
func archetypeBreakdown(s phasetiming.Summary) string {
	type kv struct {
		k string
		v int64
	}
	var rows []kv
	for k, v := range s.ByArchetype {
		rows = append(rows, kv{k, v})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].v > rows[j].v })
	parts := make([]string, 0, len(rows))
	for _, r := range rows {
		parts = append(parts, fmt.Sprintf("%s %.1f%%", r.k, s.ArchetypePercent(r.k)))
	}
	return strings.Join(parts, " · ")
}

// clockOf renders the HH:MM:SS of an RFC3339 timestamp, or "—" when absent.
func clockOf(rfc3339 string) string {
	if rfc3339 == "" {
		return "—"
	}
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return rfc3339
	}
	return t.UTC().Format("15:04:05")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
