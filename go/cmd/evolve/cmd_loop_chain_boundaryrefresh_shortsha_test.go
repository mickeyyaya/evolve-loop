package main

import (
	"testing"
)

func TestDefaultChainBoundaryAhead_ShortCommitAtHeadIsNotAhead(t *testing.T) {
	t.Parallel()
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

func TestDefaultChainBoundaryAhead_ShortCommitBehindHeadIsAhead(t *testing.T) {
	t.Parallel()
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
