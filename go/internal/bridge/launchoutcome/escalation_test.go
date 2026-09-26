package launchoutcome

import (
	"strings"
	"testing"
)

// Exit 85 is one exit class (the numeric table is frozen), so the auto-responder's escalation pattern rides
// the cause, as the 81 sub-causes do: a quota wall and a rejected model are counted and routed as what they
// are, and only a prompt nobody recognised stays unknown_prompt.
func TestClassify_EscalationPatternRidesTheCause(t *testing.T) {
	const killed = "[codex-tmux] session killed: evolve-bridge-codex-r01M3ENTP-c1707-build-pid89680-n9-1790425116\n"
	for _, tc := range []struct {
		name, stderr, cause, line string
	}{
		{
			"a quota wall (cycle 1707)",
			"[codex-tmux] submit-verify: prompt still parked at the `›` input line — re-sending Enter (2/3)\n" +
				"[auto-respond] escalation report written (pattern=rate_limit reason=escalate)\n" +
				"[codex-tmux] auto-respond escalation; abandoning run\n" + killed,
			"rate_limit", "escalation report written (pattern=rate_limit reason=escalate)",
		},
		{
			"a rejected model (cycle 1706)",
			"[auto-respond] escalation report written (pattern=model_unsupported reason=escalate)\n" +
				"[codex-tmux] auto-respond escalation; abandoning run\n" + killed,
			"model_unsupported", "escalation report written (pattern=model_unsupported reason=escalate)",
		},
		{
			"a corroborated wall at a checkpoint",
			"[codex-tmux] EXHAUSTED: pane shows a quota/rate-limit wall (persisted 2 checkpoints, corroborated by live probe) — failing over to fallback CLI (exit 85)\n",
			"rate_limit", "EXHAUSTED: pane shows a quota/rate-limit wall",
		},
		{
			"a prompt nobody recognised",
			"[auto-respond] escalation report written (pattern=unknown reason=escalate)\n" + killed,
			"unknown_prompt", "escalation report written (pattern=unknown reason=escalate)",
		},
		{
			"the loop guard, not an escalation",
			"[auto-respond] escalation report written (pattern=rate_limit reason=loop_guard)\n" + killed,
			"unknown_prompt", "",
		},
		{"no escalation line at all", killed, "unknown_prompt", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := Classify(ExitUnknownPrompt, nil, tc.stderr)

			if out.CauseCode != tc.cause {
				t.Errorf("cause_code = %q, want %q", out.CauseCode, tc.cause)
			}
			if out.Signal != CodeExitUnknownPrompt || !out.Transient {
				t.Errorf("the exit class must not change: signal %q transient %v", out.Signal, out.Transient)
			}
			if tc.line != "" && !strings.Contains(out.Err.Error(), tc.line) {
				t.Errorf("the launch error must name the escalation: %q", out.Err)
			}
			if got := CauseCode(ExitUnknownPrompt, tc.stderr); got != tc.cause {
				t.Errorf("CauseCode = %q, want %q (the ledger projection must agree)", got, tc.cause)
			}
		})
	}
}

// Other exits never borrow the escalation rule, even with an escalation line in their stderr.
func TestClassify_EscalationPatternIsScopedToExit85(t *testing.T) {
	stderr := "[auto-respond] escalation report written (pattern=rate_limit reason=escalate)\n"

	if out := Classify(ExitRespondLoopGuard, nil, stderr); out.CauseCode != "respond_loop_guard" {
		t.Errorf("exit 86 cause_code = %q, want respond_loop_guard", out.CauseCode)
	}
}
