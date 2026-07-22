package core

// failure_digest_named_test.go — apicover per-symbol naming (the two-signal
// convention: every exported symbol named by a test with a REAL assertion).
// RecurrenceCounter is the assembler's read-only ledger seam; this test pins
// that a custom implementation's count flows through into the digest — the
// 6th recurrence of the "exported but never named" parity class, caught on
// PR #350's CI.

import "testing"

// fixedCounter is a minimal RecurrenceCounter: every fingerprint has count n.
type fixedCounter struct{ n int }

func (f fixedCounter) Count(string) int { return f.n }

func TestRecurrenceCounter_CountFlowsIntoDigest(t *testing.T) {
	var rc RecurrenceCounter = fixedCounter{n: 4}
	dir := t.TempDir()
	digest, err := AssembleFailureDigest(1034, dir, rc)
	if err != nil {
		t.Fatal(err)
	}
	if digest.Recurrence != 4 {
		t.Fatalf("recurrence must be read THROUGH the RecurrenceCounter seam, got %d want 4", digest.Recurrence)
	}
}
