package launchoutcome

// classify_test.go — the classifier replays the host golden captured on the
// pre-extraction code and pins each row of the exit table (design §6 tests
// 11-19). The golden is package bridge's testdata/launch-outcome.golden.json
// (a cross-package read, declared: the oracle has ONE home).

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type goldenRow struct {
	Code      int    `json:"code"`
	Ctx       string `json:"ctx"`
	Stderr    string `json:"stderr"`
	Err       string `json:"err"`
	Sentinel  string `json:"sentinel"`
	CauseCode string `json:"cause_code"`
}

// goldenStderr mirrors package bridge's launchStderrFixtures by key — the
// bytes the fixture driver printed when the golden was captured.
var goldenStderr = map[string]string{
	"empty": "",
	"first-bridge-line": "[bridge] launch: missing required (flag or env): --profile\n" +
		"[bridge] valid: plan, default\n",
	"last-line-only": "[claude-tmux] NOTE: stream_output=true is no-op for this driver\n" +
		"[claude-tmux] prompt delivered\n" +
		"[claude-tmux] FAIL: completion never signalled\n",
	"marker-submit-wedged": "[bridge] WARN: EVOLVE_SANDBOX=on but inner sandbox not applied\n" +
		"[claude-tmux] FAIL: completion never signalled\n" +
		"[claude-tmux]   audit-report.md\n" +
		"[bridge] artifact-timeout: cause=submit_wedged reason=\"prompt submit wedged\" phase=build cycle=3 driver=claude-tmux artifact=\"a.md\" waited=300s interval=300s extends_used=0 max_extends=6 last_review=none liveness=hung progressed=false busy=false transient=false detector_error=\"\"\n",
	"marker-prose": "[claude-tmux] chatter\n" +
		"[bridge] artifact-timeout: phase=build reason=\"quoted cause=submit_wedged text\" waited=20s\n",
	"long-line":   "[claude-tmux] chatter\n" + strings.Repeat("界", 301) + "\n",
	"long-marker": "[bridge] artifact-timeout: cause=incomplete " + strings.Repeat("界", 1100) + "\n",
}

func sentinelOf(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, core.ErrArtifactTimeout):
		return "artifact_timeout"
	case errors.Is(err, core.ErrTransientBridgeFailure):
		return "transient"
	}
	return "plain"
}

func ctxErrOf(state string) error {
	if state == "cancelled" {
		return context.Canceled
	}
	return nil
}

// Test 11 — every golden row: the error string, the sentinel and the ledger
// cause code through Classify / CauseCode.
func TestClassify_ReplaysTheHostGolden(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "testdata", "launch-outcome.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []goldenRow
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 182 {
		t.Fatalf("the golden has %d rows, want 182", len(rows))
	}
	for _, row := range rows {
		stderr, ok := goldenStderr[row.Stderr]
		if !ok {
			t.Fatalf("unknown fixture %q", row.Stderr)
		}
		out := Classify(row.Code, ctxErrOf(row.Ctx), stderr)
		gotErr := ""
		if out.Err != nil {
			gotErr = out.Err.Error()
		}
		if gotErr != row.Err || sentinelOf(out.Err) != row.Sentinel || out.CauseCode != row.CauseCode || CauseCode(row.Code, stderr) != row.CauseCode {
			t.Errorf("row %d/%s/%s: err=%q sentinel=%s cause=%s, want err=%q sentinel=%s cause=%s", row.Code, row.Ctx, row.Stderr, gotErr, sentinelOf(out.Err), out.CauseCode, row.Err, row.Sentinel, row.CauseCode)
		}
		if row.Code != ExitOK && (out.ExitCode != row.Code || out.CtxCancelled != (row.Ctx == "cancelled")) {
			t.Errorf("row %d/%s: ExitCode=%d CtxCancelled=%v", row.Code, row.Ctx, out.ExitCode, out.CtxCancelled)
		}
	}
}

// Test 12 — success is the zero outcome, whatever stderr says.
func TestClassify_ExitOK_IsTheZeroOutcome(t *testing.T) {
	if out := Classify(ExitOK, nil, "[bridge] anything\n"); out != (Outcome{}) {
		t.Fatalf("Classify(0) = %+v, want the zero Outcome", out)
	}
	if out := Classify(ExitOK, context.Canceled, "x"); out != (Outcome{}) {
		t.Fatalf("Classify(0, cancelled) = %+v, want the zero Outcome", out)
	}
}

// Test 13 — the table: 11 rows over the declared non-zero exits, each with a
// name, a registered bridge signal code with a doc and a cause code; no
// duplicate exit; the default row is named separately.
func TestClassify_TableCoversEveryDeclaredExit(t *testing.T) {
	want := []int{ExitSafetyGate, ExitCostLeak, ExitBadFlags, ExitREPLBootTimeout, ExitArtifactTimeout, ExitUnknownPrompt, ExitRespondLoopGuard, ExitRequireFullUnmet, ExitCmdTimeout, ExitMissingBinary, ExitSignalDeath}
	if len(exitClasses) != len(want) {
		t.Fatalf("exitClasses has %d rows, want %d", len(exitClasses), len(want))
	}
	seen := map[int]bool{}
	for i, row := range exitClasses {
		if row.code != want[i] {
			t.Errorf("row %d: code %d, want %d", i, row.code, want[i])
		}
		if seen[row.code] {
			t.Errorf("duplicate exit %d", row.code)
		}
		seen[row.code] = true
		assertClassRow(t, row)
	}
	if driverErrorClass.name != "driver_error" || driverErrorClass.causeCode != "driver_error" || driverErrorClass.sentinel != nil {
		t.Errorf("the default row is driver_error with no sentinel: %+v", driverErrorClass)
	}
	assertClassRow(t, driverErrorClass)
}

func assertClassRow(t *testing.T, row exitClass) {
	t.Helper()
	if row.name == "" || row.causeCode == "" {
		t.Errorf("row %d: name %q cause %q must be set", row.code, row.name, row.causeCode)
	}
	if !row.signal.Valid() {
		t.Errorf("row %d: signal %q malformed", row.code, row.signal)
	}
	assertRegisteredBridgeCode(t, row.signal)
}

// Test 14 — 81 wraps ErrArtifactTimeout and is NOT transient.
func TestClassify_81_WrapsErrArtifactTimeout_NotTransient(t *testing.T) {
	out := Classify(ExitArtifactTimeout, nil, "")
	if !errors.Is(out.Err, core.ErrArtifactTimeout) || errors.Is(out.Err, core.ErrTransientBridgeFailure) || out.Transient {
		t.Fatalf("81: %+v", out)
	}
	if out.Err.Error() != "bridge: launch exit=81: core: bridge artifact timeout" {
		t.Fatalf("81 wire string: %q", out.Err.Error())
	}
}

// Test 15 — the transient set is exactly {80, 85, 86, 124}; 2/3/10/99/127/42 plain.
func TestClassify_TransientSet_IsExactly80_85_86_124(t *testing.T) {
	for _, code := range []int{ExitREPLBootTimeout, ExitUnknownPrompt, ExitRespondLoopGuard, ExitCmdTimeout} {
		out := Classify(code, nil, "")
		if !out.Transient || !errors.Is(out.Err, core.ErrTransientBridgeFailure) || errors.Is(out.Err, core.ErrArtifactTimeout) {
			t.Errorf("exit %d is transient: %+v", code, out)
		}
	}
	for _, code := range []int{ExitSafetyGate, ExitCostLeak, ExitBadFlags, ExitRequireFullUnmet, ExitMissingBinary, 42} {
		out := Classify(code, nil, "")
		if out.Transient || errors.Is(out.Err, core.ErrTransientBridgeFailure) || errors.Is(out.Err, core.ErrArtifactTimeout) {
			t.Errorf("exit %d is plain: %+v", code, out)
		}
	}
}

// Test 16 — the signal death is transient only under a cancelled context;
// the ledger cause stays driver_error and the signal BRIDGE_EXIT_SIGNAL_DEATH
// either way; a cancelled context alone makes no other exit transient.
func TestClassify_SignalDeath_TransientOnlyWhenCtxCancelled(t *testing.T) {
	cancelled := Classify(ExitSignalDeath, context.Canceled, "")
	if !cancelled.Transient || !cancelled.CtxCancelled || !errors.Is(cancelled.Err, core.ErrTransientBridgeFailure) || cancelled.CauseCode != "driver_error" {
		t.Fatalf("-1 under cancel: %+v", cancelled)
	}
	if classOf(ExitSignalDeath).name != "signal_death" || cancelled.Signal != CodeExitSignalDeath {
		t.Fatal("-1 is the signal_death class")
	}
	live := Classify(ExitSignalDeath, nil, "")
	if live.Transient || live.CtxCancelled || errors.Is(live.Err, core.ErrTransientBridgeFailure) || live.CauseCode != "driver_error" || live.Signal != CodeExitSignalDeath {
		t.Fatalf("-1 live is plain: %+v", live)
	}
	other := Classify(ExitBadFlags, context.Canceled, "")
	if other.Transient || !other.CtxCancelled || errors.Is(other.Err, core.ErrTransientBridgeFailure) {
		t.Fatalf("10 under cancel stays plain, CtxCancelled recorded: %+v", other)
	}
}

// Test 17 — an unknown non-zero exit is the driver_error class.
func TestClassify_UnknownExit_IsDriverError(t *testing.T) {
	out := Classify(42, nil, "")
	if out.CauseCode != "driver_error" || out.Signal != CodeExitDriverError || classOf(42).name != "driver_error" || out.Err.Error() != "bridge: launch exit=42" {
		t.Fatalf("42: %+v", out)
	}
}

// Test 18 — on 81 the marker summary beats the first [bridge] line, and its
// typed cause becomes the cause code.
func TestClassify_81_PrefersTheMarkerSummaryOverTheFirstBridgeLine(t *testing.T) {
	out := Classify(ExitArtifactTimeout, nil, goldenStderr["marker-submit-wedged"])
	if !strings.HasPrefix(out.Err.Error(), "bridge: launch exit=81: artifact-timeout: cause=submit_wedged ") || strings.Contains(out.Err.Error(), "EVOLVE_SANDBOX") {
		t.Fatalf("81 cause is the marker summary: %v", out.Err)
	}
	if out.CauseCode != "submit_wedged" {
		t.Fatalf("81 cause code is the typed sub-cause: %q", out.CauseCode)
	}
}

// Test 19 — every other exit keeps the first [bridge] line as its cause.
func TestClassify_Non81_CauseIsTheFirstDiagnosticLine(t *testing.T) {
	out := Classify(ExitBadFlags, nil, goldenStderr["marker-submit-wedged"])
	if out.Err.Error() != "bridge: launch exit=10: [bridge] WARN: EVOLVE_SANDBOX=on but inner sandbox not applied" || out.CauseCode != "bad_flags" {
		t.Fatalf("10: %+v", out)
	}
}
