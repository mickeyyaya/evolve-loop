package recovery

import "testing"

func TestShellSpill_NewlinePrefixBoundary(t *testing.T) {
	t.Parallel()
	d := SeedDetector()
	cases := []struct {
		name      string
		pane      string
		wantFatal bool
	}{
		{
			name:      "quote_at_pane_start_no_newline",
			pane:      "quote> still in shell continuation",
			wantFatal: false,
		},
		{
			name:      "bquote_at_pane_start_no_newline",
			pane:      "bquote> back-quoted continuation",
			wantFatal: false,
		},
		{
			name:      "quote_midline_no_newline_before",
			pane:      "the agent said: quote> foo",
			wantFatal: false,
		},
		{
			name:      "quote_newline_prefix_matches",
			pane:      "user@host evolve-loop % echo 'unterminated\nquote> continuation",
			wantFatal: true,
		},
		{
			name:      "bquote_newline_prefix_matches",
			pane:      "user@host evolve-loop % echo `backtick\nbquote> continuation",
			wantFatal: true,
		},
		{
			name:      "quote_word_only_no_gt",
			pane:      "the term 'quote' appears in prose\nnot a continuation prompt",
			wantFatal: false,
		},
		{
			name:      "bquote_word_only_no_gt",
			pane:      "bquote is a zsh term but this line has no continuation marker",
			wantFatal: false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cause, sig, ok := d.Detect(tc.pane)
			if tc.wantFatal && !ok {
				t.Fatalf("pane %q expected fatal but Detect returned ok=false", tc.pane)
			}
			if !tc.wantFatal && ok {
				t.Fatalf("pane %q expected NOT fatal but Detect returned cause=%s sig=%q", tc.pane, cause, sig)
			}
			if tc.wantFatal && cause != CauseDeadShell {
				t.Fatalf("pane %q: got cause=%s, want %s", tc.pane, cause, CauseDeadShell)
			}
		})
	}
}

// Despite the name, this pins that matching is case-sensitive.
func TestShellSpill_CaseInsensitiveBoundary(t *testing.T) {
	t.Parallel()
	d := SeedDetector()
	nonMatching := []struct {
		name string
		pane string
	}{
		{"QUOTE_upper", "user@host % test\nQUOTE> should not match"},
		{"BQUOTE_upper", "user@host % test\nBQUOTE> should not match"},
		{"Quote_mixed", "user@host % test\nQuote> mixed-case should not match"},
	}
	for _, tc := range nonMatching {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if cause, sig, ok := d.Detect(tc.pane); ok {
				t.Fatalf("case-variant %q falsely classified fatal: cause=%s sig=%q", tc.pane, cause, sig)
			}
		})
	}
}

func TestShellSpill_CommandNotFoundPrecedence(t *testing.T) {
	t.Parallel()
	d := SeedDetector()
	pane := "user@host % codex-nudge\nzsh: command not found: codex-nudge\nbquote> "
	cause, sig, ok := d.Detect(pane)
	if !ok {
		t.Fatal("pane with both command-not-found and bquote> must classify fatal")
	}
	if cause != CauseDeadShell {
		t.Fatalf("cause=%s, want %s", cause, CauseDeadShell)
	}
	if sig != ": command not found" {
		t.Fatalf("first-match must be the earlier registry entry; got sig=%q", sig)
	}
}

func TestShellSpill_MultilineWithHealthyLead(t *testing.T) {
	t.Parallel()
	d := SeedDetector()
	pane := "⏺ Reading go/internal/core/orchestrator.go…\n  ⎿ 120 lines\n✶ Deliberating… (esc to interrupt)\nuser@host % echo '\nquote> still in shell"
	cause, sig, ok := d.Detect(pane)
	if !ok {
		t.Fatal("pane with healthy header followed by quote> continuation must still classify fatal")
	}
	if cause != CauseDeadShell {
		t.Fatalf("cause=%s, want %s", cause, CauseDeadShell)
	}
	_ = sig
}

func TestDetect_NilDetector_SafeNoMatch(t *testing.T) {
	t.Parallel()
	var d *FatalPaneDetector
	if _, _, ok := d.Detect("user@host % foo\nbquote> "); ok {
		t.Fatal("nil detector must return ok=false; it is nil-receiver safe")
	}
}
