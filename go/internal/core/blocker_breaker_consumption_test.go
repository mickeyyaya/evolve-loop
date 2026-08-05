package core

// blocker_breaker_consumption_test.go — RED contract for cycle-1334's
// auto-ack-on-consumption gap (scout-report.md Task 1
// "recurrence-ack-consumption-wiring", inbox item
// 2026-08-05T09-40-00Z-recurrence-ack-for-consumed-p0.json).
//
// Cycle-1332 wired the MANUAL branch of the fix (`evolve loop --reset
// --fingerprint <fp>`, cmd_loop_fingerprint_ack_test.go). The inbox item's
// "fix" field's OTHER branch — the item explicitly says "the ack fix must
// clear BOTH stores" and names transactional consumption as the eventual
// path — is still unimplemented: a pipeline-defect P0 item can be moved to
// .evolve/inbox/consumed/ carrying the failure fingerprint in its
// consumed_by narrative (or, before a narrative is written, in its
// auto-filed notes field) and NOTHING parses it back out to ack the ledger.
// The operator has to hand-retype the fingerprint into --reset
// --fingerprint — "exactly the toil-and-tamper-surface the sanctioned flows
// exist to avoid" (the inbox item's own words).
//
// This file pins the two new exported symbols that close that gap:
//
//	ParseConsumptionFingerprint(text) (fp, ok)      — free-text extractor
//	ConsumePipelineDefectFingerprint(...) (fp, err) — the ledger writer
//
// both of which do not exist yet in blocker_breaker.go — every test below
// fails to compile until Builder adds them (a clean RED, not a false pass).

import (
	"testing"
	"time"
)

// realConsumedByText and realNotesText are taken verbatim (fingerprint only
// changed to keep the fixture self-contained) from a live consumed
// pipeline-defect item on disk
// (.evolve/inbox/consumed/2026-08-05T08-30-00Z-pipeline-defect-pipeline-blocker.json)
// — the two real shapes the parser must handle, not synthetic strings.
const (
	realConsumedByText = `console-2026-08-05: fingerprint ship|unknown|76d0f4fca190 = REPO_CONTRACT_GATE blocks from the two in-flight lanes' PRE-#415 worktrees`
	realNotesText      = `Auto-filed by the ADR-0072 halt. Evidence: failure fingerprint "ship|unknown|76d0f4fca190" recurred 3× in one batch (ceiling 3) — identical failure identities cannot be distinct honest defects (rule=identical-fingerprint fingerprint=ship|unknown|76d0f4fca190)`
)

func TestParseConsumptionFingerprint_ExtractsFromUnquotedConsumedBy(t *testing.T) {
	fp, ok := ParseConsumptionFingerprint(realConsumedByText)
	if !ok {
		t.Fatalf("must find a fingerprint in a real consumed_by narrative, got ok=false")
	}
	if fp != "ship|unknown|76d0f4fca190" {
		t.Fatalf("got fingerprint %q, want %q", fp, "ship|unknown|76d0f4fca190")
	}
}

func TestParseConsumptionFingerprint_ExtractsFromQuotedNotes(t *testing.T) {
	// Edge/semantic: the notes field's shape wraps the fingerprint in
	// double quotes (`fingerprint "X"`) — a distinct shape from consumed_by,
	// not one behavior restated.
	fp, ok := ParseConsumptionFingerprint(realNotesText)
	if !ok {
		t.Fatalf("must find a fingerprint in a real auto-filed notes field, got ok=false")
	}
	if fp != "ship|unknown|76d0f4fca190" {
		t.Fatalf("got fingerprint %q, want %q", fp, "ship|unknown|76d0f4fca190")
	}
}

func TestParseConsumptionFingerprint_NoMatchReturnsFalse(t *testing.T) {
	// Negative: free text with no `fingerprint ...` token at all — the
	// parser must fail closed (ok=false), never invent one.
	_, ok := ParseConsumptionFingerprint("closed as a duplicate, no root cause recorded here")
	if ok {
		t.Fatalf("text with no fingerprint token must report ok=false")
	}
}

func TestConsumePipelineDefectFingerprint_WritesLedgerFromConsumedBy(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	fp, err := ConsumePipelineDefectFingerprint(dir, realConsumedByText, "", "inbox-consumption", now)
	if err != nil {
		t.Fatalf("ConsumePipelineDefectFingerprint: %v", err)
	}
	if fp != "ship|unknown|76d0f4fca190" {
		t.Fatalf("got fingerprint %q, want %q", fp, "ship|unknown|76d0f4fca190")
	}
	got, err := LoadResolvedFingerprints(dir)
	if err != nil {
		t.Fatalf("LoadResolvedFingerprints after consumption: %v", err)
	}
	if !got["ship|unknown|76d0f4fca190"] {
		t.Fatalf("consumption must append the fingerprint to the ack ledger, got %+v", got)
	}
}

func TestConsumePipelineDefectFingerprint_FallsBackToNotesWhenConsumedByEmpty(t *testing.T) {
	// Semantic: an item consumed before a narrative is written (consumed_by
	// still empty) must still ack from the auto-filed notes field — the
	// second of two DISTINCT extraction paths, not a restatement of the
	// consumed_by test above.
	dir := t.TempDir()
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	fp, err := ConsumePipelineDefectFingerprint(dir, "", realNotesText, "inbox-consumption", now)
	if err != nil {
		t.Fatalf("ConsumePipelineDefectFingerprint (notes fallback): %v", err)
	}
	if fp != "ship|unknown|76d0f4fca190" {
		t.Fatalf("got fingerprint %q, want %q", fp, "ship|unknown|76d0f4fca190")
	}
	got, err := LoadResolvedFingerprints(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !got["ship|unknown|76d0f4fca190"] {
		t.Fatalf("notes-fallback consumption must append to the ack ledger, got %+v", got)
	}
}

func TestConsumePipelineDefectFingerprint_ErrorsWhenNoFingerprintFound(t *testing.T) {
	// Negative: neither field carries a parseable fingerprint — must error,
	// and must NOT write a ledger record (a P0 consumed with no
	// diagnosable fingerprint is not silently swallowed into the ledger as
	// an empty/garbage entry).
	dir := t.TempDir()
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	_, err := ConsumePipelineDefectFingerprint(dir, "closed, no fingerprint recorded", "also nothing here", "inbox-consumption", now)
	if err == nil {
		t.Fatalf("must error when neither consumed_by nor notes carries a fingerprint")
	}
	got, lerr := LoadResolvedFingerprints(dir)
	if lerr != nil {
		t.Fatal(lerr)
	}
	if len(got) != 0 {
		t.Fatalf("a failed extraction must not write ANY ledger record, got %+v", got)
	}
}

func TestConsumePipelineDefectFingerprint_IntegratesWithRuleBExclusion(t *testing.T) {
	// The literal wiring-proof fixture named in the inbox item's own `fix`
	// field: 3x identical-fingerprint failure records + a
	// resolved-fingerprints.json ack write via the new consumption helper
	// → Rule B does NOT halt on relaunch; the SAME fixture without the
	// consumption call → Rule B DOES halt (ceiling unweakened).
	fp := "ship|unknown|76d0f4fca190"
	digests := []FailureDigest{
		dg(1329, fp, "gate-block"), dg(1330, fp, "gate-block"), dg(1331, fp, "gate-block"),
	}

	// Without consumption: unchanged ADR-0072 halt behavior.
	unackedCfg := defaultBreakerCfg()
	if v := EvaluateBlockerBreaker(digests, unackedCfg); !v.Halt {
		t.Fatalf("without consumption the identical fingerprint must still halt, got %+v", v)
	}

	// With consumption: the SAME item's consumed_by narrative acks the
	// ledger, and the breaker must read it back and exclude it.
	dir := t.TempDir()
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	if _, err := ConsumePipelineDefectFingerprint(dir, realConsumedByText, "", "inbox-consumption", now); err != nil {
		t.Fatalf("ConsumePipelineDefectFingerprint: %v", err)
	}
	acked, err := LoadResolvedFingerprints(dir)
	if err != nil {
		t.Fatal(err)
	}
	ackedCfg := defaultBreakerCfg()
	ackedCfg.AckedFingerprints = acked
	if v := EvaluateBlockerBreaker(digests, ackedCfg); v.Halt {
		t.Fatalf("after consumption the acked fingerprint must be excluded from Rule B, got halt: %+v", v)
	}
}

// guard against an accidental substring-only match slipping through review:
// the parser must anchor on the literal `fingerprint` token, not merely
// "contains a pipe-delimited triplet somewhere in the string".
func TestParseConsumptionFingerprint_IgnoresUnrelatedPipeDelimitedText(t *testing.T) {
	_, ok := ParseConsumptionFingerprint("see docs/a|b|c.md for details, no fingerprint keyword here")
	if ok {
		t.Fatalf("a pipe-delimited substring with no `fingerprint` token must not match")
	}
}
