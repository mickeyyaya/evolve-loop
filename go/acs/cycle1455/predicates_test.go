//go:build acs

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

const claudeWindow = 200_000

const pctTolerance = 0.05

func assistantTurn(id string, input, output, cacheRead, cacheWrite int) string {
	return `{"type":"assistant","message":{"id":"` + id + `","usage":{` +
		`"input_tokens":` + strconv.Itoa(input) +
		`,"output_tokens":` + strconv.Itoa(output) +
		`,"cache_read_input_tokens":` + strconv.Itoa(cacheRead) +
		`,"cache_creation_input_tokens":` + strconv.Itoa(cacheWrite) + `}}}`
}

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

func inflatedTurns() []string {
	return []string{
		assistantTurn("m1", 5_000, 1_000, 60_000, 5_000),
		assistantTurn("m2", 2_000, 2_000, 120_000, 8_000),
		assistantTurn("m3", 1_000, 3_000, 178_000, 1_000),
	}
}

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
