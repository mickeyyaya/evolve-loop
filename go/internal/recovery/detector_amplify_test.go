package recovery

// detector_amplify_test.go — adversarial boundary tests for the cycle-276
// shell-spill signatures ("\nquote>" and "\nbquote>"). The builder's
// shell_spill_test.go proves positive + basic false-positive cases; these
// tests probe the newline-anchor contract that silently governs the detection
// surface: a pane that STARTS with "quote>" (no preceding \n) or has "quote>"
// inline without a newline prefix must NOT classify fatal.

import "testing"

// TestShellSpill_NewlinePrefixBoundary verifies that the \n anchor in the
// seeded signatures is load-bearing. Both "\nquote>" and "\nbquote>" require a
// newline immediately before the continuation prompt — bare "quote>" at pane
// start does NOT satisfy the substring match.
func TestShellSpill_NewlinePrefixBoundary(t *testing.T) {
	t.Parallel()
	d := SeedDetector()
	cases := []struct {
		name      string
		pane      string
		wantFatal bool
	}{
		{
			// Pane starts with "quote>" — no preceding \n — must NOT match.
			name:      "quote_at_pane_start_no_newline",
			pane:      "quote> still in shell continuation",
			wantFatal: false,
		},
		{
			// Same for bquote> at pane start.
			name:      "bquote_at_pane_start_no_newline",
			pane:      "bquote> back-quoted continuation",
			wantFatal: false,
		},
		{
			// "quote>" embedded mid-line without newline before it — no match.
			name:      "quote_midline_no_newline_before",
			pane:      "the agent said: quote> foo",
			wantFatal: false,
		},
		{
			// "\nquote>" on its own line (newline present) — MUST match.
			name:      "quote_newline_prefix_matches",
			pane:      "danleemh@host evolve-loop % echo 'unterminated\nquote> continuation",
			wantFatal: true,
		},
		{
			// "\nbquote>" on its own line — MUST match.
			name:      "bquote_newline_prefix_matches",
			pane:      "danleemh@host evolve-loop % echo `backtick\nbquote> continuation",
			wantFatal: true,
		},
		{
			// "quote>" word without ">" — must NOT match (no ">" suffix).
			name:      "quote_word_only_no_gt",
			pane:      "the term 'quote' appears in prose\nnot a continuation prompt",
			wantFatal: false,
		},
		{
			// "bquote" word without ">" — must NOT match.
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

// TestShellSpill_CaseInsensitiveBoundary ensures signatures are case-sensitive:
// "QUOTE>" and "BQUOTE>" (uppercase) must NOT trigger the detector.
func TestShellSpill_CaseInsensitiveBoundary(t *testing.T) {
	t.Parallel()
	d := SeedDetector()
	nonMatching := []struct {
		name string
		pane string
	}{
		{"QUOTE_upper", "danleemh@host % test\nQUOTE> should not match"},
		{"BQUOTE_upper", "danleemh@host % test\nBQUOTE> should not match"},
		{"Quote_mixed", "danleemh@host % test\nQuote> mixed-case should not match"},
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

// TestShellSpill_CommandNotFoundPrecedence verifies that when a pane contains
// both ": command not found" and "\nbquote>", the first match (command-not-found
// — earlier in the registry) wins. Both map to CauseDeadShell so the outcome is
// the same, but the matched signature must be the earlier entry.
func TestShellSpill_CommandNotFoundPrecedence(t *testing.T) {
	t.Parallel()
	d := SeedDetector()
	pane := "danleemh@host % codex-nudge\nzsh: command not found: codex-nudge\nbquote> "
	cause, sig, ok := d.Detect(pane)
	if !ok {
		t.Fatal("pane with both command-not-found and bquote> must classify fatal")
	}
	if cause != CauseDeadShell {
		t.Fatalf("cause=%s, want %s", cause, CauseDeadShell)
	}
	// The ": command not found" signature comes first in SeedDetector — it must
	// win when both patterns are present.
	if sig != ": command not found" {
		t.Fatalf("first-match must be the earlier registry entry; got sig=%q", sig)
	}
}

// TestShellSpill_MultilineWithHealthyLead verifies that a pane whose FIRST
// lines are healthy agent output but whose LATER lines contain "\nquote>" is
// still classified fatal. Detect scans for substring presence, not line 1.
func TestShellSpill_MultilineWithHealthyLead(t *testing.T) {
	t.Parallel()
	d := SeedDetector()
	pane := "⏺ Reading go/internal/core/orchestrator.go…\n  ⎿ 120 lines\n✶ Deliberating… (esc to interrupt)\ndanleemh@host % echo '\nquote> still in shell"
	cause, sig, ok := d.Detect(pane)
	if !ok {
		t.Fatal("pane with healthy header followed by quote> continuation must still classify fatal")
	}
	if cause != CauseDeadShell {
		t.Fatalf("cause=%s, want %s", cause, CauseDeadShell)
	}
	_ = sig
}

// TestDetect_NilDetector_SafeNoMatch confirms the nil-receiver guard still
// holds with the cycle-276 signatures in play (regression-guard, not new
// behavior — detector.go:111 documents it).
func TestDetect_NilDetector_SafeNoMatch(t *testing.T) {
	t.Parallel()
	var d *FatalPaneDetector
	if _, _, ok := d.Detect("danleemh@host % foo\nbquote> "); ok {
		t.Fatal("nil detector must return ok=false; it is nil-receiver safe")
	}
}
