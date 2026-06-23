package recovery

import "testing"

func TestFatalPaneShellSpill(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		pane string
	}{
		{"zsh_command_not_found", "user@host % Fix every violation\nzsh: command not found: Fix\n"},
		{"bquote_continuation", "user@host % knowledge-base/research/`\nbquote>\nbquote> # Evolve Architecture Designer\n"},
		{"quote_continuation", "user@host % 'unterminated\nquote>\nquote> still in shell\n"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cause, sig, ok := SeedDetector().Detect(tc.pane)
			if !ok {
				t.Fatalf("shell spill pane did not classify fatal")
			}
			if cause != CauseDeadShell {
				t.Fatalf("cause=%s sig=%q, want %s", cause, sig, CauseDeadShell)
			}
		})
	}
}

func TestFatalPaneNoFalsePositive(t *testing.T) {
	t.Parallel()
	healthy := `Working on the parser.
The user mentioned a quote and a command not found example in prose.
No shell continuation prompt is active.`
	if cause, sig, ok := SeedDetector().Detect(healthy); ok {
		t.Fatalf("healthy pane classified fatal: cause=%s sig=%q", cause, sig)
	}
}
