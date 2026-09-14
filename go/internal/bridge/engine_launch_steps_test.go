package bridge

// engine_launch_steps_test.go — ADR-0103 unit 10, the split Launch spine and
// its emissions (design §6 tests 41-48): the attempt context rides every
// unit-10 event (call_id is the llm-calls.ndjson join key), the four step
// failures are bridge.warning codes with per-step origins where two were
// "[engine]" stderr lines and two were silent, exactly one BRIDGE_EXIT_* per
// non-zero exit after the persist and the strike record, nothing on ExitOK.
// The four host codes are named by IDENTIFIER (apicover counts exported
// consts named by a bridge test).

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/launchoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// eventsWithCode filters the recorded stream by code.
func eventsWithCode(events []signalcenter.Event, code signalcenter.Code) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range events {
		if e.Code == code {
			out = append(out, e)
		}
	}
	return out
}

// exitEvents filters the recorded stream to the BRIDGE_EXIT_* codes.
func exitEvents(events []signalcenter.Event) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range events {
		if strings.HasPrefix(string(e.Code), "BRIDGE_EXIT_") {
			out = append(out, e)
		}
	}
	return out
}

// recordingDeps builds the fixture deps under a recording Center with a
// captured stderr.
func recordingDeps(t *testing.T) (Deps, *[]signalcenter.Event, *strings.Builder) {
	t.Helper()
	c, got := recordingSignals()
	stderr := new(strings.Builder)
	return Deps{Signals: c, Stderr: stderr}, got, stderr
}

// Test 41 — recordModelAttempt returns the attempt context it built: the
// ledger row's call_id, the attempt default (1) or the request's, cli, agent.
func TestRecordModelAttempt_ReturnsTheAttemptContext(t *testing.T) {
	ws := t.TempDir()
	e := NewEngine(Deps{Stderr: new(strings.Builder)})
	req := core.BridgeRequest{CLI: "codex", Agent: "build", Cycle: 7, RunID: "run-7", Workspace: ws}
	start := time.Date(2026, 9, 14, 1, 0, 0, 0, time.UTC)
	c := e.recordModelAttempt(req, "auto", ExitOK, start, start.Add(time.Second), modelDispatch{}, "", &core.BridgeResponse{})
	rec := lastLedgerRecord(t, ws)
	if c.callID == "" || c.callID != rec.CallID || c.attempt != 1 || c.cli != "codex" || c.agent != "build" || c.cycle != 7 || c.runID != "run-7" || c.phase != "build" {
		t.Fatalf("the returned context is the ledger row's identity: %+v vs call_id %q", c, rec.CallID)
	}
	req.Attempt = 3
	if c := e.recordModelAttempt(req, "auto", ExitOK, start, start, modelDispatch{}, "", &core.BridgeResponse{}); c.attempt != 3 {
		t.Fatalf("attempt follows the request when set: %d", c.attempt)
	}
}

// Test 42 — a boot-strike clear failure after a non-80 exit is ONE
// BRIDGE_BOOT_STRIKE_CLEAR_FAILED with the step origin and the call identity;
// the old "[engine] boot-strike clear failed" line is gone.
func TestEngineLaunch_BootStrikeClearFailure_IsABridgeWarning(t *testing.T) {
	deps, got, stderr := recordingDeps(t)
	deps.BootTimeoutStore = brokenBootStrikeStore(t)
	_, ws, err := launchThroughFixture(t, context.Background(), deps, launchFixtureScript(ExitBadFlags, "first-bridge-line"), core.BridgeRequest{Cycle: 5, RunID: "run-5"})
	if err == nil {
		t.Fatal("exit 10 is an error")
	}
	w := eventsWithCode(*got, CodeBootStrikeClearFailed)
	if len(w) != 1 {
		t.Fatalf("exactly one BRIDGE_BOOT_STRIKE_CLEAR_FAILED, got %d: %+v", len(w), *got)
	}
	e := w[0]
	if e.Module != signalcenter.ModuleBridge || e.Kind != signalcenter.KindBridgeWarning || e.Severity != signalcenter.SeverityWarn || e.Origin != "Engine.clearBootStrike" ||
		e.Cycle != 5 || e.RunID != "run-5" || e.Phase != "build" || e.Attempt != 1 || e.Fields["step"] != "clear_boot_strike" ||
		e.Fields["call_id"] != lastLedgerRecord(t, ws).CallID || e.Fields["cli"] != launchOutcomeFixtureCLI || e.Fields["agent"] != "build" ||
		!strings.HasPrefix(e.Reason, "boot-strike clear failed for "+launchOutcomeFixtureCLI+": ") {
		t.Fatalf("the clear failure names its step and the attempt: %+v", e)
	}
	if strings.Contains(stderr.String(), "boot-strike clear failed") {
		t.Fatalf("the hand-written [engine] line is gone; the sink renders the event: %q", stderr.String())
	}
}

// Test 43 — a boot-strike record failure after exit 80 is ONE
// BRIDGE_BOOT_STRIKE_RECORD_FAILED; the error still wraps the transient
// sentinel; a non-80 exit never records a strike.
func TestEngineLaunch_BootStrikeRecordFailure_IsABridgeWarning(t *testing.T) {
	deps, got, stderr := recordingDeps(t)
	deps.BootTimeoutStore = brokenBootStrikeStore(t)
	_, _, err := launchThroughFixture(t, context.Background(), deps, launchFixtureScript(ExitREPLBootTimeout, "empty"), core.BridgeRequest{})
	if !errors.Is(err, core.ErrTransientBridgeFailure) {
		t.Fatalf("exit 80 still wraps the transient sentinel: %v", err)
	}
	w := eventsWithCode(*got, CodeBootStrikeRecordFailed)
	if len(w) != 1 || w[0].Origin != "Engine.recordBootStrike" || w[0].Fields["step"] != "record_boot_strike" || w[0].Fields["cli"] != launchOutcomeFixtureCLI ||
		!strings.HasPrefix(w[0].Reason, "boot-timeout bench record failed for "+launchOutcomeFixtureCLI+": ") {
		t.Fatalf("exactly one BRIDGE_BOOT_STRIKE_RECORD_FAILED with its step: %+v", *got)
	}
	if len(eventsWithCode(*got, CodeBootStrikeClearFailed)) != 0 {
		t.Fatal("exit 80 never clears (the REPL did not boot)")
	}
	if strings.Contains(stderr.String(), "boot-timeout bench record failed") {
		t.Fatalf("the hand-written [engine] line is gone: %q", stderr.String())
	}
	store := clihealth.NewStore(t.TempDir(), nil)
	_, _, _ = launchThroughFixture(t, context.Background(), Deps{BootTimeoutStore: store}, launchFixtureScript(ExitBadFlags, "empty"), core.BridgeRequest{})
	if benches, _ := store.Load(); benches[launchOutcomeFixtureCLI].Strikes != 0 {
		t.Fatalf("a non-80 exit records no strike: %+v", benches)
	}
}

// Test 44 — an unwritable launch-error file is ONE
// BRIDGE_LAUNCH_ERROR_PERSIST_FAILED with the path; the classified error is
// unchanged and the BRIDGE_EXIT_* event carries no launch_error field.
func TestEngineLaunch_LaunchErrorPersistFailure_IsABridgeWarning(t *testing.T) {
	deps, got, _ := recordingDeps(t)
	ws := t.TempDir()
	target := filepath.Join(ws, "build-launch-error.txt")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	_, _, err := launchThroughFixture(t, context.Background(), deps, launchFixtureScript(ExitBadFlags, "first-bridge-line"), core.BridgeRequest{Workspace: ws})
	if err == nil || err.Error() != "bridge: launch exit=10: [bridge] launch: missing required (flag or env): --profile" {
		t.Fatalf("the classified error is unchanged by the persist failure: %v", err)
	}
	w := eventsWithCode(*got, CodeLaunchErrorPersistFailed)
	if len(w) != 1 || w[0].Origin != "Engine.persistLaunchError" || w[0].Fields["step"] != "persist_launch_error" || w[0].Fields["path"] != target ||
		!strings.HasPrefix(w[0].Reason, "launch-error persist failed: path="+target+" error=") {
		t.Fatalf("exactly one BRIDGE_LAUNCH_ERROR_PERSIST_FAILED with the path: %+v", *got)
	}
	exits := exitEvents(*got)
	if len(exits) != 1 || exits[0].Fields["launch_error"] != "" {
		t.Fatalf("the exit event carries no launch_error when nothing was persisted: %+v", exits)
	}
	if _, has := exits[0].Fields["launch_error"]; has {
		t.Fatal("launch_error is absent, not empty")
	}
}

// Test 45 — exit 0 with an unreadable result is ONE BRIDGE_RESULT_READ_FAILED
// (path, completion) and still a nil error with an empty Stdout: the on-disk
// report is the verdict source, never the response text.
func TestEngineLaunch_ResultReadFailure_IsABridgeWarning_NotAnError(t *testing.T) {
	deps, got, _ := recordingDeps(t)
	resp, ws, err := launchThroughFixture(t, context.Background(), deps, launchFixtureScript(ExitOK, "empty"), core.BridgeRequest{})
	if err != nil || resp.Stdout != "" || resp.ExitCode != ExitOK {
		t.Fatalf("an unreadable artifact is not an error: err=%v resp=%+v", err, resp)
	}
	w := eventsWithCode(*got, CodeResultReadFailed)
	if len(w) != 1 || w[0].Origin != "Engine.readResult" || w[0].Fields["step"] != "read_result" || w[0].Fields["path"] != filepath.Join(ws, "a.md") || w[0].Fields["completion"] != "artifact" ||
		!strings.HasPrefix(w[0].Reason, "result read failed: path="+filepath.Join(ws, "a.md")+" completion=artifact error=") {
		t.Fatalf("exactly one BRIDGE_RESULT_READ_FAILED naming the artifact: %+v", *got)
	}
	deps, got, _ = recordingDeps(t)
	_, ws, _ = launchThroughFixture(t, context.Background(), deps, launchFixtureScript(ExitOK, "empty"), core.BridgeRequest{Completion: "stdout"})
	w = eventsWithCode(*got, CodeResultReadFailed)
	if len(w) != 1 || w[0].Fields["path"] != filepath.Join(ws, "build-stdout.log") || w[0].Fields["completion"] != "stdout" {
		t.Fatalf("under the stdout contract the scrollback path is named: %+v", w)
	}
	if len(exitEvents(*got)) != 0 {
		t.Fatal("exit 0 emits no BRIDGE_EXIT_* code")
	}
}

// Test 46 — a non-zero exit emits exactly ONE BRIDGE_EXIT_* event from
// Engine.Launch with the classification's fields, the persisted launch-error
// path and the ledger's call_id; the reason IS the returned error string.
func TestEngineLaunch_NonZeroExit_EmitsOneBridgeExitSignal(t *testing.T) {
	deps, got, _ := recordingDeps(t)
	_, ws, err := launchThroughFixture(t, context.Background(), deps, launchFixtureScript(ExitBadFlags, "first-bridge-line"), core.BridgeRequest{Cycle: 9, RunID: "run-9"})
	if err == nil {
		t.Fatal("exit 10 is an error")
	}
	exits := exitEvents(*got)
	if len(exits) != 1 {
		t.Fatalf("exactly one BRIDGE_EXIT_* event per non-zero exit, got %d: %+v", len(exits), *got)
	}
	e := exits[0]
	if e.Code != launchoutcome.CodeExitBadFlags || e.Module != signalcenter.ModuleBridge || e.Kind != signalcenter.KindBridgeWarning || e.Severity != signalcenter.SeverityWarn ||
		e.Origin != "Engine.Launch" || e.Cycle != 9 || e.RunID != "run-9" || e.Phase != "build" || e.Attempt != 1 || e.Reason != err.Error() {
		t.Fatalf("the exit event is the classification: %+v", e)
	}
	want := map[string]string{
		"step": "classify", "call_id": lastLedgerRecord(t, ws).CallID, "cli": launchOutcomeFixtureCLI, "agent": "build",
		"exit_code": "10", "cause_code": "bad_flags", "transient": "false", "ctx_cancelled": "false",
		"launch_error": filepath.Join(ws, "build-launch-error.txt"),
	}
	for k, v := range want {
		if e.Fields[k] != v {
			t.Errorf("fields[%s] = %q, want %q", k, e.Fields[k], v)
		}
	}
	if len(e.Fields) != len(want) {
		t.Errorf("fields = %v, want exactly %v", e.Fields, want)
	}
	deps, got, _ = recordingDeps(t)
	_, _, err = launchThroughFixture(t, launchCtx("cancelled"), deps, launchFixtureScript(launchoutcome.ExitSignalDeath, "empty"), core.BridgeRequest{})
	exits = exitEvents(*got)
	if len(exits) != 1 || exits[0].Code != launchoutcome.CodeExitSignalDeath || exits[0].Fields["transient"] != "true" || exits[0].Fields["ctx_cancelled"] != "true" ||
		exits[0].Fields["exit_code"] != "-1" || exits[0].Fields["cause_code"] != "driver_error" || exits[0].Reason != err.Error() {
		t.Fatalf("a cancelled signal death: transient, ctx_cancelled, driver_error: %+v", exits)
	}
	if _, has := exits[0].Fields["launch_error"]; has {
		t.Fatal("no stderr → no launch-error file → no launch_error field")
	}
	deps, got, _ = recordingDeps(t)
	_, _, _ = launchThroughFixture(t, context.Background(), deps, launchFixtureScript(ExitCmdTimeout, "empty"), core.BridgeRequest{})
	exits = exitEvents(*got)
	if len(exits) != 1 || exits[0].Code != launchoutcome.CodeExitCommandTimeout || exits[0].Fields["transient"] != "true" || exits[0].Fields["ctx_cancelled"] != "false" {
		t.Fatalf("a live 124 is transient without a cancelled context — the two fields are distinct facts: %+v", exits)
	}
}

// Test 47 — a successful launch emits no BRIDGE_EXIT_* and no unit-10 step code.
func TestEngineLaunch_ExitOK_EmitsNoBridgeExitSignal(t *testing.T) {
	deps, got, _ := recordingDeps(t)
	deps.BootTimeoutStore = clihealth.NewStore(t.TempDir(), nil)
	if _, _, err := launchThroughFixture(t, context.Background(), deps, launchFixtureScript(ExitOK, "first-bridge-line", "artifact=both"), core.BridgeRequest{}); err != nil {
		t.Fatal(err)
	}
	for _, e := range *got {
		if strings.HasPrefix(string(e.Code), "BRIDGE_EXIT_") || e.Code == CodeBootStrikeClearFailed || e.Code == CodeBootStrikeRecordFailed ||
			e.Code == CodeLaunchErrorPersistFailed || e.Code == CodeResultReadFailed || e.Fields["step"] != "" {
			t.Fatalf("success emits nothing of the unit's: %+v", e)
		}
	}
}

// Test 48 — the declared stream order per path, asserted directly (the
// edited golden pins the same sequences): the step failures precede the ONE
// exit event; on exit 80 the strike record comes after the persist and before
// the exit event.
func TestEngineLaunch_StepFailuresPrecedeTheExitSignal(t *testing.T) {
	deps, got, _ := recordingDeps(t)
	deps.BootTimeoutStore = brokenBootStrikeStore(t)
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "build-launch-error.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, _, _ = launchThroughFixture(t, context.Background(), deps, launchFixtureScript(ExitREPLBootTimeout, "first-bridge-line"), core.BridgeRequest{Workspace: ws})
	want := []string{"BRIDGE_TOKEN_RESOLVER_MISSING", "BRIDGE_LAUNCH_ERROR_PERSIST_FAILED", "BRIDGE_BOOT_STRIKE_RECORD_FAILED", "BRIDGE_EXIT_REPL_BOOT_TIMEOUT"}
	if got := codesOf(*got); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("exit 80 stream = %v, want %v", got, want)
	}
	deps, got, _ = recordingDeps(t)
	deps.BootTimeoutStore = brokenBootStrikeStore(t)
	_, _, _ = launchThroughFixture(t, context.Background(), deps, launchFixtureScript(ExitArtifactTimeout, "marker-submit-wedged"), core.BridgeRequest{})
	want = []string{"BRIDGE_TOKEN_RESOLVER_MISSING", "BRIDGE_BOOT_STRIKE_CLEAR_FAILED", "BRIDGE_EXIT_ARTIFACT_TIMEOUT"}
	if got := codesOf(*got); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("exit 81 stream = %v, want %v", got, want)
	}
}

func codesOf(events []signalcenter.Event) []string {
	out := []string{}
	for _, e := range events {
		out = append(out, string(e.Code))
	}
	return out
}

// Test 55 — ONE producer helper (review fold, F5): warn(code, detail) is
// launchWarn with its historical origin and no step, and launchWarn omits
// the step key when the step is empty — so the four telemetry codes keep
// the exact payload they had ({call_id, cli, agent}) while every unit-10
// event carries its step. Both shapes are asserted on the same context.
func TestLaunchWarn_EmptyStepIsOmitted_WarnIsLaunchWarn(t *testing.T) {
	c, got := recordingSignals()
	ctx := attemptLogContext{signals: c, dispatchIdentity: dispatchIdentity{cycle: 3, runID: "run-3", phase: "build"}, callID: "call-9", cli: "codex", agent: "build", attempt: 1}
	ctx.launchWarn("Engine.step", "", CodeResultReadFailed, "no step", nil)
	ctx.warn(CodeTokenUsageWarning, "a caveat")
	ctx.launchWarn("Engine.readResult", "read_result", CodeResultReadFailed, "with step", map[string]string{"path": "p"})
	if len(*got) != 3 {
		t.Fatalf("three events, got %d: %+v", len(*got), *got)
	}
	for i, e := range (*got)[:2] {
		if _, has := e.Fields["step"]; has {
			t.Errorf("event %d: an empty step is omitted, not written empty: %+v", i, e.Fields)
		}
		if len(e.Fields) != 3 || e.Fields["call_id"] != "call-9" || e.Fields["cli"] != "codex" || e.Fields["agent"] != "build" {
			t.Errorf("event %d: the payload is exactly the call identity: %+v", i, e.Fields)
		}
	}
	if w := (*got)[1]; w.Origin != "attemptLogContext.warn" || w.Code != CodeTokenUsageWarning || w.Reason != "a caveat" || w.Kind != signalcenter.KindBridgeWarning || w.Cycle != 3 {
		t.Errorf("warn keeps its origin, kind and identity: %+v", w)
	}
	if s := (*got)[2]; s.Fields["step"] != "read_result" || s.Fields["path"] != "p" || len(s.Fields) != 5 {
		t.Errorf("a named step and its fields ride beside the identity: %+v", s.Fields)
	}
}

// Test 56 — the four step codes' registered docs name the FULL payload the
// event carries (review fold, Go MINOR): the step, the step's own fields and
// the call identity every launchWarn event rides (call_id, cli, agent) — a
// triage reading signal-codes.md sees what the line will hold.
func TestLaunchStepCodes_DocsNameTheFullPayload(t *testing.T) {
	for code, own := range map[signalcenter.Code]string{
		CodeBootStrikeClearFailed:    "fields step=clear_boot_strike, ",
		CodeBootStrikeRecordFailed:   "fields step=record_boot_strike, ",
		CodeLaunchErrorPersistFailed: "fields step=persist_launch_error, path, ",
		CodeResultReadFailed:         "fields step=read_result, path, completion, ",
	} {
		doc := registeredDoc(t, code)
		if !strings.HasSuffix(doc, own+"call_id, cli, agent") {
			t.Errorf("%s: doc must end with %q; got %q", code, own+"call_id, cli, agent", doc)
		}
	}
}

// registeredDoc returns the registry doc of one bridge code.
func registeredDoc(t *testing.T, code signalcenter.Code) string {
	t.Helper()
	for _, d := range signalcenter.RegisteredCodes()[signalcenter.ModuleBridge] {
		if d.Code == code {
			return d.Doc
		}
	}
	t.Fatalf("%s is not registered under bridge", code)
	return ""
}
