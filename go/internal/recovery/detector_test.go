package recovery

import "testing"

// The pane fixtures are verbatim captures of real fatal panes; keep them unedited.
const (
	paneClaudeModelError = `user@host evolve-loop % claude --model auto --dangerously-skip-permissions
⏺ There's an issue with the selected model (auto). It may not exist or you may not have access to it. Run /model to pick a different model.`

	paneCodexSelfUpdate = `│ Update ran successfully! Please restart Codex to use the new version.
user@host evolve-loop %`

	paneDeadShell = `user@host evolve-loop %
zsh: command not found: Please
user@host evolve-loop %`

	paneHealthyClaude = `⏺ Reading go/internal/core/orchestrator.go…
  ⎿ 120 lines
✶ Deliberating… (esc to interrupt)`
)

func TestSeedDetector_KnownSignatures(t *testing.T) {
	t.Parallel()
	d := SeedDetector()
	cases := []struct {
		name string
		pane string
		want TerminalCause
	}{
		{"claude_model_invalid", paneClaudeModelError, CauseModelInvalid},
		{"codex_self_update", paneCodexSelfUpdate, CauseCLISelfUpdated},
		{"dead_shell_nudge_echo", paneDeadShell, CauseDeadShell},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cause, sig, ok := d.Detect(tc.pane)
			if !ok {
				t.Fatalf("seeded detector must recognize the %s pane (cycle-262 burned maxExtends on it)", tc.name)
			}
			if cause != tc.want {
				t.Errorf("cause=%s, want %s", cause, tc.want)
			}
			if sig == "" {
				t.Errorf("matched signature must be reported for the justification trail")
			}
		})
	}
}

func TestDetect_UnknownPane_NotFatal(t *testing.T) {
	t.Parallel()
	cause, _, ok := SeedDetector().Detect(paneHealthyClaude)
	if ok {
		t.Fatalf("a healthy working pane must not classify as fatal (got cause=%s) — false positives kill live agents", cause)
	}
}

func TestDetect_FirstMatchWins(t *testing.T) {
	t.Parallel()
	d := NewFatalPaneDetector([]FatalSignature{
		{Substr: "shared marker", Cause: CauseModelInvalid},
		{Substr: "shared marker", Cause: CauseDeadShell},
	})
	cause, _, ok := d.Detect("xx shared marker xx")
	if !ok || cause != CauseModelInvalid {
		t.Fatalf("ordered registry: first matching signature must win; got cause=%v ok=%v", cause, ok)
	}
}

func TestDetect_EmptyPane_NotFatal(t *testing.T) {
	t.Parallel()
	if _, _, ok := SeedDetector().Detect(""); ok {
		t.Fatal("empty pane must not classify as fatal")
	}
}

func TestTerminalCause_SessionRecoverable(t *testing.T) {
	for cause, want := range map[TerminalCause]bool{
		CauseDeadShell:      true,
		CauseCLISelfUpdated: true,
		CauseModelInvalid:   false,
		CauseUnknown:        false,
		"":                  false,
	} {
		if got := cause.SessionRecoverable(); got != want {
			t.Errorf("%q.SessionRecoverable() = %v, want %v", cause, got, want)
		}
	}
}
