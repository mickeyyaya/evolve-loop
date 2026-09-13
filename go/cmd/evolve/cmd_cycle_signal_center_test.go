// cmd_cycle_signal_center_test.go — ADR-0101 S1 wiring proofs. The production
// composition root constructs ONE Signal Center, attaches the durable NDJSON
// sink and the WARN-filtered stderr sink, and registers the orchestrator as a
// listener; the only roots allowed to build an orchestrator WITHOUT a Center
// are pinned here, so a forgotten wiring can never be silent again (the way
// bridge.Deps.LivenessCenter stayed unset in production).
package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/failurediag"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestWireOrchestratorDeps_SignalCenterWired(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	d := wireOrchestratorDeps(root, evolveDir, io.Discard)
	if d.Signals == nil || !d.Orchestrator.SignalCenterWired() {
		t.Fatal("the production composition root must construct a Signal Center and register the orchestrator as its listener (ADR-0101 S1)")
	}
	d.Signals.Emit(signalcenter.Event{
		Cycle: 1, Module: signalcenter.ModuleLoop, Origin: "Test.wired", Kind: signalcenter.KindLoopWave,
		Severity: signalcenter.SeverityInfo, Reason: "wiring proof",
	})
	data, err := os.ReadFile(filepath.Join(core.RunWorkspacePath(root, 1), "signals.ndjson"))
	if err != nil || !strings.Contains(string(data), `"reason":"wiring proof"`) {
		t.Errorf("the durable sink must be attached at the root and resolve the cycle workspace: %v %s", err, data)
	}
	if s := d.Orchestrator.SignalSummary(); s.Cycle != 1 || s.Total != 1 {
		t.Errorf("the orchestrator listener must observe root-emitted events: %+v", s)
	}
}

// Every production call site of core.NewOrchestrator must pass
// core.WithSignalCenter, except the one pinned nil root: the routing-test
// engine, test machinery that takes a *testing.T. The --simulate root builds
// the production topology since unit 01 (ADR-0103);
// TestWireSimulateOrchestrator_SignalCenterWired proves it.
func TestNilSignalCenterRootsArePinned(t *testing.T) {
	allowed := map[string]bool{
		"internal/routingtest/engine.go": true,
	}
	callRE := regexp.MustCompile(`\bcore\.NewOrchestrator\(`)
	moduleRoot := filepath.Join("..", "..")
	var unwired []string
	err := filepath.WalkDir(moduleRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if name == "vendor" || name == "bin" || strings.HasPrefix(name, ".") && path != moduleRoot {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		// Every construction in a file needs its own wiring: a second, unwired
		// core.NewOrchestrator( beside a wired one is caught by the count.
		calls := len(callRE.FindAll(src, -1))
		if calls == 0 || strings.Count(string(src), "WithSignalCenter(") >= calls {
			return nil
		}
		rel, _ := filepath.Rel(moduleRoot, path)
		if !allowed[filepath.ToSlash(rel)] {
			unwired = append(unwired, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(unwired) != 0 {
		t.Errorf("orchestrator built without a Signal Center outside the pinned nil roots: %v", unwired)
	}
}

// The console listener is WARN-filtered (severity contract: INFO is "log only",
// it lives in signals.ndjson); a green cycle prints nothing, a WARN prints the
// one line format. os.Stderr is captured around the wiring because the sink
// binds the writer at construction.
func TestWireOrchestratorDeps_SignalCenterConsoleSinkIsFilteredAtWarn(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stderr := captureConsole(func(console io.Writer) {
		d := wireOrchestratorDeps(root, evolveDir, console)
		d.Signals.Emit(signalcenter.Event{Cycle: 2, Module: signalcenter.ModuleLoop, Origin: "Test.filtered", Kind: signalcenter.KindLoopWave, Severity: signalcenter.SeverityInfo, Reason: "info stays in the file"})
		d.Signals.Emit(signalcenter.Event{Cycle: 2, Phase: "triage", Attempt: 1, Module: signalcenter.ModuleOrchestrator, Origin: "Test.filtered", Kind: signalcenter.KindPhaseOutcome, Severity: signalcenter.SeverityWarn, Code: core.CodePhaseVerdictFail, Reason: "triage verdict=FAIL: warn reaches the console"})
	})
	if strings.Contains(stderr, "info stays in the file") {
		t.Errorf("INFO must not reach the console sink:\n%s", stderr)
	}
	if !strings.Contains(stderr, "[orchestrator] phase.outcome WARN ORCHESTRATOR_PHASE_VERDICT_FAIL cycle=2 phase=triage attempt=1") || !strings.Contains(stderr, "warn reaches the console") {
		t.Errorf("WARN prints the one line format on the console:\n%s", stderr)
	}
}

// captureConsole runs fn with a buffer as the root's console writer and
// returns what the Signal Center's WARN-filtered sink rendered into it.
func captureConsole(fn func(console io.Writer)) string {
	var buf bytes.Buffer
	fn(&buf)
	return buf.String()
}

// The signalcenter package's own registry test is blind to producer modules
// (it is a leaf); this binary links every module, so THIS is where "the real
// registry is clean" is asserted (architecture review, S1).
func TestSignalCenterRegistry_EveryLinkedModuleRegistersCleanly(t *testing.T) {
	if conflicts := signalcenter.RegistryConflicts(); len(conflicts) != 0 {
		t.Errorf("code registry conflicts across linked modules: %+v", conflicts)
	}
	if _, ok := signalcenter.IsRegistered(core.CodePhaseVerdictFail); !ok {
		t.Error("core's codes must be registered by the time the binary is linked")
	}
}

// Both production roots flush the Center before they return (S2a): an Emit
// that finds a drain in progress returns before delivery, so an exit path
// without a Flush could lose the last events of a cycle or a batch.
func TestSignalCenterFlush_IsWiredAtBothRoots(t *testing.T) {
	for _, file := range []string{"cmd_cycle.go", "cmd_loop.go"} {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(src), "Signals.Flush()") {
			t.Errorf("%s: the root must defer Signals.Flush() after wiring the orchestrator", file)
		}
	}
}

// ADR-0101 S3: the production root hands the Center to the bridge Adapter it
// injects into every phase runner, so bridge.warning / bridge.tripwire /
// pane.liveness from any dispatch reach the orchestrator's Center.
func TestWireOrchestratorDeps_SignalCenterReachesTheBridge(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	d := wireOrchestratorDeps(root, evolveDir, io.Discard)
	if d.Bridge == nil || !d.Bridge.SignalsWired() {
		t.Fatal("the production bridge Adapter must be built with the Signal Center (bridge.NewDefault(projectRoot, signals))")
	}
}

// Every bridge.NewDefault call site outside the production root passes an
// explicit nil — the Center-less registry defaults are visible, never
// implicit — and the production root passes its Center.
func TestNilSignalBridgeRootsAreExplicit(t *testing.T) {
	callRE := regexp.MustCompile(`bridge\.NewDefault\(\s*([^,)]+)\s*,\s*([^)]+)\)`)
	moduleRoot := filepath.Join("..", "..")
	err := filepath.WalkDir(moduleRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if name == "vendor" || name == "bin" || strings.HasPrefix(name, ".") && path != moduleRoot {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(moduleRoot, path)
		for _, m := range callRE.FindAllStringSubmatch(string(src), -1) {
			second := strings.TrimSuffix(strings.TrimSpace(m[2]), ",")
			if filepath.ToSlash(rel) == "cmd/evolve/cmd_cycle.go" {
				if second == "nil" {
					t.Errorf("%s: the production root must pass its Signal Center, not nil", rel)
				}
				continue
			}
			if second != "nil" {
				t.Errorf("%s: a non-root bridge.NewDefault must pass an explicit nil (Center-less registry default), got %q", rel, second)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// ADR-0101 S4a: the production root decorates the ledger so every appended
// entry is also a ledger.appended signal (Decorator over the file ledger).
func TestWireOrchestratorDeps_LedgerIsObservedByTheSignalCenter(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	d := wireOrchestratorDeps(root, evolveDir, io.Discard)
	observed, ok := d.Ledger.(*ledger.FileLedger)
	if !ok || !observed.SignalsWired() {
		t.Fatalf("the root's ledger must be the file ledger observed by the Signal Center, got %T", d.Ledger)
	}
}

// Unit 01 (ADR-0103), architecture review HIGH-1: the --simulate root builds
// the production signal topology — Center, observed ledger, console sink at
// WARN, durable cycle-less sink — so the recorder's warnings render there as
// the deleted stderr lines did.
func TestWireSimulateOrchestrator_SignalCenterWired(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var console bytes.Buffer
	d := wireSimulateOrchestrator(root, evolveDir, &console)
	if d.Signals == nil || d.Orchestrator == nil || !d.Orchestrator.SignalCenterWired() {
		t.Fatal("the --simulate root must construct a Signal Center and register the orchestrator as its listener")
	}
	if l, ok := d.Ledger.(interface{ SignalsWired() bool }); !ok || !l.SignalsWired() {
		t.Fatal("the --simulate root's ledger is observed like the production root's")
	}
	d.Signals.Emit(signalcenter.Event{
		Module: signalcenter.ModuleOutcome, Origin: "Test.simulate", Kind: signalcenter.KindOutcomeWarning,
		Severity: signalcenter.SeverityWarn, Code: "OUTCOME_TIMING_SKIPPED", Reason: "simulate wiring proof",
	})
	if !strings.Contains(console.String(), "simulate wiring proof") {
		t.Fatalf("the console sink renders WARN under --simulate: %q", console.String())
	}
	if data, err := os.ReadFile(filepath.Join(evolveDir, "signals.ndjson")); err != nil || !strings.Contains(string(data), "simulate wiring proof") {
		t.Errorf("cycle-less signals are durable under --simulate too: %v %s", err, data)
	}
}

// Unit 02 (ADR-0103): the failurediag module tag renders at the --simulate
// root too — a writer built on the root's Center, writing into a file used as
// a workspace, reaches the console sink and the durable cycle-less sink.
func TestWireSimulateOrchestrator_FailureDiagWarningRenders(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var console bytes.Buffer
	d := wireSimulateOrchestrator(root, evolveDir, &console)
	w := failurediag.NewWriter(time.Now, func(error) bool { return false }, failurediag.WithSignals(func() *signalcenter.Center { return d.Signals }))
	fileAsWorkspace := filepath.Join(root, "not-a-dir")
	if err := os.WriteFile(fileAsWorkspace, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	w.Write(fileAsWorkspace, "build", 0, errors.New("boom"), 1)
	if out := console.String(); !strings.Contains(out, "[failurediag]") || !strings.Contains(out, "FAILUREDIAG_SIDECAR_WRITE_FAILED") {
		t.Fatalf("the console sink renders the unit's WARN under --simulate: %q", out)
	}
	if data, err := os.ReadFile(filepath.Join(evolveDir, "signals.ndjson")); err != nil || !strings.Contains(string(data), `"code":"FAILUREDIAG_SIDECAR_WRITE_FAILED"`) {
		t.Errorf("the cycle-less signal is durable: %v %s", err, data)
	}
}

// Unit 03 (ADR-0103): the carryover module tag renders at the --simulate root.
func TestWireSimulateOrchestrator_CarryoverWarningRenders(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var console bytes.Buffer
	d := wireSimulateOrchestrator(root, evolveDir, &console)
	l := carryover.New(carryover.WithSignals(func() *signalcenter.Center { return d.Signals }))
	ws := filepath.Join(root, "ws")
	if err := os.MkdirAll(filepath.Join(ws, "carryover-todos.json"), 0o755); err != nil { // a directory at the path: a read fault, not absence
		t.Fatal(err)
	}
	var state core.State
	l.MergeMemo(&state, ws, 0, time.Now())
	if out := console.String(); !strings.Contains(out, "[carryover]") || !strings.Contains(out, "CARRYOVER_WORKSPACE_READ_FAILED") {
		t.Fatalf("the console sink renders the unit's WARN under --simulate: %q", out)
	}
	if data, err := os.ReadFile(filepath.Join(evolveDir, "signals.ndjson")); err != nil || !strings.Contains(string(data), `"code":"CARRYOVER_WORKSPACE_READ_FAILED"`) {
		t.Errorf("the cycle-less signal is durable: %v %s", err, data)
	}
}
