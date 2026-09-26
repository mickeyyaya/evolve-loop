package main

import (
	"testing"
)

// The 12-char short commit the Makefile actually stamps must be recognized
// as "not ahead" when it resolves to the current HEAD — the exact scenario
// that produced the audit FAIL (a freshly rebuilt binary reporting itself
// stale).
func TestDefaultChainBoundaryAhead_ShortCommitAtHeadIsNotAhead(t *testing.T) {
	dir, commitA := brfInitRepo(t)
	shortA := commitA[:12]

	ahead, err := defaultChainBoundaryAhead(dir, shortA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ahead {
		t.Fatal("a 12-char short commit resolving to HEAD must NOT report ahead=true")
	}
}

// Sanity check the same short-form seam still detects genuine lag once HEAD
// moves past the (short) commit the binary was built from.
func TestDefaultChainBoundaryAhead_ShortCommitBehindHeadIsAhead(t *testing.T) {
	dir, commitA := brfInitRepo(t)
	shortA := commitA[:12]
	brfAdvance(t, dir)

	ahead, err := defaultChainBoundaryAhead(dir, shortA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ahead {
		t.Fatal("expected ahead=true — HEAD moved past the short commit's resolved SHA")
	}
}
