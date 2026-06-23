//go:build acs

package envtaint

import "testing"

// These tests pin the two capabilities the flag-metric harness MUST have that
// the existing go/ast literal scanner (flagreaders) lacks, and which let
// cycle-20 game the metric:
//
//  1. See through a split-const dodge: `"EVOLVE_" + "WORKTREE_BASE"` is an
//     *ast.BinaryExpr, invisible to a strconv.Unquote literal scan. The
//     type-checker constant-folds it; the harness must report the folded value.
//  2. Distinguish a compile-time-constant os.Getenv argument (a real, countable
//     operator dial) from a non-constant one (dynamic key — not a fixed dial).

func TestLoad_FoldsConcatenatedStringConstant(t *testing.T) {
	const src = `package p

const envWorktreeBase = "EVOLVE_" + "WORKTREE_BASE"
`
	h, err := Load(src)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got, ok := h.ConstStringValue("envWorktreeBase")
	if !ok {
		t.Fatal("envWorktreeBase: not found or not a string constant")
	}
	if got != "EVOLVE_WORKTREE_BASE" {
		t.Errorf("folded value = %q, want %q", got, "EVOLVE_WORKTREE_BASE")
	}
}

func TestGetenvCalls_FoldsConstantArg(t *testing.T) {
	const src = `package p

import "os"

const envX = "EVOLVE_" + "X"

var _ = os.Getenv(envX)
`
	h, err := Load(src)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	calls := h.GetenvCalls()
	if len(calls) != 1 {
		t.Fatalf("GetenvCalls returned %d calls, want 1", len(calls))
	}
	if !calls[0].Constant {
		t.Errorf("call.Constant = false, want true (arg folds to a string constant)")
	}
	if calls[0].Key != "EVOLVE_X" {
		t.Errorf("call.Key = %q, want %q", calls[0].Key, "EVOLVE_X")
	}
}

func TestGetenvCalls_DetectsNonConstantArg(t *testing.T) {
	const src = `package p

import "os"

func read(k string) string { return os.Getenv("EVOLVE_" + k) }
`
	h, err := Load(src)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	calls := h.GetenvCalls()
	if len(calls) != 1 {
		t.Fatalf("GetenvCalls returned %d calls, want 1", len(calls))
	}
	if calls[0].Constant {
		t.Error("call.Constant = true, want false (arg has a non-constant operand)")
	}
}
