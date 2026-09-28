package core

import (
	"strings"
	"testing"
)

func TestComposeCorrection_CarriesReasonVerbatim(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		reason string
	}{
		{
			name:   "single_line_with_code_token",
			reason: "audit deliverable failed contract: [missing_section] required section '## Verdict' not found",
		},
		{
			name: "multiline_multi_violation_summarize_rendering",
			reason: "build deliverable failed contract:\n" +
				"[missing_section] required section '## Wiring Proof' not found\n" +
				"[stray_in_worktree] artifact also present at .evolve/worktrees/cycle-1/build-report.md\n" +
				"[bad_verdict] verdict token 'MAYBE' is not one of PASS|FAIL|WARN",
		},
		{
			name:   "unicode_and_punctuation",
			reason: "scout deliverable failed contract: [missing_key] key “researchBacking” absent — expected ≥1 entry (naïve façade, 100% ✗)",
		},
		{
			name:   "trailing_and_leading_whitespace_is_preserved",
			reason: "  \t[empty_artifact] artifact is zero bytes\n\n",
		},
		{
			name:   "percent_and_backslash_are_not_format_interpreted",
			reason: `[invalid_json] parse error at C:\runs\cycle-1: 100% of keys unread, want %s`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := composeCorrection(tc.reason, "")
			if !strings.Contains(got, tc.reason) {
				t.Errorf("composeCorrection dropped or transformed the reason.\n reason (%d bytes): %q\n output (%d bytes): %q\nthe rejection reason MUST appear byte-for-byte: downstream code re-parses its [code] tokens and the re-dispatched agent reads its literal text",
					len(tc.reason), tc.reason, len(got), got)
			}
			if n := strings.Count(got, tc.reason); n != 1 {
				t.Errorf("reason appears %d times in the directive, want exactly 1: %q", n, got)
			}
		})
	}
}

// Pins the structure the verbatim reason sits inside (framing before it,
// remediation instruction after it): a degenerate implementation returning
// the bare reason would satisfy verbatim-inclusion alone while destroying the
// directive's meaning.
func TestComposeCorrection_FramingSurroundsTheReason(t *testing.T) {
	t.Parallel()
	const reason = "[missing_artifact] no file at the contracted path"
	got := composeCorrection(reason, "")

	idx := strings.Index(got, reason)
	if idx < 0 {
		t.Fatalf("reason absent from directive: %q", got)
	}
	before, after := got[:idx], got[idx+len(reason):]

	if !strings.Contains(before, "REJECTED") {
		t.Errorf("no rejection framing PRECEDES the reason; prefix was %q", before)
	}
	if !strings.Contains(after, "contracted path") {
		t.Errorf("no remediation instruction FOLLOWS the reason; suffix was %q", after)
	}
	if !strings.Contains(after, "Do not change unrelated files") {
		t.Errorf("no scope constraint follows the reason; suffix was %q", after)
	}
}

func TestComposeCorrection_EmptyReasonStillProducesADirective(t *testing.T) {
	t.Parallel()
	got := composeCorrection("", "")
	if !strings.Contains(got, "REJECTED") || !strings.Contains(got, "contracted path") {
		t.Errorf("empty reason produced a directive missing its framing: %q", got)
	}
}
