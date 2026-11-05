package wave_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func rangeRoot(t *testing.T, cycle string) (string, string) {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", cycle)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, ws
}

func denyRead(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o755) })
	if f, err := os.Open(path); err == nil {
		_ = f.Close()
		t.Fatalf("%s is still readable after chmod 0: the fixture cannot prove a read failure as this user", path)
	}
}

func writeRangeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func failurePaths(r wave.Range) []string {
	out := make([]string, 0, len(r.Failures))
	for _, f := range r.Failures {
		out = append(out, f.Path)
	}
	return out
}

func wantFailureAt(t *testing.T, r wave.Range, path string) {
	t.Helper()
	for _, f := range r.Failures {
		if f.Path == path && f.Err != nil && strings.Contains(f.Error(), path) {
			return
		}
	}
	t.Errorf("failures %v, want one at %s that names the path in its text", failurePaths(r), path)
}

func TestReadRunRange_ReportsAnUnreadableRunsDirectory(t *testing.T) {
	t.Parallel()
	root, _ := rangeRoot(t, "cycle-5")
	runs := filepath.Join(root, ".evolve", "runs")
	denyRead(t, runs)

	r := wave.ReadRunRange(root, 0, 0)

	wantFailureAt(t, r, runs)
}

func TestReadRunRange_ReportsAnUnreadableLandingDirectory(t *testing.T) {
	t.Parallel()
	root, _ := rangeRoot(t, "cycle-5")
	landing := filepath.Join(root, ".evolve", "landing")
	if err := os.MkdirAll(landing, 0o755); err != nil {
		t.Fatal(err)
	}
	denyRead(t, landing)

	r := wave.ReadRunRange(root, 0, 0)

	wantFailureAt(t, r, landing)
}

func TestReadRunRange_ReportsAnUnreadableLaneScope(t *testing.T) {
	t.Parallel()
	root, ws := rangeRoot(t, "cycle-5")
	scope := filepath.Join(ws, "lane-scope.json")
	writeRangeFile(t, scope, `{"todo_ids":["a-todo"]}`)
	denyRead(t, scope)

	r := wave.ReadRunRange(root, 0, 0)

	wantFailureAt(t, r, scope)
}

func TestReadRunRange_ReportsACorruptLaneScope(t *testing.T) {
	t.Parallel()
	root, ws := rangeRoot(t, "cycle-5")
	scope := filepath.Join(ws, "lane-scope.json")
	writeRangeFile(t, scope, `{not json`)

	r := wave.ReadRunRange(root, 0, 0)

	wantFailureAt(t, r, scope)
}

func TestReadRunRange_ReportsAnUnreadableModelAttemptLedger(t *testing.T) {
	t.Parallel()
	root, ws := rangeRoot(t, "cycle-5")
	ledger := llmcalls.Path(ws)
	writeRangeFile(t, ledger, "")
	denyRead(t, ledger)

	r := wave.ReadRunRange(root, 0, 0)

	wantFailureAt(t, r, ledger)
}

func TestReadRunRange_ReportsAnUnreadableLoopSignalStream(t *testing.T) {
	t.Parallel()
	root, _ := rangeRoot(t, "cycle-5")
	stream := filepath.Join(root, ".evolve", signalcenter.StreamFileName)
	writeRangeFile(t, stream, "")
	denyRead(t, stream)

	r := wave.ReadRunRange(root, 0, 0)

	wantFailureAt(t, r, stream)
}

func TestReadRunRange_ReportsNothingForAHealthyTreeAndForAbsentFiles(t *testing.T) {
	t.Parallel()
	root, ws := rangeRoot(t, "cycle-5")
	writeRangeFile(t, filepath.Join(ws, "lane-scope.json"), `{"todo_ids":["a-todo"]}`)

	r := wave.ReadRunRange(root, 0, 0)

	if len(r.Failures) != 0 {
		t.Errorf("failures = %v, want none: an absent file is not a failure", failurePaths(r))
	}
	if len(r.Cycles) != 1 || r.Cycles[0].ID != 5 || r.Cycles[0].Todo != "a-todo" {
		t.Errorf("cycles = %+v, want cycle 5 with todo a-todo", r.Cycles)
	}
}

func TestReadCycles_SharesTheFailuresAsWarnings(t *testing.T) {
	t.Parallel()
	root, ws := rangeRoot(t, "cycle-5")
	scope := filepath.Join(ws, "lane-scope.json")
	writeRangeFile(t, scope, `{not json`)

	_, warnings := wave.ReadCycles(root, 0, 0)

	if !strings.Contains(strings.Join(warnings, "\n"), scope) {
		t.Errorf("warnings = %v, want one that names %s", warnings, scope)
	}
}

func TestReadRunRange_ReportsACorruptDossierUnderKnowledgeBase(t *testing.T) {
	t.Parallel()
	root, _ := rangeRoot(t, "cycle-5")
	dir := filepath.Join(root, "knowledge-base", "cycles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "cycle-6.json")
	writeRangeFile(t, bad, `{not json`)

	r := wave.ReadRunRange(root, 0, 0)

	wantFailureAt(t, r, bad)
}

func TestReadRunRange_ReportsAnUnreadableCycleState(t *testing.T) {
	t.Parallel()
	root, ws := rangeRoot(t, "cycle-5")
	state := filepath.Join(ws, "cycle-state.json")
	writeRangeFile(t, state, `{"cycle_id":5}`)
	denyRead(t, state)

	r := wave.ReadRunRange(root, 0, 0)

	wantFailureAt(t, r, state)
}

func TestReadRunRange_ReportsACorruptCycleState(t *testing.T) {
	t.Parallel()
	root, ws := rangeRoot(t, "cycle-5")
	state := filepath.Join(ws, "cycle-state.json")
	writeRangeFile(t, state, `{`)

	r := wave.ReadRunRange(root, 0, 0)

	wantFailureAt(t, r, state)
}

func TestReadRunRange_ReportsACorruptCycleStateWhenTheRunStateIsGood(t *testing.T) {
	t.Parallel()
	root, ws := rangeRoot(t, "cycle-5")
	state := filepath.Join(ws, "cycle-state.json")
	writeRangeFile(t, state, `{`)
	writeRangeFile(t, filepath.Join(ws, "run.json"), `{"cycle_id":5}`)

	r := wave.ReadRunRange(root, 0, 0)

	wantFailureAt(t, r, state)
}

func TestReadRunRange_ReportsAnUnreadableLiveCycleState(t *testing.T) {
	t.Parallel()
	root, _ := rangeRoot(t, "cycle-5")
	live := filepath.Join(root, ".evolve", "cycle-state.json")
	writeRangeFile(t, live, `{"cycle_id":5}`)
	denyRead(t, live)

	r := wave.ReadRunRange(root, 0, 0)

	wantFailureAt(t, r, live)
}

func TestReadRunRange_ReportsAnUnreadableSignalStreamOfACycle(t *testing.T) {
	t.Parallel()
	root, ws := rangeRoot(t, "cycle-5")
	stream := filepath.Join(ws, signalcenter.StreamFileName)
	writeRangeFile(t, stream, "")
	denyRead(t, stream)

	r := wave.ReadRunRange(root, 0, 0)

	wantFailureAt(t, r, stream)
}

func TestReadRunRange_ReportsAnUnreadableLandingIntent(t *testing.T) {
	t.Parallel()
	root, _ := rangeRoot(t, "cycle-5")
	intent := landing.IntentPath(filepath.Join(root, ".evolve"), 5)
	if err := os.MkdirAll(filepath.Dir(intent), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRangeFile(t, intent, `{}`)
	denyRead(t, intent)

	r := wave.ReadRunRange(root, 0, 0)

	wantFailureAt(t, r, intent)
}

func TestReadRunRange_AnUnreadableLedgerIsOneFailureNotTwo(t *testing.T) {
	t.Parallel()
	root, ws := rangeRoot(t, "cycle-5")
	ledger := llmcalls.Path(ws)
	writeRangeFile(t, ledger, "")
	denyRead(t, ledger)

	r := wave.ReadRunRange(root, 0, 0)
	_, _, warnings := wave.ReadRunCycles(root, 0, 0)

	if len(r.Failures) != 1 || len(warnings) != 1 {
		t.Errorf("failures %v, warnings %v, want one of each: one read failed once", failurePaths(r), warnings)
	}
}

func TestReadRunRange_AMalformedSignalLineIsAWarningNotAFailure(t *testing.T) {
	t.Parallel()
	root, ws := rangeRoot(t, "cycle-5")
	writeRangeFile(t, filepath.Join(ws, signalcenter.StreamFileName), "{torn\n")

	r := wave.ReadRunRange(root, 0, 0)

	if len(r.Failures) != 0 || len(r.Warnings) != 1 {
		t.Errorf("failures %v, warnings %v, want no failure and one warning", failurePaths(r), r.Warnings)
	}
}

func TestReadRunCycles_ADeniedRunsDirectoryReachesTheWarningsEvenWhenAFileUnderItIsNamed(t *testing.T) {
	t.Parallel()
	root, _ := rangeRoot(t, "cycle-5")
	runs := filepath.Join(root, ".evolve", "runs")
	intent := landing.IntentPath(filepath.Join(root, ".evolve"), 5)
	if err := os.MkdirAll(filepath.Dir(intent), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRangeFile(t, intent, `{}`)
	denyRead(t, runs)

	_, _, warnings := wave.ReadRunCycles(root, 0, 0)

	for _, w := range warnings {
		if strings.HasPrefix(w, runs+": ") {
			return
		}
	}
	t.Errorf("warnings = %v, want one that starts with %s: ", warnings, runs)
}

func TestReadRunRange_ACorruptDossierAboveTheCeilingIsNotAFailureOfTheRange(t *testing.T) {
	t.Parallel()
	root, _ := rangeRoot(t, "cycle-2")
	dir := filepath.Join(root, "knowledge-base", "cycles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeRangeFile(t, filepath.Join(dir, "cycle-90.json"), `{not json`)

	r := wave.ReadRunRange(root, 0, 3)
	_, warnings := wave.ReadCycles(root, 0, 3)

	if len(r.Failures) != 0 || len(warnings) != 0 {
		t.Errorf("failures %v, warnings %v, want none: cycle 90 is outside the window 1..3", failurePaths(r), warnings)
	}
}

func TestReadRunRange_ACorruptDossierAtTheFloorIsNotAFailureOfTheRange(t *testing.T) {
	t.Parallel()
	root, _ := rangeRoot(t, "cycle-6")
	dir := filepath.Join(root, "knowledge-base", "cycles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeRangeFile(t, filepath.Join(dir, "cycle-5.json"), `{not json`)

	r := wave.ReadRunRange(root, 5, 0)

	if len(r.Failures) != 0 {
		t.Errorf("failures %v, want none: the floor 5 is outside the window, which starts at cycle 6", failurePaths(r))
	}
}

func TestReadRunRange_ATornLoopStreamKeepsItsGoodEvents(t *testing.T) {
	t.Parallel()
	root, _ := rangeRoot(t, "cycle-5")
	stream := filepath.Join(root, ".evolve", signalcenter.StreamFileName)
	writeRangeFile(t, stream, eventAt(1, "ledger.appended", "", "")+"{torn\n")

	r := wave.ReadRunRange(root, 0, 0)

	if len(r.Loop) != 1 || len(r.Warnings) != 1 {
		t.Errorf("loop %v, warnings %v, want one event and one warning: a torn line must not lose the good lines", r.Loop, r.Warnings)
	}
}

func TestReadRunCycles_ACorruptLaneScopeReachesTheWarnings(t *testing.T) {
	t.Parallel()
	root, ws := rangeRoot(t, "cycle-5")
	scope := filepath.Join(ws, "lane-scope.json")
	writeRangeFile(t, scope, `{not json`)

	_, _, warnings := wave.ReadRunCycles(root, 0, 0)

	if !strings.Contains(strings.Join(warnings, "\n"), scope) {
		t.Errorf("warnings = %v, want one that names %s", warnings, scope)
	}
}

func TestReadRunRange_TheTodoOfACycleIsTheFirstIDOfItsScope(t *testing.T) {
	t.Parallel()
	root, ws := rangeRoot(t, "cycle-5")
	writeRangeFile(t, filepath.Join(ws, "lane-scope.json"), `{"todo_ids":["first-todo","second-todo"]}`)

	r := wave.ReadRunRange(root, 0, 0)

	if len(r.Cycles) != 1 || r.Cycles[0].Todo != "first-todo" {
		t.Errorf("cycles = %+v, want todo first-todo", r.Cycles)
	}
}
