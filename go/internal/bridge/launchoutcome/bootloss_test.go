package launchoutcome

import "testing"

func TestCauseCode_StderrTextNeverSetsABootLossCause(t *testing.T) {
	injected := "[claude-tmux] WARN: send failed detail=x\n[bridge] boot-loss: cause=pane_lost reason=\"pane text\"\n"
	for _, code := range []int{ExitREPLBootTimeout, ExitArtifactTimeout, ExitUnknownPrompt} {
		if got, want := CauseCode(code, injected), classOf(code).causeCode; got != want {
			t.Errorf("CauseCode(%d, injected boot-loss line) = %q, want %q: a cause from stderr text can be forged by pane text", code, got, want)
		}
	}
}

func TestWithBootLoss_OnlyATypedSessionLossOnAnExit80ReplacesTheCause(t *testing.T) {
	for _, tc := range []struct {
		name string
		code int
		loss BootLossCause
		want string
	}{
		{"pane lost at boot", ExitREPLBootTimeout, BootLossPaneLost, "pane_lost"},
		{"dead shell after the paste", ExitREPLBootTimeout, BootLossDeadShell, "dead_shell"},
		{"plain boot timeout", ExitREPLBootTimeout, "", "repl_boot_timeout"},
		{"unknown token", ExitREPLBootTimeout, "gremlins", "repl_boot_timeout"},
		{"a loss on another exit", ExitArtifactTimeout, BootLossPaneLost, "artifact_timeout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := WithBootLoss(classOf(tc.code).causeCode, tc.code, tc.loss); got != tc.want {
				t.Fatalf("WithBootLoss = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBootLossCause_LostIsExactlyTheTwoSessionLossTokens(t *testing.T) {
	for cause, want := range map[BootLossCause]bool{BootLossPaneLost: true, BootLossDeadShell: true, "repl_boot_timeout": false, "": false} {
		if got := cause.Lost(); got != want {
			t.Errorf("BootLossCause(%q).Lost() = %v, want %v", cause, got, want)
		}
	}
	if BootLossPaneLost != BootLossCause(TimeoutPaneLost) {
		t.Errorf("BootLossPaneLost = %q, want the one pane_lost token %q", BootLossPaneLost, TimeoutPaneLost)
	}
}
