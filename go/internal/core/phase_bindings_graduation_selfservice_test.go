package core

// phase_bindings_graduation_selfservice_test.go — RED contract for the
// SELF-SERVICE half of the build-entry graduation floor (inbox
// acs-apicover-enrollment-in-builder-brief, 0.94).
//
// Evidence: batch-21 HALTED at cycle-1218 on the identical-fingerprint ceiling
// because THREE lanes' build phases aborted on the same cause — a new internal
// package absent from go/.apicover-enforce. The floor's refusal was correct and
// already NAMED the offending packages, but the reason only gestured at the
// obligation ("add its pattern line and an apicover_named_test.go"), so each
// lane had to re-derive the doctrine: which file, which line, which path.
//
// The fix is a message that is SELF-SERVING, not merely correct: it must emit
// the EXACT two edits per package, so a builder that hits it can comply
// verbatim. Naming the class is not enough — the abort text IS the remediation
// interface (the builder never reads ADR-0069).
//
// Scope boundary: this changes the message only. The detection contract
// (TestBuildGraduationCheck) is unchanged and must keep passing.

import (
	"context"
	"strings"
	"testing"
)

// TestBuildGraduationCheck_MessageEmitsTheExactTwoEdits is the acceptance: for
// each flagged package the reason must spell out both edits — the pattern line
// to append to go/.apicover-enforce, and the apicover_named_test.go path to
// create — as copy-pasteable text.
func TestBuildGraduationCheck_MessageEmitsTheExactTwoEdits(t *testing.T) {
	wt := initGitWorktree(t)
	gradWrite(t, wt, "go/.apicover-enforce", "./internal/other\n")
	gradCommitAll(t, wt)
	gradWrite(t, wt, "go/internal/brandnew/x.go", "package brandnew\n")

	got := buildGraduationCheck(context.Background(), wt)
	if got == "" {
		t.Fatal("premise broken: an ungraduated new package must still abort")
	}
	for _, want := range []string{
		// Edit 1: the file to append to, and the verbatim line to append.
		"go/.apicover-enforce",
		"./internal/brandnew",
		// Edit 2: the exact test file path to create, derived from the package.
		"go/internal/brandnew/apicover_named_test.go",
		// The obligation the test file must discharge — a bare empty file passes
		// the enforce list and then fails the repo-wide unnamed-export gate.
		"every exported symbol",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("graduation abort reason must be self-serving and contain %q;\ngot: %s", want, got)
		}
	}
}

// TestBuildGraduationCheck_MessageIsPerPackage is the scope arm: two ungraduated
// packages must each get their own two edits. A message that emits one shared
// prescription forces the builder to re-derive the second package's paths — the
// exact re-derivation this change removes.
func TestBuildGraduationCheck_MessageIsPerPackage(t *testing.T) {
	wt := initGitWorktree(t)
	gradWrite(t, wt, "go/.apicover-enforce", "./internal/other\n")
	gradCommitAll(t, wt)
	gradWrite(t, wt, "go/internal/alpha/a.go", "package alpha\n")
	gradWrite(t, wt, "go/internal/beta/b.go", "package beta\n")

	got := buildGraduationCheck(context.Background(), wt)
	for _, want := range []string{
		"./internal/alpha",
		"go/internal/alpha/apicover_named_test.go",
		"./internal/beta",
		"go/internal/beta/apicover_named_test.go",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("per-package prescription missing %q;\ngot: %s", want, got)
		}
	}
}

// TestBuildGraduationCheck_MessageDistinguishesTheTwoApicoverGates — ADR-0069
// documents TWO apicover gates: the per-cycle ACS coverage gate and the
// repo-wide enforce list. ONLY the second needs the enrollment line, and a
// message that blurs them sends the builder to edit ACS predicates it is
// role-gated out of. The reason must say which gate it is.
func TestBuildGraduationCheck_MessageDistinguishesTheTwoApicoverGates(t *testing.T) {
	wt := initGitWorktree(t)
	gradWrite(t, wt, "go/.apicover-enforce", "./internal/other\n")
	gradCommitAll(t, wt)
	gradWrite(t, wt, "go/internal/brandnew/x.go", "package brandnew\n")

	got := buildGraduationCheck(context.Background(), wt)
	if !strings.Contains(got, "repo-wide") {
		t.Errorf("reason must name the repo-wide enforce gate (not the per-cycle ACS gate — ADR-0069);\ngot: %s", got)
	}
}
