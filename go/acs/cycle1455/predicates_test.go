//go:build acs

// Package cycle1455 materialises the cycle-1455 acceptance criteria for the one
// fleet-scoped todo-id pinned to this lane, `context-fill-telemetry-and-cap` —
// specifically its live follow-on defect `contextfill-ratio-over-100pct`
// (inbox 2026-08-12, weight 0.75, pipeline-repair).
//
// The defect: `tokenusage.ScanConfigRoot` sums EVERY assistant turn's
// Input+CacheRead+CacheWrite into one grand total (scanner.go:147-153), and
// `DefaultResolver` feeds that summed total into `FillPct` against a
// SINGLE-turn 200K effective window (defaultresolver.go:38). Each turn's own
// cache_read_input_tokens already carries that turn's whole prior context, so
// summing turn N with turn N+1 re-counts the same context once per turn. Live
// symptom, twice in one monitored wave: scout 566.9%, triage 114.3%.
//
// Predicate strategy — every predicate below EXERCISES the system (the cycle-85
// degenerate-predicate ban); not one greps source:
//
//   - 001 drives the real production resolver over a real multi-turn transcript
//     fixture and asserts the reading is the terminal/peak turn's, naming the
//     summed (190) and first-turn (35) wrong answers explicitly.
//   - 002 is the anti-overfit half: `Result.Usage` is the COST number and
//     summing turns is CORRECT for it. A fix that greens 001 by making the
//     scanner stop summing reds 002.
//   - 003 is the negative case: an honest single-turn overrun must stay
//     unclamped and legible at 120%, not summed to 175% and not flattened to
//     100% (fillpct.go's own documented promise).
//   - 004 is the anti-false-positive edge: three modest turns sum past the 60%
//     warn line while the real reading is 47% — the WARN must go silent.
//   - 005 is the sentinel edge: zero in-window turns means nothing OBSERVED the
//     context; reading that as a measured 0% makes the launch look permanently
//     empty.
//   - 006 shells ONE named package (never a `./...` sweep, per the
//     flaky-predicate-shape rules) to pin no-regression across the existing
//     scanner/fillpct/defaultresolver/apicover suites.
//
// Every fixture grows monotonically — real transcripts do, context only
// accumulates within a phase — so the terminal turn IS the peak turn and either
// extraction satisfies these predicates. What they rule out is the sum, the
// first turn, and the mean.
package cycle1455

import (
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// claudeWindow is the conservative effective context window fillpct.go pins for
// the claude family; every percentage below is against it.
const claudeWindow = 200_000

// pctTolerance is the float comparison slack for percentage assertions.
const pctTolerance = 0.05

// assistantTurn renders one assistant transcript line with the four usage
// counters a real Claude Code transcript reports.
func assistantTurn(id string, input, output, cacheRead, cacheWrite int) string {
	return `{"type":"assistant","message":{"id":"` + id + `","usage":{` +
		`"input_tokens":` + strconv.Itoa(input) +
		`,"output_tokens":` + strconv.Itoa(output) +
		`,"cache_read_input_tokens":` + strconv.Itoa(cacheRead) +
		`,"cache_creation_input_tokens":` + strconv.Itoa(cacheWrite) + `}}}`
}

// transcriptFixture writes a config root holding one transcript whose first
// user message carries artifactPath (the primary attribution key) followed by
// the given assistant lines, and returns that root.
func transcriptFixture(t *testing.T, artifactPath string, turns []string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "projects", "-some-worktree")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	body := `{"type":"user","message":{"id":"u1","content":"Artifact path: ` + artifactPath + `"}}` + "\n" +
		strings.Join(turns, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "sess.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}
	return root
}

// inflatedTurns is the 566.9%-class reproducer: prompt-side totals of
// 70_000 / 130_000 / 180_000. Summed they are 380_000 (190% — the bug); the
// terminal (== peak) turn is 180_000, a plausible 90%.
func inflatedTurns() []string {
	return []string{
		assistantTurn("m1", 5_000, 1_000, 60_000, 5_000),
		assistantTurn("m2", 2_000, 2_000, 120_000, 8_000),
		assistantTurn("m3", 1_000, 3_000, 178_000, 1_000),
	}
}

// resolve drives the real production resolver — the same closure both
// composition roots (adapters/bridge and subagent) wire into
// gobridge.Deps.TokenResolver — over a fixture root.
func resolve(t *testing.T, root, artifactPath string, w tokenusage.Window) tokenusage.Result {
	t.Helper()
	w.Driver = "claude-tmux"
	w.ArtifactPath = artifactPath
	got, err := tokenusage.DefaultResolver(root)(w)
	if err != nil {
		t.Fatalf("resolver returned error (telemetry must be best-effort): %v", err)
	}
	return got
}

// TestC1455_001_FillPctIsOneTurnNotTheSumOfTurns is the load-bearing predicate.
// 90% is the terminal turn's own occupancy; 190% is the summed artefact this
// cycle removes.
func TestC1455_001_FillPctIsOneTurnNotTheSumOfTurns(t *testing.T) {
	const artifact = "/ws/cycle-1455/scout-report.md"
	got := resolve(t, transcriptFixture(t, artifact, inflatedTurns()), artifact, tokenusage.Window{})

	if got.Source != tokenusage.SourceTranscript {
		t.Fatalf("Source = %q, want %q — the fixture did not reach the transcript tier, so this predicate proved nothing",
			got.Source, tokenusage.SourceTranscript)
	}
	if math.Abs(got.FillPct-190) <= pctTolerance {
		t.Fatalf("FillPct = %v — the SUM of all three turns (380000/%d). Each turn's cache_read already carries that turn's whole prior context; summing turns re-counts it once per turn (live symptom: scout 566.9%%)",
			got.FillPct, claudeWindow)
	}
	if math.Abs(got.FillPct-35) <= pctTolerance {
		t.Fatalf("FillPct = %v — the FIRST turn (70000/%d), not the turn that shows how full the window ended up", got.FillPct, claudeWindow)
	}
	if math.Abs(got.FillPct-90) > pctTolerance {
		t.Errorf("FillPct = %v, want 90 (terminal/peak turn 180000 / %d)", got.FillPct, claudeWindow)
	}
}

// TestC1455_002_SummedUsageSurvivesForCostAccounting is the anti-overfit half:
// Result.Usage is the cost/spend figure and summing turns is correct for it. A
// fix that repairs fill% by making the scanner stop summing would silently
// under-report every cycle's spend.
func TestC1455_002_SummedUsageSurvivesForCostAccounting(t *testing.T) {
	const artifact = "/ws/cycle-1455/scout-report.md"
	root := transcriptFixture(t, artifact, inflatedTurns())

	res, err := tokenusage.ScanConfigRoot(root, tokenusage.Window{Driver: "claude-tmux", ArtifactPath: artifact})
	if err != nil {
		t.Fatalf("ScanConfigRoot: %v", err)
	}
	if res.Usage.Input != 8_000 || res.Usage.Output != 6_000 || res.Usage.CacheRead != 358_000 || res.Usage.CacheWrite != 14_000 {
		t.Errorf("Usage = %+v, want {Input:8000 Output:6000 CacheRead:358000 CacheWrite:14000} — the summed total is the COST figure and must survive the fill%% fix untouched", res.Usage)
	}
}

// TestC1455_003_HonestOverrunStaysUnclampedAndLegible is the negative case. A
// genuine single-turn overrun (240_000 prompt-side tokens against the
// deliberately-conservative 200K window) is a REAL signal — fillpct.go promises
// over-full readings are not clamped. Removing the summation inflation must not
// also flatten honest overruns to 100%.
func TestC1455_003_HonestOverrunStaysUnclampedAndLegible(t *testing.T) {
	const artifact = "/ws/cycle-1455/build-report.md"
	root := transcriptFixture(t, artifact, []string{
		assistantTurn("m1", 10_000, 500, 100_000, 0),
		assistantTurn("m2", 5_000, 800, 235_000, 0),
	})
	got := resolve(t, root, artifact, tokenusage.Window{})

	if math.Abs(got.FillPct-120) > pctTolerance {
		t.Fatalf("FillPct = %v, want 120 (terminal turn 240000 / %d) — an honest overrun must be neither summed up to 175%% nor clamped down to 100%%", got.FillPct, claudeWindow)
	}
	warn := tokenusage.FillWarn("build", got.FillPct, 60)
	if warn == "" {
		t.Fatalf("FillWarn on a genuine 120%% overrun is silent — the one launch that really is past the window must warn")
	}
	if !strings.Contains(warn, "120.0") {
		t.Errorf("warn %q omits the real reading (120.0%%) — a clamped or rounded-away percentage cannot tell an operator how far past the window a launch is", warn)
	}
	if !strings.Contains(warn, "build") {
		t.Errorf("warn %q does not name the phase — an unattributed fill WARN is unactionable", warn)
	}
}

// TestC1455_004_CorrectedReadingDoesNotFalsePositiveTheWarn is the
// anti-false-positive edge. Three modest turns (42_000 / 62_000 / 94_000) sum to
// 198_000 — 99%, well past the 60% warn line — while the terminal turn is only
// 47%. Today this launch warns spuriously; after the fix it must be silent.
func TestC1455_004_CorrectedReadingDoesNotFalsePositiveTheWarn(t *testing.T) {
	const artifact = "/ws/cycle-1455/audit-report.md"
	root := transcriptFixture(t, artifact, []string{
		assistantTurn("m1", 2_000, 300, 40_000, 0),
		assistantTurn("m2", 2_000, 300, 60_000, 0),
		assistantTurn("m3", 4_000, 300, 90_000, 0),
	})
	got := resolve(t, root, artifact, tokenusage.Window{})

	if math.Abs(got.FillPct-47) > pctTolerance {
		t.Fatalf("FillPct = %v, want 47 (terminal turn 94000 / %d; the summed artefact is 99)", got.FillPct, claudeWindow)
	}
	if warn := tokenusage.FillWarn("audit", got.FillPct, 60); warn != "" {
		t.Errorf("FillWarn fired on a 47%%-full launch: %q — the summed reading crossed the 60%% line, the real one never did", warn)
	}
}

// TestC1455_005_ZeroObservedTurnsDegradeToTheSentinel is the
// sentinel-preservation edge. A transcript that attributes to the launch but
// whose assistant turns all fall OUTSIDE the launch window observed no context
// at all — reading that as a measured 0% would make the launch look like an
// empty context forever. It must degrade to the documented negative sentinel,
// in the same vocabulary the uncovered-driver path already uses.
func TestC1455_005_ZeroObservedTurnsDegradeToTheSentinel(t *testing.T) {
	const artifact = "/ws/cycle-1455/test-report.md"
	root := transcriptFixture(t, artifact, []string{
		`{"type":"assistant","timestamp":"2026-07-07T23:59:00Z","message":{"id":"m1","usage":{"input_tokens":90000,"output_tokens":100,"cache_read_input_tokens":10000,"cache_creation_input_tokens":0}}}`,
	})
	start, err := time.Parse(time.RFC3339, "2026-07-07T10:00:00Z")
	if err != nil {
		t.Fatalf("parse window start: %v", err)
	}
	end, err := time.Parse(time.RFC3339, "2026-07-07T10:10:00Z")
	if err != nil {
		t.Fatalf("parse window end: %v", err)
	}
	got := resolve(t, root, artifact, tokenusage.Window{Start: start, End: end})

	if got.FillPct != tokenusage.FillPctUnmeasured {
		t.Errorf("FillPct = %v, want FillPctUnmeasured (%v) — zero in-window turns means nothing observed the context, not that the context was empty",
			got.FillPct, tokenusage.FillPctUnmeasured)
	}
	if warn := tokenusage.FillWarn("tdd", got.FillPct, 60); warn != "" {
		t.Errorf("FillWarn on the unmeasured sentinel = %q, want silence", warn)
	}
}

// TestC1455_006_TokenusageSuiteStaysGreen pins no-regression across the
// package's existing scanner / fillpct / defaultresolver / apicover suites —
// including the repo-wide apicover gate's named-symbol requirement, which bites
// if the fix introduces a new exported symbol without naming it in
// apicover_named_test.go (tokenusage is enrolled at .apicover-enforce:410).
// ONE named package, never a `./...` sweep (flaky-predicate-shape rules).
func TestC1455_006_TokenusageSuiteStaysGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", filepath.Join(root, "go"), "-count=1", "./internal/tokenusage")
	if err != nil && code == 0 {
		t.Fatalf("could not run the tokenusage suite: %v", err)
	}
	if code != 0 {
		t.Errorf("`go test ./internal/tokenusage` exit=%d — the fill%% fix regressed the package's existing suites\nstdout:\n%s\nstderr:\n%s",
			code, stdout, stderr)
	}
}
