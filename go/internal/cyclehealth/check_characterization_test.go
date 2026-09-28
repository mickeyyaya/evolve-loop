package cyclehealth

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheck_GeneratedAtComesFromTheClock(t *testing.T) {
	ws := freshWorkspace(t, 1)
	at := time.Unix(2000, 0).UTC()
	r, err := Check(Options{Cycle: 1, Workspace: ws, NowFn: func() time.Time { return at }})
	if err != nil {
		t.Fatal(err)
	}
	if !r.GeneratedAt.Equal(at) || !readReport(t, ws).GeneratedAt.Equal(at) {
		t.Fatalf("GeneratedAt = %v, want %v", r.GeneratedAt, at)
	}
}

func TestCheck_RunsTheDossierCommitmentSignal(t *testing.T) {
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-4")
	cycles := filepath.Join(root, "knowledge-base", "cycles")
	for _, dir := range []string{ws, cycles} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := json.Marshal(map[string]any{
		"cycle": 4, "run_id": "01JHISTORICALRUN000000000", "goal": "g", "final_verdict": "PASS",
		"tasks":  []string{},
		"phases": []map[string]any{{"name": "build", "verdict": "PASS", "duration_ms": 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cycles, "cycle-4.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Check(Options{Cycle: 4, Workspace: ws})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range r.Anomalies {
		if a.Signal == "dossier_commitment" && a.Severity == SeverityFatal {
			return
		}
	}
	t.Fatalf("no fatal dossier_commitment anomaly in %+v", r.Anomalies)
}

func TestCheck_ReportWriteFailureIsReturnedWithTheReport(t *testing.T) {
	ws := filepath.Join(t.TempDir(), "absent")
	r, err := Check(Options{Cycle: 3, Workspace: ws, NowFn: func() time.Time { return time.Unix(2000, 0) }})
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v, want the write failure wrapped", err)
	}
	if r.Cycle != 3 || r.Workspace != ws || len(r.SignalsRun) != len(signalNames()) {
		t.Fatalf("report = %+v, want the computed report alongside the error", r)
	}
}

func classifyFixture(t *testing.T, timing string, rollup string) (Outcome, string) {
	t.Helper()
	ws := t.TempDir()
	if timing != "" {
		writeTiming(t, ws, timing)
	}
	if rollup != "" {
		if err := os.WriteFile(filepath.Join(ws, "interaction-summary.json"), []byte(rollup), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return ClassifyOutcome(ws)
}

func TestClassifyOutcome_UnreadableTimingStillMeansPhasesRan(t *testing.T) {
	if got, detail := classifyFixture(t, "{not json", ""); got != OutcomeFailedUnexplained {
		t.Fatalf("outcome = %s (%q), want %s", got, detail, OutcomeFailedUnexplained)
	}
}

func TestClassifyOutcome_ShipFailIsNotShipped(t *testing.T) {
	if got, detail := classifyFixture(t, `[{"phase":"ship","verdict":"FAIL"}]`, ""); got != OutcomeFailedExplained {
		t.Fatalf("outcome = %s (%q), want %s", got, detail, OutcomeFailedExplained)
	}
}

func TestClassifyOutcome_SalvageNeedsBothCounters(t *testing.T) {
	timing := `[{"phase":"build","verdict":"PASS"}]`
	for _, rollup := range []string{
		`{"by_rung":{"salvage":1},"by_result":{}}`,
		`{"by_rung":{"retry":1},"by_result":{"artifact_appeared":1}}`,
	} {
		if got, detail := classifyFixture(t, timing, rollup); got != OutcomeFailedUnexplained {
			t.Fatalf("rollup %s: outcome = %s (%q), want %s", rollup, got, detail, OutcomeFailedUnexplained)
		}
	}
}

func TestClassifyOutcome_MalformedRollupIsNotSalvage(t *testing.T) {
	if got, detail := classifyFixture(t, `[{"phase":"build","verdict":"PASS"}]`, "{"); got != OutcomeFailedUnexplained {
		t.Fatalf("outcome = %s (%q), want %s", got, detail, OutcomeFailedUnexplained)
	}
}

func TestClassifyOutcome_SalvageDetailNamesEachCounter(t *testing.T) {
	got, detail := classifyFixture(t, `[{"phase":"build","verdict":"PASS"}]`, `{"by_rung":{"salvage":2},"by_result":{"artifact_appeared":5}}`)
	if got != OutcomeSalvaged || !strings.Contains(detail, "salvage=2, artifact_appeared=5") {
		t.Fatalf("outcome = %s detail = %q", got, detail)
	}
}

func TestClassifyOutcome_QuotaPrefixMustLeadTheReason(t *testing.T) {
	got, detail := classifyFixture(t, `[{"phase":"build","verdict":"FAIL","abort_reason":"phase build: all-families-exhausted later"}]`, "")
	if got != OutcomeFailedExplained {
		t.Fatalf("outcome = %s (%q), want %s", got, detail, OutcomeFailedExplained)
	}
}

func TestClassifyOutcome_FailDiagnosticsJoinWithSemicolons(t *testing.T) {
	timing := `[{"phase":"triage","verdict":"FAIL","diagnostics":[{"severity":"error","message":"first"},{"severity":"error","message":"second"}]}]`
	got, detail := classifyFixture(t, timing, "")
	if got != OutcomeFailedExplained || !strings.Contains(detail, "first; second") {
		t.Fatalf("outcome = %s detail = %q", got, detail)
	}
}
