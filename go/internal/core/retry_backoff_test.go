package core

// retry_backoff_test.go — regression lock for composeCorrection's
// VERBATIM-INCLUSION property (cycle-1510 task
// `contract-correction-verbatim-output-fidelity`, carryover from the
// cycle-1508 audit FAIL defect M1).
//
// Honest framing, stated up front because it is the whole lesson of
// inst-L1508b: this property ALREADY HOLDS on the pre-change code
// (retry_backoff.go:12-17 concatenates the reason with `+`). These tests are
// therefore PRE-EXISTING GREEN by design, not RED. Cycle-1508 failed audit
// precisely for claiming a tautologically-green criterion as work "established"
// by a product change; the correct disposition is to say so and convert the
// unlocked property into a locked one. The value here is future-facing: any
// later refactor of composeCorrection that reformats, re-wraps, truncates,
// escapes, or normalizes the rejection reason now fails loudly instead of
// silently degrading the correction directive the phase re-dispatch depends on.
//
// Why verbatim matters. The reason string is deliverable.summarize()'s
// rendering, carrying "[code] message" tokens. contractViolationCodeRE parses
// those SAME tokens back out of the directive downstream
// (contract_escalation.go), and the re-dispatched agent is expected to read the
// literal violation text. Any lossy transform breaks both consumers at once.

import (
	"strings"
	"testing"
)

// TestComposeCorrection_CarriesReasonVerbatim asserts the reason survives
// composeCorrection byte-for-byte, across the three shapes most likely to be
// mangled by a "helpful" reformat: multi-line content, embedded [code] tokens,
// and non-ASCII bytes.
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
			got := composeCorrection(tc.reason)
			if !strings.Contains(got, tc.reason) {
				t.Errorf("composeCorrection dropped or transformed the reason.\n reason (%d bytes): %q\n output (%d bytes): %q\nthe rejection reason MUST appear byte-for-byte: downstream code re-parses its [code] tokens and the re-dispatched agent reads its literal text",
					len(tc.reason), tc.reason, len(got), got)
			}
			// The reason must appear EXACTLY ONCE — a duplicated echo is as much
			// a fidelity defect as a truncation, and would let a paraphrasing
			// implementation pass by appending the original as an afterthought.
			if n := strings.Count(got, tc.reason); n != 1 {
				t.Errorf("reason appears %d times in the directive, want exactly 1: %q", n, got)
			}
		})
	}
}

// TestComposeCorrection_FramingSurroundsTheReason pins the STRUCTURE the
// verbatim reason sits inside: rejection framing before it, remediation
// instruction after it. Without this, a degenerate implementation that returns
// the bare reason would satisfy verbatim-inclusion while destroying the
// directive's meaning.
func TestComposeCorrection_FramingSurroundsTheReason(t *testing.T) {
	t.Parallel()
	const reason = "[missing_artifact] no file at the contracted path"
	got := composeCorrection(reason)

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

// TestComposeCorrection_EmptyReasonStillProducesADirective is the EDGE row: a
// zero-length reason must not yield an empty or malformed directive. The
// re-dispatched agent still needs the framing even when the gate had nothing
// quotable to say.
func TestComposeCorrection_EmptyReasonStillProducesADirective(t *testing.T) {
	t.Parallel()
	got := composeCorrection("")
	if !strings.Contains(got, "REJECTED") || !strings.Contains(got, "contracted path") {
		t.Errorf("empty reason produced a directive missing its framing: %q", got)
	}
}
