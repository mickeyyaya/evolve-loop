package launchoutcome

// exit_test.go — the numeric contract and the ledger projection (design §6
// tests 27, 31).

import "testing"

func TestExitCodes_NumericContractUnchanged(t *testing.T) {
	for name, tc := range map[string]struct{ got, want int }{
		"ExitOK": {ExitOK, 0}, "ExitSafetyGate": {ExitSafetyGate, 2}, "ExitCostLeak": {ExitCostLeak, 3},
		"ExitBadFlags": {ExitBadFlags, 10}, "ExitREPLBootTimeout": {ExitREPLBootTimeout, 80},
		"ExitArtifactTimeout": {ExitArtifactTimeout, 81}, "ExitUnknownPrompt": {ExitUnknownPrompt, 85},
		"ExitRespondLoopGuard": {ExitRespondLoopGuard, 86}, "ExitRequireFullUnmet": {ExitRequireFullUnmet, 99},
		"ExitCmdTimeout": {ExitCmdTimeout, 124}, "ExitMissingBinary": {ExitMissingBinary, 127},
		"ExitSignalDeath": {ExitSignalDeath, -1}, "ExitModelMismatch": {ExitModelMismatch, 87},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %d, want %d", name, tc.got, tc.want)
		}
	}
}

func TestCauseCode_ProjectsTheTableColumn(t *testing.T) {
	for code, want := range map[int]string{
		ExitOK: "", ExitSafetyGate: "safety_gate", ExitCostLeak: "cost_leak", ExitBadFlags: "bad_flags",
		ExitREPLBootTimeout: "repl_boot_timeout", ExitArtifactTimeout: "artifact_timeout",
		ExitUnknownPrompt: "unknown_prompt", ExitRespondLoopGuard: "respond_loop_guard", ExitModelMismatch: "model_mismatch",
		ExitRequireFullUnmet: "required_tier_unavailable", ExitCmdTimeout: "command_timeout",
		ExitMissingBinary: "missing_binary", ExitSignalDeath: "driver_error", 42: "driver_error",
	} {
		if got := CauseCode(code, ""); got != want {
			t.Errorf("CauseCode(%d) = %q, want %q", code, got, want)
		}
	}
}
