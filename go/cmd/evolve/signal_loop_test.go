package main

// signal_loop_test.go — ADR-0101 S4a: the loop module's producers. A batch halt
// (system failure, pipeline blocker, a fleet lane's halt code, a wave-boundary
// halt) is ONE loop.halt INCIDENT whose code names the rule; a wave summary is
// loop.wave INFO (the report line stays — INFO never prints); a min-width
// repair is loop.wave WARN; an escalation boundary is loop.escalation WARN.
// The hand-written "[loop] … HALT" lines are gone: the root's WARN-filtered
// stderr sink renders them. The batch report reads the driven runner's
// per-cycle SignalSummary through the loop's own orchestrator seam.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func recordingLoopSignals() (*signalcenter.Center, *[]signalcenter.Event, *bytes.Buffer) {
	c := signalcenter.New()
	got := &[]signalcenter.Event{}
	c.Subscribe(func(e signalcenter.Event) { *got = append(*got, e) })
	var console bytes.Buffer
	c.Subscribe(signalcenter.Filter(signalcenter.StderrSink(&console), signalcenter.SeverityWarn))
	return c, got, &console
}

func loopHalts(got []signalcenter.Event) []signalcenter.Event {
	var halts []signalcenter.Event
	for _, e := range got {
		if e.Kind == signalcenter.KindLoopHalt {
			halts = append(halts, e)
		}
	}
	return halts
}

func TestEmitLoopHalt_IsAnIncidentNamingTheRuleAndTheCycle(t *testing.T) {
	c, got, console := recordingLoopSignals()
	emitLoopHalt(c, 1640, "haltOnSystemFailure", CodeLoopSystemFailureHalt, "verdict-incoherence: recorded FAIL but audit=PASS",
		map[string]string{"category": "verdict-incoherence", "level": "system"})
	if len(*got) != 1 {
		t.Fatalf("one loop.halt, got %d", len(*got))
	}
	e := (*got)[0]
	if e.Module != signalcenter.ModuleLoop || e.Kind != signalcenter.KindLoopHalt || e.Severity != signalcenter.SeverityIncident ||
		e.Code != CodeLoopSystemFailureHalt || e.Cycle != 1640 || e.Origin != "haltOnSystemFailure" ||
		e.Fields["category"] != "verdict-incoherence" || !strings.Contains(e.Reason, "recorded FAIL") {
		t.Errorf("a halt is an INCIDENT with the rule's code, the cycle and the floor's fields: %+v", e)
	}
	if !strings.HasPrefix(console.String(), "[loop] loop.halt INCIDENT LOOP_SYSTEM_FAILURE_HALT cycle=1640") {
		t.Errorf("the console renders the halt in the one line format: %q", console.String())
	}
}

func TestEmitLoopWave_InfoForASummaryWarnForARepair(t *testing.T) {
	c, got, console := recordingLoopSignals()
	emitLoopWave(c, 3, "runWaveIteration", "", "wave 3: 2/3 lanes ok", map[string]string{"lanes_ok": "2", "lanes": "3"})
	emitLoopWave(c, 4, "minWidthRepair", CodeLoopMinWidthRepair, "wave 4: min-width repair dispatched 1/1 isolated lane (fleet.count=3 shrank to 0)",
		map[string]string{"desired": "3", "realized": "0"})
	if len(*got) != 2 {
		t.Fatalf("two loop.wave events, got %d", len(*got))
	}
	summary, repair := (*got)[0], (*got)[1]
	if summary.Kind != signalcenter.KindLoopWave || summary.Severity != signalcenter.SeverityInfo || summary.Code != "" ||
		summary.Cycle != 0 || summary.Fields["wave"] != "3" || summary.Fields["lanes_ok"] != "2" {
		t.Errorf("a wave summary is INFO, batch-level (no cycle), with the wave number in fields: %+v", summary)
	}
	if repair.Severity != signalcenter.SeverityWarn || repair.Code != CodeLoopMinWidthRepair || repair.Fields["wave"] != "4" || repair.Fields["desired"] != "3" {
		t.Errorf("a min-width repair is a WARN with its code: %+v", repair)
	}
	if strings.Contains(console.String(), "lanes ok") || !strings.Contains(console.String(), "LOOP_MIN_WIDTH_REPAIR") {
		t.Errorf("INFO stays off the console, the WARN reaches it: %q", console.String())
	}
}

func TestEmitLoopWave_NilFieldsStillCarryTheWave(t *testing.T) {
	c, got, _ := recordingLoopSignals()
	emitLoopWave(c, 9, "runWaveIteration", "", "wave 9: 1/1 lanes ok", nil)
	if len(*got) != 1 || (*got)[0].Fields["wave"] != "9" {
		t.Errorf("the wave number is stamped even without caller fields: %+v", *got)
	}
}

// The producer stamps the wave on its own copy: a caller's map is never
// written to (go review S4a).
func TestEmitLoopWave_DoesNotWriteIntoTheCallersFields(t *testing.T) {
	c, got, _ := recordingLoopSignals()
	fields := map[string]string{"lanes": "3"}
	emitLoopWave(c, 5, "runWaveIteration", "", "wave 5: 3/3 lanes ok", fields)
	if _, stamped := fields["wave"]; stamped || len(fields) != 1 {
		t.Errorf("the caller's map was written to: %v", fields)
	}
	if len(*got) != 1 || (*got)[0].Fields["wave"] != "5" || (*got)[0].Fields["lanes"] != "3" {
		t.Errorf("the event carries the wave and the caller's fields: %+v", *got)
	}
}

func TestEmitLoopEscalation_IsAWarnNamingTheStage(t *testing.T) {
	c, got, _ := recordingLoopSignals()
	emitLoopEscalation(c, 1641, "applyEscalationBoundary", "escalation boundary staged 3 items",
		map[string]string{"stage": "enforce", "bumped": "1", "filed": "2", "skipped": "0", "planned": "0"})
	if len(*got) != 1 {
		t.Fatalf("one loop.escalation, got %d", len(*got))
	}
	e := (*got)[0]
	if e.Kind != signalcenter.KindLoopEscalation || e.Severity != signalcenter.SeverityWarn || e.Code != CodeLoopEscalationBoundary ||
		e.Cycle != 1641 || e.Fields["stage"] != "enforce" || e.Fields["filed"] != "2" {
		t.Errorf("an escalation boundary is a WARN with the stage and the counts: %+v", e)
	}
}

// haltOnSystemFailure is the ONE shared halt+escalate action: it emits ONE
// loop.halt INCIDENT whose code the caller's rule supplies, whose
// fields.next is the dossier's own next_action (one home for what the
// operator does), and whose fields name the dossier and the P0 item it wrote.
func TestHaltOnSystemFailure_EmitsOneHaltNamingTheDossierItWrote(t *testing.T) {
	c, got, console := recordingLoopSignals()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(filepath.Join(evolveDir, "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	sf := &cyclestate.SystemFailureSignal{Category: "verdict-incoherence", Level: "system", Evidence: "recorded FAIL but audit=PASS, acs=PASS", Halt: true}
	rc := haltOnSystemFailure(evolveDir, root, 862, filepath.Join(evolveDir, "runs", "cycle-862"), sf, &stderr, c, systemFailureRule)
	if rc != systemFailureHaltExitCode {
		t.Fatalf("the halt exit code is unchanged: %d", rc)
	}
	if strings.Contains(stderr.String(), "SYSTEM-FAILURE HALT") {
		t.Errorf("the hand-written halt message is gone; the sink renders the signal: %q", stderr.String())
	}
	halts := loopHalts(*got)
	if len(halts) != 1 {
		t.Fatalf("exactly one loop.halt INCIDENT: %+v", *got)
	}
	e := halts[0]
	if e.Code != CodeLoopSystemFailureHalt || e.Cycle != 862 || e.Fields["category"] != "verdict-incoherence" || e.Fields["level"] != "system" {
		t.Errorf("the halt names the rule's code, the cycle and the floor: %+v", e)
	}
	escPath := filepath.Join(evolveDir, "pipeline-escalation.json")
	itemPath := filepath.Join(evolveDir, "inbox", "pipeline-defect-verdict-incoherence-cycle862.json")
	if e.Fields["escalation"] != escPath || e.Fields["inbox_item"] != itemPath {
		t.Errorf("the halt names the dossier and the P0 item it wrote: %+v", e.Fields)
	}
	if _, err := os.Stat(itemPath); err != nil {
		t.Errorf("the named P0 item exists: %v", err)
	}
	raw, err := os.ReadFile(escPath)
	if err != nil {
		t.Fatal(err)
	}
	var dossier struct {
		NextAction string `json:"next_action"`
	}
	if err := json.Unmarshal(raw, &dossier); err != nil {
		t.Fatal(err)
	}
	if dossier.NextAction == "" || e.Fields["next"] != dossier.NextAction {
		t.Errorf("fields.next IS the dossier's next_action (one home):\n signal %q\n dossier %q", e.Fields["next"], dossier.NextAction)
	}
	if !strings.Contains(console.String(), "LOOP_SYSTEM_FAILURE_HALT cycle=862") {
		t.Errorf("the console line: %q", console.String())
	}
}

// The pipeline-blocker breaker halts through the same chokepoint with its
// own rule: ONE INCIDENT whose code names the breaker and whose fields carry
// the rule and fingerprint — not a second, generic system-failure INCIDENT
// on top (S4a architecture review MEDIUM-1).
func TestBlockerBreakerHalt_OneIncidentNamesTheRule(t *testing.T) {
	c, got, console := recordingLoopSignals()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	writeDigestFixture(t, evolveDir, 11, "build|guard-abort|aaa", "guard-abort")
	writeDigestFixture(t, evolveDir, 12, "audit|guard-abort|bbb", "guard-abort")
	var stderr bytes.Buffer
	rc, halted := blockerBreakerHalt(evolveDir, root, 10, &stderr, c)
	if !halted || rc != systemFailureHaltExitCode {
		t.Fatalf("the breaker halts with the halt exit code: halted=%v rc=%d", halted, rc)
	}
	halts := loopHalts(*got)
	if len(halts) != 1 {
		t.Fatalf("one breaker halt is ONE INCIDENT: %+v", halts)
	}
	e := halts[0]
	if e.Code != CodeLoopPipelineBlockerHalt || e.Cycle != 12 || e.Fields["category"] != "pipeline-blocker" ||
		e.Fields["rule"] == "" || e.Fields["fingerprint"] == "" || e.Fields["next"] == "" || e.Fields["escalation"] == "" {
		t.Errorf("the INCIDENT names the breaker's rule, the fingerprint, and what the halt wrote: %+v", e)
	}
	if strings.Contains(console.String(), "LOOP_SYSTEM_FAILURE_HALT") || !strings.Contains(console.String(), "LOOP_PIPELINE_BLOCKER_HALT cycle=12") {
		t.Errorf("the console renders the breaker's code once: %q", console.String())
	}
	if strings.Contains(stderr.String(), "PIPELINE-BLOCKER HALT") {
		t.Errorf("the hand-written line is gone: %q", stderr.String())
	}
}

func TestFormatSignalReport_NamesCountsAndTheLastIncident(t *testing.T) {
	s := signalcenter.NewSummary()
	if got := formatSignalReport(7, s.Snapshot()); got != "" {
		t.Errorf("a cycle with no signals adds no report line, got %q", got)
	}
	s.Observe(signalcenter.Event{Cycle: 7, Kind: signalcenter.KindPhaseOutcome, Severity: signalcenter.SeverityInfo, Reason: "scout PASS"})
	s.Observe(signalcenter.Event{Cycle: 7, Kind: signalcenter.KindPhaseOutcome, Severity: signalcenter.SeverityWarn, Code: "ORCHESTRATOR_PHASE_VERDICT_FAIL", Reason: "triage FAIL"})
	s.Observe(signalcenter.Event{Cycle: 7, Kind: signalcenter.KindSystemFailure, Severity: signalcenter.SeverityIncident, Code: "ORCHESTRATOR_SYSTEM_FAILURE", Reason: "verdict-incoherence: forged"})
	want := "[loop] cycle 7 signals: 3 total, 1 WARN, 1 INCIDENT (last INCIDENT ORCHESTRATOR_SYSTEM_FAILURE — verdict-incoherence: forged)\n"
	if got := formatSignalReport(7, s.Snapshot()); got != want {
		t.Errorf("the batch report renders the orchestrator's view:\n got %q\nwant %q", got, want)
	}
}

// summaryOrch is a loopCycleRunner whose only interesting answer is its
// SignalSummary — the report must read the runner the loop drives, not the
// concrete orchestrator in deps (S4a architecture review MEDIUM-3).
type summaryOrch struct{ summary signalcenter.Summary }

func (o *summaryOrch) RunCycle(context.Context, core.CycleRequest) (core.CycleResult, error) {
	return core.CycleResult{}, nil
}

func (o *summaryOrch) RunCycleFromPhase(context.Context, core.CycleRequest, *core.ResumePoint) (core.CycleResult, error) {
	return core.CycleResult{}, nil
}

func (o *summaryOrch) SignalSummary() signalcenter.Summary { return o.summary }

// noSignals is the SignalSummary of the scripted runners that raise none.
type noSignals struct{}

func (noSignals) SignalSummary() signalcenter.Summary { return signalcenter.Summary{} }

func TestObserveSequentialCycle_ReportsTheDrivenRunnersSignalSummary(t *testing.T) {
	s := signalcenter.NewSummary()
	s.Observe(signalcenter.Event{Cycle: 7, Kind: signalcenter.KindPhaseOutcome, Severity: signalcenter.SeverityWarn, Code: "ORCHESTRATOR_PHASE_VERDICT_FAIL", Reason: "triage FAIL"})
	s.Observe(signalcenter.Event{Cycle: 7, Kind: signalcenter.KindSystemFailure, Severity: signalcenter.SeverityIncident, Code: "ORCHESTRATOR_SYSTEM_FAILURE", Reason: "forged"})
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	var stdout, stderr bytes.Buffer
	b := &loopBatchCoordinator{orch: &summaryOrch{summary: s.Snapshot()}, cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, result: &loopResult{}, stdout: &stdout, stderr: &stderr}
	d := b.observeSequentialCycle(sequentialCycle{cycle: 7, lastBefore: 6, lastAfter: 7, workspace: filepath.Join(evolveDir, "runs", "cycle-7")}, &sequentialBatchState{threshold: 3})
	if d.flow != batchProceed {
		t.Fatalf("a quiet cycle proceeds: %+v", d)
	}
	if want := "[loop] cycle 7 signals: 2 total, 1 WARN, 1 INCIDENT (last INCIDENT ORCHESTRATOR_SYSTEM_FAILURE — forged)\n"; !strings.Contains(stderr.String(), want) {
		t.Errorf("the batch report renders the driven runner's summary:\n got %q\nwant %q", stderr.String(), want)
	}
}

// testRootSignals builds the production sink topology over a throwaway root
// for the direct producer tests (the stub root builds it over the test's own).
func testRootSignals(t *testing.T, console io.Writer) *signalcenter.Center {
	t.Helper()
	root := t.TempDir()
	return newRootSignalCenter(root, filepath.Join(root, ".evolve"), console)
}

func ndjsonLines(t *testing.T, path string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("durable file %s: %v", path, err)
	}
	return strings.Count(strings.TrimSpace(string(raw)), "\n") + 1
}

// newRootSignalCenter is the ONE sink topology: the durable signals.ndjson
// (per cycle workspace; the batch-level file under the evolve dir for
// cycle-less signals — the loop's own halts and wave summaries, which would
// otherwise be reported as drops after every wave) and the console at WARN
// and above. The production root and the loop tests' stub root both build it
// here, so a test that asserts the rendered line proves production's
// topology (S4a architecture review MEDIUM-2).
func TestNewRootSignalCenter_WritesTheDurableFilesAndRendersWarnOnTheConsole(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	var console bytes.Buffer
	c := newRootSignalCenter(root, evolveDir, &console)
	emitLoopWave(c, 1, "runWaveIteration", "", "wave 1: 1/1 lanes ok", nil)
	c.Emit(signalcenter.Event{Cycle: 3, Module: signalcenter.ModuleLoop, Origin: "test", Kind: signalcenter.KindLoopWave, Severity: signalcenter.SeverityInfo, Reason: "quiet"})
	c.Emit(signalcenter.Event{Cycle: 3, Module: signalcenter.ModuleLoop, Origin: "test", Kind: signalcenter.KindLoopWave, Severity: signalcenter.SeverityWarn, Code: CodeLoopMinWidthRepair, Reason: "shrank"})
	c.Flush()
	if strings.Contains(console.String(), "quiet") || !strings.Contains(console.String(), "LOOP_MIN_WIDTH_REPAIR cycle=3") {
		t.Errorf("the console shows WARN and above only: %q", console.String())
	}
	if strings.Contains(console.String(), "SINK_DROPPED") {
		t.Errorf("a batch-level signal is durable, never a drop report: %q", console.String())
	}
	if n := ndjsonLines(t, filepath.Join(core.RunWorkspacePath(root, 3), "signals.ndjson")); n != 2 {
		t.Errorf("both cycle-3 signals are in the cycle's file: %d lines", n)
	}
	if n := ndjsonLines(t, filepath.Join(evolveDir, "signals.ndjson")); n != 1 {
		t.Errorf("the batch-level wave is in the evolve dir's file: %d lines", n)
	}
}

// A fleet lane that exited with the system-failure halt code stops the
// batch through fleetHaltDecision — one loop.halt INCIDENT, rendered, whose
// reason points at the lane's own INCIDENT instead of restating what the
// lane wrote (S4a architecture review MEDIUM-4).
func TestFleetHaltDecision_ALaneHaltCodeIsALoopHaltIncident(t *testing.T) {
	c, got, console := recordingLoopSignals()
	var stdout, stderr bytes.Buffer
	b := &loopBatchCoordinator{deps: orchDeps{Signals: c}, result: &loopResult{}, cfg: loopConfig{EvolveDir: filepath.Join(t.TempDir(), ".evolve")}, stdout: &stdout, stderr: &stderr}
	decision := b.fleetHaltDecision("wave", 3, []fleet.Result{{Index: 0, ExitCode: 0}, {Index: 1, ExitCode: systemFailureHaltExitCode}})
	if decision.flow != batchReturn || decision.exitCode != systemFailureHaltExitCode || b.result.StopReason != "system_failure_halt" {
		t.Fatalf("the halt decision is unchanged: %+v stop=%q", decision, b.result.StopReason)
	}
	if len(*got) != 1 || (*got)[0].Code != CodeLoopFleetLaneHalt || (*got)[0].Severity != signalcenter.SeverityIncident || (*got)[0].Fields["iteration"] != "3" || (*got)[0].Fields["kind"] != "wave" {
		t.Errorf("one loop.halt INCIDENT naming the lane halt: %+v", *got)
	}
	if reason := (*got)[0].Reason; !strings.Contains(reason, string(CodeLoopSystemFailureHalt)) || strings.Contains(reason, "pipeline-escalation.json") {
		t.Errorf("the reason points at the lane's own INCIDENT and does not restate its dossier: %q", reason)
	}
	if strings.Contains(stderr.String(), "SYSTEM-FAILURE HALT") || !strings.Contains(console.String(), "LOOP_FLEET_LANE_HALT") {
		t.Errorf("the hand-written line is gone and the console renders the signal: stderr=%q console=%q", stderr.String(), console.String())
	}
	if b.fleetHaltDecision("wave", 4, []fleet.Result{{ExitCode: 0}}).flow != batchProceed || len(*got) != 1 {
		t.Errorf("no halt code, no halt, no signal: %+v", *got)
	}
}
