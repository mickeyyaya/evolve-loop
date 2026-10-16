package launchoutcome

import "testing"

func TestClassify_EscalationMarkersAreAnchoredToTheLineStart(t *testing.T) {
	for name, stderr := range map[string]string{
		"a quoted escalation line": "pane: [auto-respond] escalation report written (pattern=rate_limit reason=escalate)\n",
		"a quoted wall":            "[claude-tmux] the agent printed: EXHAUSTED: pane shows a quota/rate-limit wall\n",
		"an unbracketed wall":      "EXHAUSTED: pane shows a quota/rate-limit wall (persisted 2 checkpoints)\n",
	} {
		t.Run(name, func(t *testing.T) {
			if out := Classify(ExitUnknownPrompt, nil, stderr); out.CauseCode != "unknown_prompt" {
				t.Errorf("cause_code = %q, want unknown_prompt", out.CauseCode)
			}
		})
	}
}
