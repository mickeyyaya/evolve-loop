package core

import (
	"testing"
	"time"
)

// realConsumedByText and realNotesText are the two real shapes from a live consumed pipeline-defect item.
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
	fp, ok := ParseConsumptionFingerprint(realNotesText)
	if !ok {
		t.Fatalf("must find a fingerprint in a real auto-filed notes field, got ok=false")
	}
	if fp != "ship|unknown|76d0f4fca190" {
		t.Fatalf("got fingerprint %q, want %q", fp, "ship|unknown|76d0f4fca190")
	}
}

func TestParseConsumptionFingerprint_NoMatchReturnsFalse(t *testing.T) {
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
	fp := "ship|unknown|76d0f4fca190"
	digests := []FailureDigest{
		dg(1329, fp, "gate-block"), dg(1330, fp, "gate-block"), dg(1331, fp, "gate-block"),
	}

	unackedCfg := defaultBreakerCfg()
	if v := EvaluateBlockerBreaker(digests, unackedCfg); !v.Halt {
		t.Fatalf("without consumption the identical fingerprint must still halt, got %+v", v)
	}

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

func TestParseConsumptionFingerprint_IgnoresUnrelatedPipeDelimitedText(t *testing.T) {
	_, ok := ParseConsumptionFingerprint("see docs/a|b|c.md for details, no fingerprint keyword here")
	if ok {
		t.Fatalf("a pipe-delimited substring with no `fingerprint` token must not match")
	}
}
