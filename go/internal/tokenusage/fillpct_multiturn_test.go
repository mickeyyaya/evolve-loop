package tokenusage

import (
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

const pctTolerance = 0.05

// multiTurnFixture's turns carry no timestamp, so withinWindow admits all of them.
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

func assistantTurn(id string, input, output, cacheRead, cacheWrite int) string {
	return `{"type":"assistant","message":{"id":"` + id + `","usage":{` +
		`"input_tokens":` + strconv.Itoa(input) +
		`,"output_tokens":` + strconv.Itoa(output) +
		`,"cache_read_input_tokens":` + strconv.Itoa(cacheRead) +
		`,"cache_creation_input_tokens":` + strconv.Itoa(cacheWrite) + `}}}`
}

// inflatedTurns' prompt sides are 70K, 130K and 180K: 190% of the window summed, 90% at the peak.
func inflatedTurns() []string {
	return []string{
		assistantTurn("m1", 5_000, 1_000, 60_000, 5_000),
		assistantTurn("m2", 2_000, 2_000, 120_000, 8_000),
		assistantTurn("m3", 1_000, 3_000, 178_000, 1_000),
	}
}

// The fill reads the peak turn; these fixtures grow monotonically, as real transcripts do, so the terminal turn is the peak.
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

// The only assistant turn is timestamped outside the launch window.
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
