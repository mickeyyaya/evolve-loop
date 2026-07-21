// `evolve tokens report [--last N]` is the S7 (token-telemetry) read-only
// reporter: it walks the last N cycles' phase-timing.json logs (the same
// per-cycle walk budgethistory.Collect uses for duration/cost) and rolls
// their per-phase terminal token usage into a ranked top-consumers table —
// the evidence that answers "which phase/site is burning the most tokens"
// once S1-S6 populate real (non-zero) token counts. --json emits the
// TokensReport struct for tooling.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

// PhaseTokenTotal is one phase's summed token usage across the walked
// cycles, plus how many of those cycles actually ran it.
type PhaseTokenTotal struct {
	Phase      string                `json:"phase"`
	Tokens     cyclestate.TokenUsage `json:"tokens"`
	CycleCount int                   `json:"cycle_count"`
}

// TokensReport is the walked-window aggregate `evolve tokens report` emits.
// PhasesWithData/PhasesRun are the telemetry-coverage counters (cycle-779):
// how many walked phase runs carried ANY token data versus how many ran —
// so an unmeasured window reads as a coverage gap, not as "free".
type TokensReport struct {
	CyclesWalked   []int                 `json:"cycles_walked"`
	Phases         []PhaseTokenTotal     `json:"phases"` // ranked, highest InputTokens first
	TotalTokens    cyclestate.TokenUsage `json:"total_tokens"`
	WastedTokens   cyclestate.TokenUsage `json:"wasted_tokens"` // FAIL-verdict phases (work that didn't ship)
	CacheHitRatio  float64               `json:"cache_hit_ratio"`
	PhasesWithData int                   `json:"phases_with_data"`
	PhasesRun      int                   `json:"phases_run"`
	// TripwireCount/Tripwires surface the engine's telemetry-coverage tripwire
	// (cycle-1005): a non-claude launch that exits 0, runs past the 60s success
	// threshold, and resolves to source=none burned real tokens the resolver
	// never measured. The engine records `"tripwire":true` in llm-calls.ndjson;
	// this reporter reads and surfaces it so the miss shows up in the report,
	// not just engine stderr.
	TripwireCount int             `json:"tripwire_count"`
	Tripwires     []TripwireEvent `json:"tripwires"`
}

// TripwireEvent is one surfaced telemetry-coverage tripwire — the CLI/agent/
// phase/cycle of a non-claude success launch whose token usage went unmeasured.
type TripwireEvent struct {
	Cycle      int    `json:"cycle"`
	CLI        string `json:"cli"`
	Agent      string `json:"agent"`
	Phase      string `json:"phase"`
	DurationMS int64  `json:"duration_ms"`
	ExitCode   int    `json:"exit_code"`
}

// llmCallTripwireRecord is the minimal decode shape for an llm-calls.ndjson
// record — a local copy of the tripwire-relevant fields (matching engine
// llmCallLog's json tags) so cmd/evolve need not import internal/bridge's
// unexported record type (the wiring class that broke prior attempts).
type llmCallTripwireRecord struct {
	CLI        string `json:"cli"`
	Agent      string `json:"agent"`
	Phase      string `json:"phase"`
	DurationMS int64  `json:"duration_ms"`
	ExitCode   int    `json:"exit_code"`
	Tripwire   bool   `json:"tripwire"`
}

func runTokens(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "evolve tokens: missing subcommand (try: report)")
		return 10
	}
	switch args[0] {
	case "report":
		return runTokensReport(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve tokens: unknown subcommand %q\n", args[0])
		return 10
	}
}

func runTokensReport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve tokens report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		projectRoot string
		evolveDir   string
		last        int
		jsonOut     bool
	)
	fs.StringVar(&projectRoot, "project-root", ".", "absolute path to the project root (default cwd)")
	fs.StringVar(&evolveDir, "evolve-dir", "", "path to .evolve/ state directory (default <project-root>/.evolve)")
	fs.IntVar(&last, "last", 8, "number of most-recent cycles to walk")
	fs.BoolVar(&jsonOut, "json", false, "emit the TokensReport as JSON")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if last <= 0 {
		fmt.Fprintf(stderr, "evolve tokens report: --last must be positive, got %d\n", last)
		return 10
	}
	projectRoot = paths.AbsoluteRoot("--project-root", projectRoot, func(m string) {
		fmt.Fprintf(stderr, "evolve tokens report: WARN: %s\n", m)
	})
	if evolveDir == "" {
		evolveDir = filepath.Join(projectRoot, ".evolve")
	}
	runsDir := filepath.Join(evolveDir, "runs")

	cycles := recentCyclesWithTiming(runsDir, last)
	if len(cycles) == 0 {
		fmt.Fprintf(stderr, "evolve tokens report: no cycle timing logs under %s\n", runsDir)
		return 10
	}

	report := buildTokensReport(runsDir, cycles)
	if jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(stderr, "evolve tokens report: encode: %v\n", err)
			return 10
		}
		return 0
	}
	renderTokensReport(stdout, report)
	return 0
}

// recentCyclesWithTiming scans runsDir for cycle-N dirs carrying a
// phase-timing.json, and returns up to `last` of the highest cycle numbers,
// ascending (oldest first — matches budgethistory.Collect's walk order).
func recentCyclesWithTiming(runsDir string, last int) []int {
	matches, _ := filepath.Glob(filepath.Join(runsDir, "cycle-*", phasetiming.FileName))
	seen := map[int]bool{}
	var nums []int
	for _, m := range matches {
		n := cycleNumber(filepath.Base(filepath.Dir(m)))
		if n <= 0 || seen[n] {
			continue
		}
		seen[n] = true
		nums = append(nums, n)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(nums)))
	if len(nums) > last {
		nums = nums[:last]
	}
	sort.Ints(nums)
	return nums
}

// buildTokensReport walks the given cycle numbers' phase-timing.json logs and
// rolls their per-phase terminal token usage into a ranked TokensReport.
// Missing/unreadable cycles are skipped as absent evidence, matching
// budgethistory.Collect's degrade-gracefully contract.
func buildTokensReport(runsDir string, cycles []int) TokensReport {
	totals := map[string]cyclestate.TokenUsage{}
	counts := map[string]int{}
	report := TokensReport{CyclesWalked: cycles}
	var cacheReadSum, cacheDenomSum int
	for _, c := range cycles {
		ws := filepath.Join(runsDir, fmt.Sprintf("cycle-%d", c))
		// Read tripwires before the phasetiming continue so a cycle with real
		// tripwire hits but no/empty phase-timing data still surfaces them.
		tws := readCycleTripwires(runsDir, c)
		report.Tripwires = append(report.Tripwires, tws...)
		report.TripwireCount += len(tws)
		entries, err := phasetiming.Read(ws)
		if err != nil {
			continue
		}
		for _, e := range entries {
			totals[e.Phase] = addTokenUsage(totals[e.Phase], e.Tokens)
			counts[e.Phase]++
			report.PhasesRun++
			if e.Tokens != (cyclestate.TokenUsage{}) {
				report.PhasesWithData++
			}
			report.TotalTokens = addTokenUsage(report.TotalTokens, e.Tokens)
			if e.Verdict == "FAIL" {
				report.WastedTokens = addTokenUsage(report.WastedTokens, e.Tokens)
			}
		}
		s := phasetiming.Rollup(entries)
		cacheReadSum += s.TotalTokens.CacheRead
		cacheDenomSum += s.TotalTokens.Input + s.TotalTokens.CacheRead
	}
	if cacheDenomSum > 0 {
		report.CacheHitRatio = float64(cacheReadSum) / float64(cacheDenomSum)
	}
	report.Phases = rankPhasesByInputTokens(totals, counts)
	return report
}

// readCycleTripwires streams a cycle's llm-calls.ndjson (bufio.Scanner, bounded
// — not read-all-then-split) and returns the tripwire-flagged records as
// TripwireEvents. A missing/unreadable file or a malformed line is skipped as
// absent evidence, matching buildTokensReport's degrade-gracefully contract.
func readCycleTripwires(runsDir string, cycle int) []TripwireEvent {
	path := filepath.Join(runsDir, fmt.Sprintf("cycle-%d", cycle), "llm-calls.ndjson")
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []TripwireEvent
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024) // tolerate long records
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec llmCallTripwireRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		if !rec.Tripwire {
			continue
		}
		out = append(out, TripwireEvent{
			Cycle: cycle, CLI: rec.CLI, Agent: rec.Agent, Phase: rec.Phase,
			DurationMS: rec.DurationMS, ExitCode: rec.ExitCode,
		})
	}
	return out
}

// stripControlBytes removes control bytes (< 0x20 and 0x7f) from an
// llm-calls.ndjson-sourced string before it reaches the TTY. A compromised
// non-claude driver could embed ANSI escapes in its own record's CLI/agent/
// phase fields to rewrite or hide the very tripwire line meant to expose it
// (cycle-1010 audit F1); the --json path is unaffected (already safe).
func stripControlBytes(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

// rankPhasesByInputTokens sorts phases by summed InputTokens, highest first;
// ties break alphabetically by phase name for deterministic output.
func rankPhasesByInputTokens(totals map[string]cyclestate.TokenUsage, counts map[string]int) []PhaseTokenTotal {
	rows := make([]PhaseTokenTotal, 0, len(totals))
	for phase, tok := range totals {
		rows = append(rows, PhaseTokenTotal{Phase: phase, Tokens: tok, CycleCount: counts[phase]})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Tokens.Input != rows[j].Tokens.Input {
			return rows[i].Tokens.Input > rows[j].Tokens.Input
		}
		return rows[i].Phase < rows[j].Phase
	})
	return rows
}

// addTokenUsage sums two TokenUsage values field-wise.
func addTokenUsage(a, b cyclestate.TokenUsage) cyclestate.TokenUsage {
	a.Input += b.Input
	a.Output += b.Output
	a.CacheRead += b.CacheRead
	a.CacheWrite += b.CacheWrite
	return a
}

func renderTokensReport(w io.Writer, r TokensReport) {
	fmt.Fprintf(w, "Token usage report — cycles %v\n\n", r.CyclesWalked)
	renderTripwires(w, r)
	if len(r.Phases) == 0 {
		fmt.Fprintln(w, "(no token usage recorded in this window)")
		fmt.Fprintf(w, "Coverage: %d/%d phases with token data\n", r.PhasesWithData, r.PhasesRun)
		return
	}
	fmt.Fprintf(w, "%-28s %10s %10s %10s %10s %6s\n", "PHASE", "INPUT", "OUTPUT", "CACHE_R", "CACHE_W", "CYCLES")
	for _, p := range r.Phases {
		fmt.Fprintf(w, "%-28s %10d %10d %10d %10d %6d\n",
			truncate(p.Phase, 28), p.Tokens.Input, p.Tokens.Output, p.Tokens.CacheRead, p.Tokens.CacheWrite, p.CycleCount)
	}
	fmt.Fprintf(w, "\nTotal: input=%d output=%d cache_read=%d cache_write=%d\n",
		r.TotalTokens.Input, r.TotalTokens.Output, r.TotalTokens.CacheRead, r.TotalTokens.CacheWrite)
	fmt.Fprintf(w, "Wasted (FAIL verdicts): input=%d output=%d\n", r.WastedTokens.Input, r.WastedTokens.Output)
	fmt.Fprintf(w, "Cache-hit ratio: %.1f%%\n", r.CacheHitRatio*100)
	fmt.Fprintf(w, "Coverage: %d/%d phases with token data\n", r.PhasesWithData, r.PhasesRun)
}

// renderTripwires prints the telemetry-coverage tripwire section. It runs
// unconditionally above the empty-phases early return so a cycle with tripwire
// hits but no phase-timing data still surfaces the miss (cycle-1007 render-order
// regression). Silent when no tripwire fired. CLI/agent/phase fields are
// control-byte-stripped before hitting the TTY (F1).
func renderTripwires(w io.Writer, r TokensReport) {
	if r.TripwireCount == 0 {
		return
	}
	fmt.Fprintf(w, "⚠️  TRIPWIRE: %d non-claude success launch(es) resolved to source=none — token usage unmeasured\n", r.TripwireCount)
	for _, t := range r.Tripwires {
		fmt.Fprintf(w, "  TRIPWIRE cycle=%d cli=%s agent=%s phase=%s dur=%dms exit=%d\n",
			t.Cycle, stripControlBytes(t.CLI), stripControlBytes(t.Agent), stripControlBytes(t.Phase), t.DurationMS, t.ExitCode)
	}
	fmt.Fprintln(w)
}
