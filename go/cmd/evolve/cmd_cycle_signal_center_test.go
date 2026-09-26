package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/failurediag"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/failurelearning"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopwave"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/audit/ciparitygate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
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

// The one pinned nil root, internal/routingtest, is test machinery that takes a *testing.T.
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
		// Counted per file, so an unwired call beside a wired one still fails.
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

// captureConsole returns what the root's WARN-filtered console sink rendered.
func captureConsole(fn func(console io.Writer)) string {
	var buf bytes.Buffer
	fn(&buf)
	return buf.String()
}

// Only this binary links every producer module, so the real registry is checked here.
func TestSignalCenterRegistry_EveryLinkedModuleRegistersCleanly(t *testing.T) {
	if conflicts := signalcenter.RegistryConflicts(); len(conflicts) != 0 {
		t.Errorf("code registry conflicts across linked modules: %+v", conflicts)
	}
	if _, ok := signalcenter.IsRegistered(core.CodePhaseVerdictFail); !ok {
		t.Error("core's codes must be registered by the time the binary is linked")
	}
}

func TestSignalCenterFlush_IsWiredAtBothRoots(t *testing.T) {
	for file, needle := range map[string]string{"cmd_cycle.go": "Signals.Flush()", "cmd_loop.go": "Signals.Flush()", "cmd_loop_chain.go": "signals.Flush()"} {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(src), needle) {
			t.Errorf("%s: the root must defer %s after wiring its Center", file, needle)
		}
	}
}

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

func TestWireSimulateOrchestrator_FailureLearningWarningRenders(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(filepath.Join(evolveDir, "policy.json"), 0o755); err != nil { // a directory at the path: a read fault, not absence
		t.Fatal(err)
	}
	var console bytes.Buffer
	d := wireSimulateOrchestrator(root, evolveDir, &console)
	e := failurelearning.New(time.Now, carryover.New(), failurelearning.WithSignals(func() *signalcenter.Center { return d.Signals }))
	e.WriteFloor(failurelearning.Failure{Cycle: 3, Phase: core.PhaseAudit, ProjectRoot: root, Workspace: filepath.Join(root, "ws")}, failurelearning.Learned{Summary: "s"})
	if out := console.String(); !strings.Contains(out, "[failurelearning]") || !strings.Contains(out, "FAILURELEARNING_POLICY_LOAD_FAILED") {
		t.Fatalf("the console sink renders the unit's WARN under --simulate: %q", out)
	}
	if data, err := os.ReadFile(filepath.Join(core.RunWorkspacePath(root, 3), "signals.ndjson")); err != nil || !strings.Contains(string(data), `"code":"FAILURELEARNING_POLICY_LOAD_FAILED"`) {
		t.Errorf("the cycle-stamped signal is durable in the cycle workspace: %v %s", err, data)
	}
}

func TestConsoleSinkThresholdHasOneHome(t *testing.T) {
	const home = "internal/signalcenter/sinks.go"
	moduleRoot := filepath.Join("..", "..")
	var respelled []string
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
		if rel = filepath.ToSlash(rel); rel != home && strings.Contains(string(src), "StderrSink(") {
			respelled = append(respelled, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(respelled) != 0 {
		t.Errorf("the console threshold is spelled outside signalcenter.ConsoleSink: %v", respelled)
	}
	for _, root := range []string{"cmd_cycle.go", filepath.Join(moduleRoot, "internal", "cli", "phasecmd", "phase_observer.go")} {
		src, err := os.ReadFile(root)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(src), "signalcenter.ConsoleSink(") {
			t.Errorf("%s: a composition root consumes signalcenter.ConsoleSink", root)
		}
	}
}

// Ship and retro are not BaseRunner-backed, so they carry no verdict engine.
func TestWireOrchestratorDeps_SignalCenterReachesEveryPhaseRunner(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	d := wireOrchestratorDeps(root, evolveDir, io.Discard)
	if d.Bridge == nil || d.Bridge.Signals() != d.Signals {
		t.Fatal("the production Adapter carries the root's Center (Signals() == orchDeps.Signals)")
	}
	if len(d.Runners) == 0 {
		t.Fatal("orchDeps.Runners must expose the phase-runner map the orchestrator was built over")
	}
	for phase, r := range d.Runners {
		if phase == core.PhaseShip || phase == core.PhaseRetro {
			continue
		}
		w, ok := r.(interface{ SignalsWired() bool })
		if !ok {
			t.Errorf("phase %s: runner %T exposes no SignalsWired()", phase, r)
			continue
		}
		if !w.SignalsWired() {
			t.Errorf("phase %s: the verdict engine reaches no Center — built off a Center-less bridge?", phase)
		}
	}
	for _, phase := range []core.Phase{core.PhaseScout, core.PhaseBuild, core.PhaseBuildPlanner, core.PhaseAudit, core.PhaseTriage} {
		if _, ok := d.Runners[phase]; !ok {
			t.Errorf("phase %s missing from the root's runner map", phase)
		}
	}
}

func TestWireSimulateOrchestrator_AuditLedgerWarningRenders(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	ws := core.RunWorkspacePath(root, 1)
	ancestorWS := core.RunWorkspacePath(root, 0)
	if err := os.MkdirAll(filepath.Join(ws, defectledger.LedgerFile), 0o755); err != nil { // a directory at the path: a read fault, not absence
		t.Fatal(err)
	}
	if err := os.MkdirAll(ancestorWS, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := defectledger.Write(ancestorWS, defectledger.Doc{Entries: []defectledger.Entry{{ID: "d1", Text: "inherited", Status: defectledger.StatusOpen}}}); err != nil {
		t.Fatal(err)
	}
	if err := continuation.WriteManifest(ws, continuation.Continuation{Cycle: 0, SnapshotSHA: "deadbeef"}); err != nil {
		t.Fatal(err)
	}
	var console bytes.Buffer
	d := wireSimulateOrchestrator(root, evolveDir, &console)
	l := defectledger.New(func(string) []string { return nil }, func(string, defectledger.Request) (bool, string) { return false, "stub" },
		defectledger.WithSignals(func() *signalcenter.Center { return d.Signals }))
	if v := l.Reconcile(defectledger.Request{Cycle: 1, Workspace: ws, ProjectRoot: root}); !v.Blocked {
		t.Fatalf("an unreadable own ledger blocks: %+v", v)
	}
	if out := console.String(); !strings.Contains(out, "[audit] audit.warning WARN AUDIT_LEDGER_UNREADABLE cycle=1 phase=audit") || !strings.Contains(out, "origin=Ledger.Reconcile") {
		t.Fatalf("the console sink renders the unit's WARN under --simulate: %q", out)
	}
	if data, err := os.ReadFile(filepath.Join(ws, "signals.ndjson")); err != nil || !strings.Contains(string(data), `"code":"AUDIT_LEDGER_UNREADABLE"`) || !strings.Contains(string(data), `"module":"audit"`) {
		t.Errorf("the cycle-stamped signal is durable in the cycle workspace: %v %s", err, data)
	}
}

func TestAuditRoot_PassesTheSignalCenter(t *testing.T) {
	src, err := os.ReadFile("cmd_cycle.go")
	if err != nil {
		t.Fatal(err)
	}
	var auditLine string
	for _, line := range strings.Split(string(src), "\n") {
		if strings.Contains(line, "core.PhaseAudit:") {
			auditLine = line
		}
	}
	if auditLine == "" || !strings.Contains(auditLine, "audit.WithSignals(") {
		t.Fatalf("the core.PhaseAudit runner must be built with audit.WithSignals(…): %q", auditLine)
	}
}

// inlineHooks is a minimal runner.Hooks whose prompt ships as data, with no agent doc on disk.
type inlineHooks struct{}

func (inlineHooks) PhaseName() string                         { return "audit" }
func (inlineHooks) AgentPromptName() string                   { return "evolve-auditor" }
func (inlineHooks) ArtifactFilename(core.PhaseRequest) string { return "audit-report.md" }
func (inlineHooks) DefaultModel() string                      { return "opus" }
func (inlineHooks) ComposePrompt(body string, _ core.PhaseRequest) string {
	return body
}
func (inlineHooks) Classify(string, core.PhaseRequest, core.BridgeResponse) (string, []core.Diagnostic, string) {
	return core.VerdictPASS, nil, ""
}
func (inlineHooks) InlinePromptBody() (string, bool) { return "inline body", true }

// writingBridge writes the contracted artifact and exits cleanly.
type writingBridge struct{}

func (writingBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	if req.ArtifactPath != "" {
		_ = os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755)
		_ = os.WriteFile(req.ArtifactPath, []byte("# audit\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\"} -->\n"), 0o644)
	}
	return core.BridgeResponse{}, nil
}
func (writingBridge) Probe(context.Context) (core.BridgeProbe, error) { return core.BridgeProbe{}, nil }

func TestWireSimulateOrchestrator_RunnerWarningRenders(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var console bytes.Buffer
	d := wireSimulateOrchestrator(root, evolveDir, &console)
	r := runner.New(runner.Options{
		Hooks: inlineHooks{}, Bridge: writingBridge{}, Prompts: prompts.NewFromFS(fstest.MapFS{}),
		VerifyFn: func(phase string, roots phasecontract.Roots) (deliverable.Result, error) {
			return deliverable.Result{OK: true, Phase: phase}, nil
		},
		StdoutFilter: func(string, string) error { return errors.New("synthetic filter blowup") },
		Signals:      func() *signalcenter.Center { return d.Signals },
	})
	if !r.SignalsWired() {
		t.Fatal("Options.Signals wires the runner's verdict engine")
	}
	ws := core.RunWorkspacePath(root, 7)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	resp, err := r.Run(context.Background(), core.PhaseRequest{Cycle: 7, ProjectRoot: root, Workspace: ws})
	if err != nil || resp.Verdict != core.VerdictPASS {
		t.Fatalf("a failing filter never blocks the phase: %+v %v", resp, err)
	}
	if out := console.String(); !strings.Contains(out, "[runner] runner.warning WARN RUNNER_STDOUT_FILTER_FAILED cycle=7 phase=audit") {
		t.Fatalf("the console sink renders the unit's WARN under --simulate: %q", out)
	}
	if data, err := os.ReadFile(filepath.Join(ws, "signals.ndjson")); err != nil || !strings.Contains(string(data), `"code":"RUNNER_STDOUT_FILTER_FAILED"`) {
		t.Errorf("the cycle-stamped signal is durable in the cycle workspace: %v %s", err, data)
	}
}

func TestWireSimulateOrchestrator_LoopWaveAndChainWarningsRender(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var console bytes.Buffer
	d := wireSimulateOrchestrator(root, evolveDir, &console)
	wave := newWaveEngine(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, d.Storage, io.Discard, func() *signalcenter.Center { return d.Signals })
	wave.RepairMinWidth(context.Background(), policy.FleetConfig{Count: 1}, policy.FleetConfig{Count: 1}, loopwave.DispatchRequest{Wave: 2})
	if err := os.WriteFile(filepath.Join(evolveDir, "inbox"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	chain := loopchain.NewDriver(loopchain.Roots{ProjectRoot: root, EvolveDir: evolveDir}, policy.ChainConfig{MaxBatches: 1}, loopchain.DriverDeps{
		Batch: func() int { return 0 }, Refresh: func(int) bool { return false }, LastRefresh: func() (*loopchain.RefreshLogEntry, error) { return nil, nil },
		FleetWidth: func() int { return 1 }, QuotaPause: func() (loopchain.QuotaPause, bool) { return loopchain.QuotaPause{}, false },
	}, io.Discard, loopchain.WithSignals(func() *signalcenter.Center { return d.Signals }))
	if r := chain.Run(); r.StopReason != loopchain.StopInboxUnreadable {
		t.Fatalf("the chain stops on an unreadable inbox: %+v", r)
	}
	d.Signals.Flush()
	out := console.String()
	for _, want := range []string{"[loop] loop.wave WARN LOOP_WAVE_EMPTY_PLAN", "origin=Engine.RepairMinWidth", "[loop] loop.halt INCIDENT LOOP_CHAIN_INBOX_UNREADABLE", "origin=Driver.Run"} {
		if !strings.Contains(out, want) {
			t.Errorf("the console sink renders the unit's signals under --simulate; lacks %q:\n%s", want, out)
		}
	}
	data, err := os.ReadFile(filepath.Join(evolveDir, "signals.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{`"code":"LOOP_WAVE_EMPTY_PLAN"`, `"code":"LOOP_CHAIN_INBOX_UNREADABLE"`} {
		if !strings.Contains(string(data), code) {
			t.Errorf("the cycle-less signal is durable in <evolveDir>/signals.ndjson; lacks %s", code)
		}
	}
}

func TestAuditRootPassesTheCenter(t *testing.T) {
	src, err := os.ReadFile("cmd_cycle.go")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`(?s)audit\.NewDefaultWithStageCompactSpec\(.{0,240}?audit\.WithSignals\(`) // the option belongs to THIS call (gofmt wraps it onto the next line)
	if n := len(re.FindAll(src, -1)); n != 1 {
		t.Fatalf("the audit runner must be built with audit.WithSignals(…) exactly once in cmd_cycle.go, found %d", n)
	}
}

func TestWireSimulateOrchestrator_CIParityWarningRenders(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(filepath.Join(root, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go", "go.mod"), []byte("module ciparitytest\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var console bytes.Buffer
	d := wireSimulateOrchestrator(root, evolveDir, &console)
	runs := 0
	redThenGreen := func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, so, _ io.Writer) (int, error) {
		runs++
		if runs == 1 {
			_, _ = io.WriteString(so, "--- FAIL: TestFlaky (0.00s)\nFAIL\tpkg\t1.0s\n")
			return 1, nil
		}
		return 0, nil
	}
	changed := func(string, int) ([]string, bool) { return []string{"./internal/p/..."}, true }
	g := ciparitygate.New(redThenGreen, changed, ciparitygate.WithSignals(func() *signalcenter.Center { return d.Signals }))
	ws := core.RunWorkspacePath(root, 3)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := g.IntegrationTier(ciparitygate.Request{Cycle: 3, ProjectRoot: root, Worktree: root, Workspace: ws}); err == nil {
		t.Fatal("red-then-green must surface the flake WARN")
	}
	if out := console.String(); !strings.Contains(out, "[audit]") || !strings.Contains(out, "AUDIT_CIPARITY_TIER_FLAKE_ABSORBED") {
		t.Fatalf("the console sink renders the unit's WARN under --simulate: %q", out)
	}
	if data, err := os.ReadFile(filepath.Join(ws, "signals.ndjson")); err != nil || !strings.Contains(string(data), `"code":"AUDIT_CIPARITY_TIER_FLAKE_ABSORBED"`) {
		t.Errorf("the cycle-stamped signal is durable in the cycle workspace: %v %s", err, data)
	}
	if _, err := os.Stat(filepath.Join(ws, "integration-tier.log")); err != nil {
		t.Errorf("integration-tier.log sits beside signals.ndjson: %v", err)
	}
}
