//go:build acs

// Package cycle1334 materializes the cycle-1334 acceptance criteria for this
// fleet lane's sole assigned todo-id, pipeline-repair-consumption-ack-wiring
// (per R9.3 no predicates bind to any other lane's items).
//
// Incident recap: cycle-1332 wired the MANUAL ack branch (`evolve loop
// --reset --fingerprint <fp>` — cmd_loop_fingerprint_ack_test.go) but
// explicitly deferred the transactional-consumption branch, whose
// production caller did not exist as code. The sibling P1 inbox item
// (2026-08-05T09-40-00Z-recurrence-ack-for-consumed-p0.json) names that gap
// literally: a pipeline-defect P0 item's `consumed_by` narrative (or, before
// a narrative is written, its auto-filed `notes` field) already carries the
// failure fingerprint as free text, but nothing parses it back out to write
// .evolve/resolved-fingerprints.json — the operator has to hand-retype it
// into --reset --fingerprint, "exactly the toil-and-tamper-surface the
// sanctioned flows exist to avoid" (the item's own words).
//
// AC map (1:1, from scout-report.md Task 1 "recurrence-ack-consumption-
// wiring" + its "Acceptance Criteria Summary" + the P1 item's explicit
// wiring-proof ask):
//
//	AC1 core.ParseConsumptionFingerprint(text) extracts the fingerprint
//	    triplet from the unquoted `fingerprint X` shape a consumed_by
//	    narrative uses.
//	    → C1334_001.
//	AC2 Semantic: the SAME parser also extracts from the quoted
//	    `fingerprint "X"` shape the auto-filed notes field uses — a
//	    distinct text shape, not the same behavior restated.
//	    → C1334_002.
//	AC3 Negative: free text with no `fingerprint` token returns ok=false —
//	    fails closed, never invents a fingerprint.
//	    → C1334_003.
//	AC4 Negative/precision: a pipe-delimited substring with no literal
//	    `fingerprint` keyword must not false-match (anchors on the token,
//	    not "any triplet-shaped text").
//	    → C1334_004.
//	AC5 core.ConsumePipelineDefectFingerprint writes an ack-ledger record
//	    (via AppendResolvedFingerprint) atomically from a consumed_by
//	    narrative — the literal fix this cycle's task selects.
//	    → C1334_005.
//	AC6 Semantic: the SAME helper falls back to the notes field when
//	    consumed_by is empty (an item consumed before a narrative exists) —
//	    a second, distinct extraction path.
//	    → C1334_006.
//	AC7 Negative: neither field carries a fingerprint → error, and NO
//	    ledger record written (never silently swallows an undiagnosed P0).
//	    → C1334_007.
//	AC8 Rule B integration — the wiring-proof fixture named verbatim in the
//	    inbox item's `fix` field: 3x identical-fingerprint failure records
//	    + a resolved-fingerprints.json ack write via the new consumption
//	    helper → Rule B does NOT halt; the SAME fixture without the
//	    consumption call → Rule B DOES halt (ceiling unweakened).
//	    → C1334_008.
//	AC9 Caller proof: `evolve inbox ack-fingerprint <item-path>` — driven
//	    from the REAL production entrypoint (runInbox, cmd/evolve) —
//	    reaches ConsumePipelineDefectFingerprint and writes the ledger. A
//	    predicate that only calls the core helper directly proves nothing
//	    about the CLI surface actually being wired (house rule: a wiring
//	    proof is a reachability test).
//	    → C1334_009.
//	AC10 Semantic: the SAME CLI subcommand also falls back to notes,
//	    exercising AC6's path from the real entrypoint.
//	    → C1334_010.
//	AC11 Negative: a missing item path returns a nonzero exit and writes NO
//	    ledger file.
//	    → C1334_011.
//	AC12 Negative: a real item with no parseable fingerprint in either field
//	    returns a nonzero exit and writes NO ledger file.
//	    → C1334_012.
//
// "Both: existing --reset --fingerprint manual path remains byte-identical"
// (scout-report.md Acceptance Criteria Summary) is covered by the EXISTING
// cycle-1332 regression tests (TestEvaluateBlockerBreaker_*,
// TestAppendResolvedFingerprint_WritesRecord,
// TestRunLoop_FingerprintAck_AppendsLedgerRecord) — this cycle adds a NEW
// writer path beside them without touching their code paths, so no new
// predicate is needed; disposition = pre-existing GREEN, verified
// unmodified (see test-report.md Coverage Map).
//
// Adversarial axes: negative (AC3/AC4/AC7/AC11/AC12), edge (AC3 empty
// match, AC4 near-miss text), semantic (AC1 vs AC2 two distinct text
// shapes; AC5 vs AC6 two distinct extraction paths; AC9/AC10 mirror AC5/AC6
// from the real CLI entrypoint — not one behavior restated four times). No
// source-grep predicates (cycle-85 rule): every predicate here runs the
// system under test as a subprocess (named unit test) or the SSOT eval
// quality checker — none asserts on source-file text alone.
package cycle1334

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	evalSlug = "pipeline-defect-consumption-fingerprint-ack"
	corePkg  = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	cmdPkg   = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
)

// requireNamedPass runs `go test -run <pattern> <pkg>` as a subprocess and
// requires the exact named test(s) to report a verbose PASS marker. This is
// the RED mechanism: today none of these names exist (compile failure) or
// they exist but do not yet report PASS (runtime failure) — either way
// stdout carries no "--- PASS: <name>" line and the loop below fails every
// predicate, a clean, well-diagnosed RED (cycle1313/cycle1332 precedent).
func requireNamedPass(t *testing.T, pkg, pattern string, names []string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", pattern, pkg)
	if code != 0 || err != nil {
		t.Logf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pattern, pkg, code, err, stdout, stderr)
	}
	for _, name := range names {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("test %s did not report PASS in %s (missing, unimplemented, or regressed)", name, pkg)
		}
	}
}

// AC1: the free-text parser extracts the fingerprint from the unquoted
// consumed_by shape.
func TestC1334_001_parse_consumption_fingerprint_from_consumed_by(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestParseConsumptionFingerprint_ExtractsFromUnquotedConsumedBy$",
		[]string{"TestParseConsumptionFingerprint_ExtractsFromUnquotedConsumedBy"})
}

// AC2: the SAME parser handles the quoted notes shape.
func TestC1334_002_parse_consumption_fingerprint_from_notes(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestParseConsumptionFingerprint_ExtractsFromQuotedNotes$",
		[]string{"TestParseConsumptionFingerprint_ExtractsFromQuotedNotes"})
}

// AC3 (negative): no fingerprint token → ok=false.
func TestC1334_003_parse_consumption_fingerprint_no_match(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestParseConsumptionFingerprint_NoMatchReturnsFalse$",
		[]string{"TestParseConsumptionFingerprint_NoMatchReturnsFalse"})
}

// AC4 (negative/precision): a pipe-delimited substring with no literal
// `fingerprint` keyword must not false-match.
func TestC1334_004_parse_consumption_fingerprint_ignores_unrelated_pipes(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestParseConsumptionFingerprint_IgnoresUnrelatedPipeDelimitedText$",
		[]string{"TestParseConsumptionFingerprint_IgnoresUnrelatedPipeDelimitedText"})
}

// AC5: the consumption helper writes an ack-ledger record from consumed_by.
func TestC1334_005_consume_pipeline_defect_fingerprint_writes_ledger(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestConsumePipelineDefectFingerprint_WritesLedgerFromConsumedBy$",
		[]string{"TestConsumePipelineDefectFingerprint_WritesLedgerFromConsumedBy"})
}

// AC6: the SAME helper falls back to notes when consumed_by is empty.
func TestC1334_006_consume_pipeline_defect_fingerprint_notes_fallback(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestConsumePipelineDefectFingerprint_FallsBackToNotesWhenConsumedByEmpty$",
		[]string{"TestConsumePipelineDefectFingerprint_FallsBackToNotesWhenConsumedByEmpty"})
}

// AC7 (negative): neither field carries a fingerprint → error, no ledger
// write.
func TestC1334_007_consume_pipeline_defect_fingerprint_errors_without_match(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestConsumePipelineDefectFingerprint_ErrorsWhenNoFingerprintFound$",
		[]string{"TestConsumePipelineDefectFingerprint_ErrorsWhenNoFingerprintFound"})
}

// AC8: the wiring-proof fixture named verbatim in the P1 item's `fix`
// field — 3x identical fingerprints halt Rule B; consuming (acking) the
// fingerprint excludes it on the next evaluation.
func TestC1334_008_consume_pipeline_defect_fingerprint_integrates_rule_b(t *testing.T) {
	requireNamedPass(t, corePkg,
		"^TestConsumePipelineDefectFingerprint_IntegratesWithRuleBExclusion$",
		[]string{"TestConsumePipelineDefectFingerprint_IntegratesWithRuleBExclusion"})
}

// AC9: caller proof — `evolve inbox ack-fingerprint <item-path>`, driven
// through the real runInbox entrypoint, reaches the ledger writer.
func TestC1334_009_run_inbox_ack_fingerprint_writes_ledger(t *testing.T) {
	requireNamedPass(t, cmdPkg,
		"^TestRunInbox_AckFingerprint_WritesLedgerFromRealItem$",
		[]string{"TestRunInbox_AckFingerprint_WritesLedgerFromRealItem"})
}

// AC10: the SAME CLI subcommand falls back to notes from the real
// entrypoint.
func TestC1334_010_run_inbox_ack_fingerprint_notes_fallback(t *testing.T) {
	requireNamedPass(t, cmdPkg,
		"^TestRunInbox_AckFingerprint_FallsBackToNotesField$",
		[]string{"TestRunInbox_AckFingerprint_FallsBackToNotesField"})
}

// AC11 (negative): a missing item path returns nonzero and writes no
// ledger.
func TestC1334_011_run_inbox_ack_fingerprint_missing_item(t *testing.T) {
	requireNamedPass(t, cmdPkg,
		"^TestRunInbox_AckFingerprint_MissingItemReturnsNonZero$",
		[]string{"TestRunInbox_AckFingerprint_MissingItemReturnsNonZero"})
}

// AC12 (negative): a real item with no parseable fingerprint returns
// nonzero and writes no ledger.
func TestC1334_012_run_inbox_ack_fingerprint_no_fingerprint(t *testing.T) {
	requireNamedPass(t, cmdPkg,
		"^TestRunInbox_AckFingerprint_NoFingerprintInItemReturnsNonZero$",
		[]string{"TestRunInbox_AckFingerprint_NoFingerprintInItemReturnsNonZero"})
}

// AC-support: the permanent eval entry (.evolve/evals/<slug>.md) exists and
// passes the SSOT quality checker with a non-empty, real-command evidence
// set (closes the vacuous-empty-eval hole — Step 6b of the TDD contract).
func TestC1334_013_eval_file_passes_quality_check(t *testing.T) {
	evalPath := filepath.Join(acsassert.RepoRoot(t), ".evolve", "evals", evalSlug+".md")
	res, err := evalqualitycheck.Check(evalqualitycheck.Options{Path: evalPath})
	if err != nil {
		t.Fatalf("eval quality-check %s: %v", evalPath, err)
	}
	if res.Overall != evalqualitycheck.LevelPass {
		for _, c := range res.Commands {
			if c.Level != evalqualitycheck.LevelPass {
				t.Errorf("eval command %q classified level %d: %s", c.Line, c.Level, c.Reason)
			}
		}
		t.Fatalf("eval %s overall level %d, want PASS(0)", evalPath, res.Overall)
	}
	if len(res.Commands) < 2 {
		t.Fatalf("eval %s has %d classifiable command(s), want >=2 (vacuous-empty-eval guard)", evalPath, len(res.Commands))
	}
}
