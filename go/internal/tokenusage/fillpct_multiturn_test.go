package tokenusage

// fillpct_multiturn_test.go — RED contract for cycle-1455 task
// `contextfill-ratio-over-100pct` (inbox 2026-08-12, weight 0.75,
// pipeline-repair; observed twice in one monitored wave: scout 566.9%, triage
// 114.3%).
//
// The defect, in one line: ScanConfigRoot sums EVERY assistant turn's
// Input+CacheRead+CacheWrite into one grand total (scanner.go:147-153), and
// DefaultResolver feeds that summed total straight into FillPct against a
// SINGLE-turn 200K window (defaultresolver.go:38). Each turn's own
// cache_read_input_tokens already carries that turn's entire prior context, so
// summing turn N with turn N+1 re-counts the same context once per turn. A
// 12-turn phase near the ceiling therefore reports several hundred percent.
//
// The contract is BEHAVIOURAL and implementation-agnostic — it never names a
// new symbol. Every fixture below grows monotonically (real transcripts do:
// context only accumulates within a phase), so the terminal turn IS the peak
// turn and either extraction satisfies these tests. What the fixtures DO rule
// out is the sum, the first turn, and the mean.
//
// The two invariants the fix must not break: Result.Usage stays the SUM (it is
// the cost/spend number, and correct as-is), and an honest over-100% reading
// stays unclamped and legible (fillpct.go's own documented promise).

import (
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// pctTolerance is the float comparison slack for percentage assertions.
const pctTolerance = 0.05

// multiTurnFixture writes a transcript whose first user message carries
// artifactPath (the primary attribution key) followed by the given assistant
// turns, and returns the config root to hand ScanConfigRoot/DefaultResolver.
// Turns carry no timestamp, so withinWindow admits them all — these fixtures
// exercise the fill arithmetic, not the window filter.
func multiTurnFixture(t *testing.T, artifactPath string, turns []string) string {
	t.Helper()
	root := t.TempDir()
	body := `{"type":"user","message":{"id":"u1","content":"Artifact path: ` + artifactPath + `"}}` + "\n"
	for _, turn := range turns {
		body += turn + "\n"
	}
	writeTranscript(t, filepath.Join(root, "projects", "-some-worktree"), "sess.jsonl", body)
	return root
}

// assistantTurn renders one assistant transcript line with the four usage
// counters a real Claude Code transcript reports.
func assistantTurn(id string, input, output, cacheRead, cacheWrite int) string {
	return `{"type":"assistant","message":{"id":"` + id + `","usage":{` +
		`"input_tokens":` + strconv.Itoa(input) +
		`,"output_tokens":` + strconv.Itoa(output) +
		`,"cache_read_input_tokens":` + strconv.Itoa(cacheRead) +
		`,"cache_creation_input_tokens":` + strconv.Itoa(cacheWrite) + `}}}`
}

// inflatedTurns is the 566.9%-class reproducer: three turns whose prompt-side
// totals are 70_000 / 130_000 / 180_000. Summed they are 380_000 (190% of the
// 200K claude window — the bug). The terminal (== peak) turn is 180_000, a
// plausible 90%.
func inflatedTurns() []string {
	return []string{
		assistantTurn("m1", 5_000, 1_000, 60_000, 5_000),
		assistantTurn("m2", 2_000, 2_000, 120_000, 8_000),
		assistantTurn("m3", 1_000, 3_000, 178_000, 1_000),
	}
}

// TestFillPct_UsesTerminalTurnNotSumOfTurns is the load-bearing predicate for
// the defect. 90% is the terminal turn's own occupancy; 190% is the summed
// artefact this cycle removes. The intermediate assertions name the specific
// wrong answers so a failure diagnoses itself.
func TestFillPct_UsesTerminalTurnNotSumOfTurns(t *testing.T) {
	const artifact = "/ws/cycle-1455/scout-report.md"
	root := multiTurnFixture(t, artifact, inflatedTurns())

	got, err := DefaultResolver(root)(Window{Driver: "claude-tmux", ArtifactPath: artifact})
	if err != nil {
		t.Fatalf("resolver returned error (telemetry must be best-effort): %v", err)
	}
	if got.Source != SourceTranscript {
		t.Fatalf("Source = %q, want %q — fixture did not reach the transcript tier", got.Source, SourceTranscript)
	}
	if math.Abs(got.FillPct-190) <= pctTolerance {
		t.Fatalf("FillPct = %v — this is the SUM of all three turns' prompt-side tokens (380000/200000). "+
			"Each turn's cache_read already carries that turn's whole prior context; summing turns re-counts it once per turn (the 566.9%% live symptom)", got.FillPct)
	}
	if math.Abs(got.FillPct-35) <= pctTolerance {
		t.Fatalf("FillPct = %v — this is the FIRST turn (70000/200000), not the turn that shows how full the window ended up", got.FillPct)
	}
	if math.Abs(got.FillPct-90) > pctTolerance {
		t.Errorf("FillPct = %v, want 90 (the terminal/peak turn's own 180000 prompt-side tokens / 200000 window)", got.FillPct)
	}
}

// TestScanConfigRoot_MultiTurnTranscript_UsageStaysTheSum is the anti-overfit
// half: Result.Usage is the COST number and summing turns is correct for it.
// A fix that repairs fill% by making the scanner stop summing would silently
// under-report every cycle's spend.
func TestScanConfigRoot_MultiTurnTranscript_UsageStaysTheSum(t *testing.T) {
	const artifact = "/ws/cycle-1455/scout-report.md"
	root := multiTurnFixture(t, artifact, inflatedTurns())

	res, err := ScanConfigRoot(root, Window{Driver: "claude-tmux", ArtifactPath: artifact})
	if err != nil {
		t.Fatalf("ScanConfigRoot: %v", err)
	}
	want := cyclestate.TokenUsage{Input: 8_000, Output: 6_000, CacheRead: 358_000, CacheWrite: 14_000}
	if res.Usage != want {
		t.Errorf("Usage = %+v, want %+v — the summed total is the COST figure and must survive the fill%% fix untouched", res.Usage, want)
	}
}

// TestFillWarn_OverHundredPercent_StaysLegible is the negative/edge case the
// eval pins as `over-100-still-legible`. A genuine single-turn overrun (240_000
// prompt-side tokens against the deliberately-conservative 200K effective
// window) is a REAL signal — fillpct.go promises over-full readings are not
// clamped. The fix must remove the summation inflation without also flattening
// honest overruns to 100%.
func TestFillWarn_OverHundredPercent_StaysLegible(t *testing.T) {
	const artifact = "/ws/cycle-1455/build-report.md"
	root := multiTurnFixture(t, artifact, []string{
		assistantTurn("m1", 10_000, 500, 100_000, 0),
		assistantTurn("m2", 5_000, 800, 235_000, 0),
	})

	got, err := DefaultResolver(root)(Window{Driver: "claude-tmux", ArtifactPath: artifact})
	if err != nil {
		t.Fatalf("resolver returned error: %v", err)
	}
	if math.Abs(got.FillPct-120) > pctTolerance {
		t.Fatalf("FillPct = %v, want 120 (terminal turn 240000 / 200000) — an honest overrun must be neither summed up to 175%% nor clamped down to 100%%", got.FillPct)
	}
	warn := FillWarn("build", got.FillPct, 60)
	if warn == "" {
		t.Fatalf("FillWarn on a genuine 120%% overrun is silent — the one launch that really is past the window must warn")
	}
	if !strings.Contains(warn, "120.0") {
		t.Errorf("warn %q does not carry the real reading (120.0%%) — a clamped or rounded-away percentage tells the operator nothing about how far past the window the launch is", warn)
	}
	if !strings.Contains(warn, "build") {
		t.Errorf("warn %q does not name the phase — an unattributed fill WARN is unactionable", warn)
	}
}

// TestFillWarn_CorrectedFixtureDoesNotFalsePositive is the anti-false-positive
// edge case. Three modest turns (42_000 / 62_000 / 94_000) sum to 198_000 —
// 99%, comfortably past the 60% warn line — while the terminal turn is only
// 47%. Today this launch warns spuriously; after the fix it must be silent.
func TestFillWarn_CorrectedFixtureDoesNotFalsePositive(t *testing.T) {
	const artifact = "/ws/cycle-1455/audit-report.md"
	root := multiTurnFixture(t, artifact, []string{
		assistantTurn("m1", 2_000, 300, 40_000, 0),
		assistantTurn("m2", 2_000, 300, 60_000, 0),
		assistantTurn("m3", 4_000, 300, 90_000, 0),
	})

	got, err := DefaultResolver(root)(Window{Driver: "claude-tmux", ArtifactPath: artifact})
	if err != nil {
		t.Fatalf("resolver returned error: %v", err)
	}
	if math.Abs(got.FillPct-47) > pctTolerance {
		t.Fatalf("FillPct = %v, want 47 (terminal turn 94000 / 200000; the summed artefact is 99)", got.FillPct)
	}
	if warn := FillWarn("audit", got.FillPct, 60); warn != "" {
		t.Errorf("FillWarn fired on a 47%%-full launch: %q — the summed reading crossed the 60%% line, the real one never did", warn)
	}
}

// TestFillPct_ZeroObservedTurns_IsUnmeasured is the sentinel-preservation edge
// (scout Key Finding #3). A transcript that attributes to the launch but whose
// assistant turns all fall OUTSIDE the launch window observed no context at
// all. Reading that as a measured 0% would make the launch look like an empty
// context forever; it must degrade to the documented negative sentinel, in the
// same vocabulary the uncovered-driver path already uses.
func TestFillPct_ZeroObservedTurns_IsUnmeasured(t *testing.T) {
	const artifact = "/ws/cycle-1455/test-report.md"
	root := t.TempDir()
	body := `{"type":"user","message":{"id":"u1","content":"Artifact path: ` + artifact + `"}}` + "\n" +
		`{"type":"assistant","timestamp":"2026-07-07T23:59:00Z","message":{"id":"m1","usage":{"input_tokens":90000,"output_tokens":100,"cache_read_input_tokens":10000,"cache_creation_input_tokens":0}}}` + "\n"
	writeTranscript(t, filepath.Join(root, "projects", "-some-worktree"), "sess.jsonl", body)

	got, err := DefaultResolver(root)(Window{
		Driver:       "claude-tmux",
		ArtifactPath: artifact,
		Start:        mustParse(t, launchWindowStart),
		End:          mustParse(t, launchWindowEnd),
	})
	if err != nil {
		t.Fatalf("resolver returned error: %v", err)
	}
	if got.FillPct != FillPctUnmeasured {
		t.Errorf("FillPct = %v, want FillPctUnmeasured (%v) — zero in-window turns means nothing observed the context, not that the context was empty",
			got.FillPct, FillPctUnmeasured)
	}
	if warn := FillWarn("tdd", got.FillPct, 60); warn != "" {
		t.Errorf("FillWarn on the unmeasured sentinel = %q, want silence", warn)
	}
}
