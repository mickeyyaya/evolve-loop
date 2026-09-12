// cmd_cycle_signal_center_test.go — ADR-0101 S1 wiring proofs. The production
// composition root constructs ONE Signal Center, attaches the durable NDJSON
// sink and the WARN-filtered stderr sink, and registers the orchestrator as a
// listener; the only roots allowed to build an orchestrator WITHOUT a Center
// are pinned here, so a forgotten wiring can never be silent again (the way
// bridge.Deps.LivenessCenter stayed unset in production).
package main

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestWireOrchestratorDeps_SignalCenterWired(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	d := wireOrchestratorDeps(root, evolveDir)
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
// core.WithSignalCenter, except the two pinned nil roots.
func TestNilSignalCenterRootsArePinned(t *testing.T) {
	allowed := map[string]bool{
		"cmd/evolve/cmd_cycle_simulate.go": true,
		"internal/routingtest/engine.go":   true,
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
	stderr := redirectStderr(t, func() {
		d := wireOrchestratorDeps(root, evolveDir)
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

func redirectStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	done := make(chan string)
	go func() {
		data, _ := io.ReadAll(r)
		done <- string(data)
	}()
	fn()
	os.Stderr = orig
	_ = w.Close()
	out := <-done
	_ = r.Close()
	return out
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
