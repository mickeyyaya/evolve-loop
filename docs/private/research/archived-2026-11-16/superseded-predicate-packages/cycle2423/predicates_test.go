//go:build acs

package cycle2423

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	designDoc   = "docs/architecture/signal-center-design.md"
	packageNote = "docs/architecture/packages/internal-signalcenter.md"
	expectedSrc = "go/internal/signalcenter/expected.go"
	expectedTst = "go/internal/signalcenter/expected_test.go"
	buildCycle  = 2365
	buildPhase  = "build"
	buildRun    = "run-2365-build"
)

var startedAt = time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

type rig struct {
	clock    *clock
	path     string
	producer *signalcenter.Center
	listener *signalcenter.ExpectedSignals
	warnings func() []signalcenter.Event
}

func deadlines(key string) (time.Duration, bool) {
	switch key {
	case "expected_signal.phase_seconds":
		return 90 * time.Minute, true
	case "expected_signal.wave_seconds":
		return 10 * time.Minute, true
	}
	return 0, false
}

func newRig(t *testing.T) *rig {
	t.Helper()
	r := &rig{clock: &clock{t: startedAt}, path: filepath.Join(t.TempDir(), signalcenter.StreamFileName)}
	r.producer = signalcenter.New(signalcenter.WithClock(r.clock.now))
	r.producer.Subscribe(r.producer.NDJSONSink(func(int) string { return r.path }))
	loop := signalcenter.New(signalcenter.WithClock(r.clock.now))
	loop.Subscribe(loop.NDJSONSink(func(int) string { return r.path }))
	var mu sync.Mutex
	var seen []signalcenter.Event
	loop.Subscribe(func(e signalcenter.Event) {
		if e.Kind == signalcenter.KindExpectedAbsent {
			mu.Lock()
			seen = append(seen, e)
			mu.Unlock()
		}
	})
	r.warnings = func() []signalcenter.Event {
		mu.Lock()
		defer mu.Unlock()
		return append([]signalcenter.Event(nil), seen...)
	}
	r.listener = signalcenter.NewExpectedSignals(loop, signalcenter.DefaultExpectedPairs(), deadlines, r.clock.now)
	return r
}

func (r *rig) poll(t *testing.T) {
	t.Helper()
	if err := r.listener.Poll(r.path); err != nil {
		t.Fatalf("Poll: %v", err)
	}
}

func phaseEvent(kind signalcenter.Kind, origin string, cycle int, phase, run string) signalcenter.Event {
	return signalcenter.Event{Module: signalcenter.ModuleOrchestrator, Origin: origin, Kind: kind, Severity: signalcenter.SeverityInfo, Reason: "r", Cycle: cycle, Phase: phase, RunID: run}
}

func dispatch(cycle int, phase, run string) signalcenter.Event {
	return phaseEvent(signalcenter.KindPhaseDispatched, signalcenter.OriginPhaseDispatched, cycle, phase, run)
}

func outcome(cycle int, phase, run string) signalcenter.Event {
	return phaseEvent(signalcenter.KindPhaseOutcome, signalcenter.OriginPhaseOutcome, cycle, phase, run)
}

func absentBuildPhase(t *testing.T) (*rig, signalcenter.Event) {
	t.Helper()
	r := newRig(t)
	r.producer.Emit(dispatch(buildCycle, buildPhase, buildRun))
	r.clock.advance(91 * time.Minute)
	r.poll(t)
	got := r.warnings()
	if len(got) != 1 {
		t.Fatalf("RED: want exactly one SIGNAL_EXPECTED_ABSENT warning, got %d: %+v", len(got), got)
	}
	return r, got[0]
}

func startText() string { return startedAt.UTC().Format(time.RFC3339Nano) }

func TestC2423_001_TheReasonOfAnAbsentPhaseEndNamesItsCyclePhaseRunIDAndStartTime(t *testing.T) {
	_, w := absentBuildPhase(t)
	for _, need := range []string{fmt.Sprint(buildCycle), buildPhase, buildRun, startText()} {
		if !strings.Contains(w.Reason, need) {
			t.Errorf("RED: reason %q does not name %q", w.Reason, need)
		}
	}
	if w.Code != signalcenter.CodeExpectedAbsent || w.Severity != signalcenter.SeverityWarn {
		t.Errorf("the warning is %s %s, want WARN %s", w.Severity, w.Code, signalcenter.CodeExpectedAbsent)
	}
}

func TestC2423_002_TheFieldsOfAnAbsentPhaseEndNameItsCyclePhaseRunIDAndStartTime(t *testing.T) {
	_, w := absentBuildPhase(t)
	want := map[string]string{
		"cycle": fmt.Sprint(buildCycle), "phase": buildPhase, "run_id": buildRun, "started": startText(),
		"pair": "phase", "start_kind": "phase.dispatched", "end_kind": "phase.outcome,phase.aborted",
	}
	for key, value := range want {
		if w.Fields[key] != value {
			t.Errorf("RED: fields[%q] = %q, want %q (fields: %v)", key, w.Fields[key], value, w.Fields)
		}
	}
}

func TestC2423_003_TheWarningEventCarriesTheCyclePhaseAndRunIDOfItsStart(t *testing.T) {
	_, w := absentBuildPhase(t)
	if w.Cycle != buildCycle || w.Phase != buildPhase || w.RunID != buildRun {
		t.Errorf("Event cycle/phase/run = %d/%q/%q, want %d/%q/%q", w.Cycle, w.Phase, w.RunID, buildCycle, buildPhase, buildRun)
	}
}

func TestC2423_004_ThePersistedWarningInTheStreamNamesTheSameCyclePhaseAndRunID(t *testing.T) {
	r, _ := absentBuildPhase(t)
	chunk, err := signalcenter.ReadStream(r.path, 0)
	if err != nil {
		t.Fatal(err)
	}
	var persisted []signalcenter.Event
	for _, e := range chunk.Events {
		if e.Kind == signalcenter.KindExpectedAbsent {
			persisted = append(persisted, e)
		}
	}
	if len(persisted) != 1 {
		t.Fatalf("RED: the stream holds %d absent warnings, want 1", len(persisted))
	}
	e := persisted[0]
	if e.Cycle != buildCycle || e.Fields["cycle"] != fmt.Sprint(buildCycle) || e.Fields["phase"] != buildPhase || e.Fields["run_id"] != buildRun {
		t.Errorf("RED: persisted warning cycle=%d fields=%v", e.Cycle, e.Fields)
	}
	if !strings.Contains(e.Reason, buildRun) {
		t.Errorf("RED: persisted reason %q does not name the run id", e.Reason)
	}
}

func TestC2423_005_AWaveWarningNamesItsRunIDAndStartTimeAndNoInventedCycle(t *testing.T) {
	r := newRig(t)
	r.producer.Emit(signalcenter.Event{Module: signalcenter.ModuleLoop, Origin: "Test.emit", Kind: signalcenter.KindWaveStarted, Severity: signalcenter.SeverityInfo, Reason: "r", Fields: map[string]string{"run_id": "wave-17"}})
	r.clock.advance(11 * time.Minute)
	r.poll(t)
	got := r.warnings()
	if len(got) != 1 {
		t.Fatalf("want one wave warning, got %d: %+v", len(got), got)
	}
	w := got[0]
	if w.Fields["run_id"] != "wave-17" || !strings.Contains(w.Reason, "wave-17") || !strings.Contains(w.Reason, startText()) {
		t.Errorf("RED: wave warning reason=%q fields=%v does not name run id wave-17 and the start time", w.Reason, w.Fields)
	}
	if strings.Contains(w.Reason, "cycle 0") || strings.Contains(w.Reason, "cycle=0") {
		t.Errorf("a cycle-less wave warning must not name a cycle: %q", w.Reason)
	}
}

func TestC2423_006_AnEndInAnotherCycleDoesNotCloseTheStartAndTheWarningNamesTheOpenCycle(t *testing.T) {
	r := newRig(t)
	r.producer.Emit(dispatch(buildCycle, buildPhase, buildRun))
	r.producer.Emit(dispatch(buildCycle+1, buildPhase, "run-other"))
	r.producer.Emit(outcome(buildCycle+1, buildPhase, "run-other"))
	r.clock.advance(91 * time.Minute)
	r.poll(t)
	got := r.warnings()
	if len(got) != 1 {
		t.Fatalf("want one warning for the one open cycle, got %d: %+v", len(got), got)
	}
	if got[0].Fields["cycle"] != fmt.Sprint(buildCycle) || strings.Contains(got[0].Reason, "run-other") {
		t.Errorf("RED: warning names the wrong start: reason=%q fields=%v", got[0].Reason, got[0].Fields)
	}
}

func TestC2423_007_AnEndBeforeTheDeadlineGivesNoWarning(t *testing.T) {
	r := newRig(t)
	r.producer.Emit(dispatch(buildCycle, buildPhase, buildRun))
	r.clock.advance(30 * time.Minute)
	r.producer.Emit(outcome(buildCycle, buildPhase, buildRun))
	r.clock.advance(2 * time.Hour)
	r.poll(t)
	if got := r.warnings(); len(got) != 0 {
		t.Errorf("a closed start warned: %+v", got)
	}
}

func parseFile(t *testing.T, path string, mode parser.Mode) *ast.File {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, mode)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return parsed
}

func TestC2423_008_EachFunctionOfExpectedGoIsFullyCovered(t *testing.T) {
	root := acsassert.RepoRoot(t)
	moduleDir, err := acsassert.ModuleDir(filepath.Join(root, "go"))
	if err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(t.TempDir(), "cover.out")
	test := exec.Command("go", "test", "-count=1", "-coverprofile", profile, "./internal/signalcenter")
	test.Dir = moduleDir
	if out, err := test.CombinedOutput(); err != nil {
		t.Fatalf("go test ./internal/signalcenter: %v\n%s", err, acsassert.OutputTail(string(out), 30))
	}
	fn := exec.Command("go", "tool", "cover", "-func", profile)
	fn.Dir = moduleDir
	report, err := fn.CombinedOutput()
	if err != nil {
		t.Fatalf("go tool cover: %v\n%s", err, report)
	}
	parsed := parseFile(t, filepath.Join(root, expectedSrc), 0)
	checked := 0
	for _, decl := range parsed.Decls {
		f, ok := decl.(*ast.FuncDecl)
		if !ok || f.Name.Name == "init" {
			continue
		}
		checked++
		pct, err := acsassert.FuncCoverage(string(report), f.Name.Name)
		if err != nil {
			t.Errorf("%s: %v", f.Name.Name, err)
			continue
		}
		if pct < 100 {
			t.Errorf("%s coverage = %.1f%%, want 100%%", f.Name.Name, pct)
		}
	}
	if checked == 0 {
		t.Fatal("no function found in expected.go")
	}
}

func TestC2423_009_TheChangedGoFilesCarryNoComment(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range []string{expectedSrc, expectedTst} {
		parsed := parseFile(t, filepath.Join(root, rel), parser.ParseComments)
		for _, group := range parsed.Comments {
			for _, c := range group.List {
				t.Errorf("%s carries a comment: %q", rel, c.Text)
			}
		}
	}
}

func TestC2423_010_TheTwoDocumentsPassSteLintWithNoFinding(t *testing.T) {
	root := acsassert.RepoRoot(t)
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{designDoc, packageNote} {
		findings, err := stelint.LintFile(filepath.Join(root, rel), stelint.Options{Words: words})
		if err != nil {
			t.Fatal(err)
		}
		var real []string
		for _, f := range findings {
			if !f.IsSkipNotice() {
				real = append(real, fmt.Sprintf("line %d %s: %s", f.Line, f.Rule, f.Excerpt))
			}
		}
		if len(real) > 0 {
			t.Errorf("%s: %d ste-lint finding(s), first: %v", rel, len(real), real[:min(5, len(real))])
		}
	}
}

func blockNaming(t *testing.T, rel, section string, needs ...string) bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), rel))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if section != "" {
		_, rest, found := strings.Cut(text, section)
		if !found {
			t.Fatalf("%s: section %q is missing", rel, section)
		}
		text, _, _ = strings.Cut(rest, "\n## ")
	}
	for _, block := range strings.Split(text, "\n- ") {
		lower := strings.ToLower(block)
		all := true
		for _, need := range needs {
			if !strings.Contains(lower, need) {
				all = false
			}
		}
		if all {
			return true
		}
	}
	return false
}

// acs-predicate: config-check
func TestC2423_011_TheDesignSectionStatesWhichIdentityTheAbsentWarningCarries(t *testing.T) {
	if !blockNaming(t, designDoc, "### 8.1 Expected signals", "warning", "cycle", "phase", "run_id", "reason") {
		t.Errorf("RED: no item of %s 8.1 says that the warning reason and fields name the cycle, the phase and the run_id", designDoc)
	}
}

// acs-predicate: config-check
func TestC2423_012_ThePackageNoteStatesWhichIdentityTheAbsentWarningCarries(t *testing.T) {
	if !blockNaming(t, packageNote, "", "expected", "cycle", "phase", "run_id", "start") {
		t.Errorf("RED: no item of %s says that the absent warning names the cycle, the phase, the run_id and the start time", packageNote)
	}
}
