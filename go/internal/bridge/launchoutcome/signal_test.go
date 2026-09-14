package launchoutcome

// signal_test.go — the BRIDGE_EXIT_* projection, Outcome.Signal (design §6
// tests 29-30).

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func assertRegisteredBridgeCode(t *testing.T, code signalcenter.Code) {
	t.Helper()
	if m, ok := signalcenter.IsRegistered(code); !ok || m != signalcenter.ModuleBridge {
		t.Errorf("%s must be registered under module bridge (owner %q, registered %v)", code, m, ok)
	}
}

// Test 29 — Outcome.Signal is the table's signal column; the naming RULE
// code == "BRIDGE_EXIT_" + upper(name) holds for every row; every code is
// valid and registered under bridge; success has no code; exactly 12 codes
// carry the BRIDGE_EXIT_ prefix.
func TestOutcomeSignal_IsTheTableColumnAndFollowsTheNamingRule(t *testing.T) {
	rows := append([]exitClass{}, exitClasses...)
	rows = append(rows, driverErrorClass)
	for _, row := range rows {
		want := signalcenter.Code("BRIDGE_EXIT_" + strings.ToUpper(row.name))
		if row.signal != want {
			t.Errorf("row %d (%s): signal %s, want %s", row.code, row.name, row.signal, want)
		}
		if !row.signal.Valid() || !row.signal.BelongsTo(signalcenter.ModuleBridge) {
			t.Errorf("row %d: %s malformed or not a bridge code", row.code, row.signal)
		}
		assertRegisteredBridgeCode(t, row.signal)
	}
	for code, want := range map[int]signalcenter.Code{
		ExitSafetyGate: CodeExitSafetyGate, ExitCostLeak: CodeExitCostLeak, ExitBadFlags: CodeExitBadFlags,
		ExitREPLBootTimeout: CodeExitREPLBootTimeout, ExitArtifactTimeout: CodeExitArtifactTimeout,
		ExitUnknownPrompt: CodeExitUnknownPrompt, ExitRespondLoopGuard: CodeExitRespondLoopGuard,
		ExitRequireFullUnmet: CodeExitRequiredTierUnavailable, ExitCmdTimeout: CodeExitCommandTimeout,
		ExitMissingBinary: CodeExitMissingBinary, ExitSignalDeath: CodeExitSignalDeath, 42: CodeExitDriverError,
	} {
		if got := Classify(code, nil, "").Signal; got != want {
			t.Errorf("Classify(%d).Signal = %s, want %s", code, got, want)
		}
	}
	if got := Classify(ExitOK, nil, "").Signal; got != "" {
		t.Errorf("Classify(0).Signal = %q, want \"\" (success carries no code)", got)
	}
	registered := 0
	for _, docs := range signalcenter.RegisteredCodes() {
		for _, d := range docs {
			if strings.HasPrefix(string(d.Code), "BRIDGE_EXIT_") {
				registered++
				if d.Doc == "" {
					t.Errorf("%s registered without a doc", d.Code)
				}
			}
		}
	}
	if registered != 12 {
		t.Errorf("BRIDGE_EXIT_ codes registered = %d, want 12", registered)
	}
}

// Test 30 — 81 is ONE code regardless of the marker's sub-cause.
func TestOutcomeSignal_81_IsOneCodeRegardlessOfSubCause(t *testing.T) {
	for _, stderr := range []string{"", "[bridge] artifact-timeout: cause=submit_wedged phase=x\n", "[bridge] artifact-timeout: cause=incomplete\n"} {
		if out := Classify(ExitArtifactTimeout, nil, stderr); out.Signal != CodeExitArtifactTimeout || out.ExitCode != ExitArtifactTimeout {
			t.Errorf("81 projects onto BRIDGE_EXIT_ARTIFACT_TIMEOUT whatever the sub-cause (stderr %q)", stderr)
		}
	}
	if CauseCode(ExitArtifactTimeout, "[bridge] artifact-timeout: cause=submit_wedged phase=x\n") != "submit_wedged" {
		t.Error("the sub-cause rides the cause code, not the signal code")
	}
}
