package main

// cmd_loop_chain_boundaryrefresh_shortsha_test.go — regression test (cycle
// 1320, resuming the 1314 boundary-binary-refresh task after an audit FAIL).
//
// Defect: defaultChainBoundaryAhead reported ahead=true for an up-to-date
// binary. version.Commit() is stamped by the Makefile as a 12-char SHORT
// commit (`git rev-parse --short=12 HEAD`, go/Makefile:16), but the function
// compared it directly against `git rev-parse HEAD`'s FULL 40-char SHA — a
// length mismatch that can never be equal even when current — then fell
// through to `git merge-base --is-ancestor`, which treats equal commits as
// ancestors too (non-strict), so it returned ahead=true anyway. Fixed by
// resolving runningCommit to its full SHA (`git rev-parse <short>`) before
// comparing.
//
// cmd_loop_chain_boundaryrefresh_test.go is frozen ("Do NOT modify this
// file") so this regression lives in a sibling file instead.

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
