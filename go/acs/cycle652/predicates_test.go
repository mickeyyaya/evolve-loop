//go:build acs

// Package cycle652 encodes the acceptance criteria for
// builder-task-binding-topn-gate (inbox weight 0.97, 8th recurrence of the
// wrong-task-build defect: cycles 282, 310, 522, 575, 577, 599, 640, 645).
//
// These predicates are BEHAVIORAL: each shells `go test` against the real
// go/internal/topngate package (the system under test) rather than grepping
// source, so a predicate greens only when the gate actually blocks/approves
// the right builds — not when a magic string is present. The white-box unit
// suite the Builder must turn GREEN lives at
// go/internal/topngate/{gate_test.go,reviewer_test.go,builder_authority_test.go}
// (copied forward from the preserved cycle-645 worktree per the escalation
// note "the next attempt should START from those tests").
package cycle652

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goTest runs `go test <args...>` inside the worktree's go/ module dir and
// returns combined output plus the exit code. Exit 0 == the named tests
// passed. Requires the go toolchain (always present in this repo).
func goTest(t *testing.T, args ...string) (string, int) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	full := append([]string{"test"}, args...)
	cmd := exec.Command("go", full...)
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return string(out), exitErr.ExitCode()
	}
	t.Fatalf("go test failed to run: %v\n%s", err, out)
	return string(out), -1
}

// TestC652_001_OutOfLaneBuildBlocked binds AC-1: a build report claiming a
// slug outside triage ## top_n is surfaced as a LOUD ADVISORY (logged reason,
// approved) — the committed set is the binding authority.
// POLICY CHANGE 2026-07-22 (operator-directed, cycles 916 + 1012): the
// out-of-lane label check is now ADVISORY — both recorded fatal rejections
// discarded CORRECT work over label drift between two LLM strings, with zero
// true-fraud catches. The binding authority is triage's committed set; scope
// verification (deliverable files vs committed item scope) is the queued
// fraud guard. These predicates now bind the ADVISORY contract.
func TestC652_001_OutOfLaneBuildAdvisory(t *testing.T) {
	out, code := goTest(t, "-race", "-count=1", "-run", "TestTopNBindingGate|TestNewReviewer_Enforce", "./internal/topngate/")
	if code != 0 {
		t.Errorf("AC-1 (advisory contract): the label-drift suite must PASS; got exit=%d\n%s", code, out)
	}
}

// TestC652_002_BuilderPromptNamesTopNSoleAuthority binds AC-2:
// agents/evolve-builder.md names triage-report.md's ## top_n as the SOLE task
// authority and demotes scout-report.md to background context. Exercised by
// the real TestBuilderPromptNamesTopNAsSoleTaskAuthority assertion, which
// checks for triage-report.md + top_n + a sole/authoritative/exclusive claim
// + an explicit scout-report demotion — not a single greppable token.
func TestC652_002_BuilderPromptNamesTopNSoleAuthority(t *testing.T) {
	out, code := goTest(t, "-count=1", "-run", "TestBuilderPromptNamesTopNAsSoleTaskAuthority", "./internal/topngate/")
	if code != 0 {
		t.Errorf("AC-2: builder-prompt task-authority suite must PASS; got exit=%d\n%s", code, out)
	}
}

// TestC652_003_ReplayCycle640ShapeBlocksBeforeAudit binds AC-3: replaying the
// cycle-640 shape (triage=statefile task, build=token-resolver task) passes
// with a loud drift advisory — see the 2026-07-22 policy note on AC-1; the
// replay test itself documents why the fatal form was retired.
func TestC652_003_ReplayCycle640ShapeAdvisory(t *testing.T) {
	out, code := goTest(t, "-count=1", "-run", "TestReplayCycle640Shape", "./internal/topngate/")
	if code != 0 {
		t.Errorf("AC-3: cycle-640-replay (advisory contract) must PASS; got exit=%d\n%s", code, out)
	}
}

// TestC652_004_TouchedPackageRaceClean binds AC-4: `go test -race` on the
// touched package passes (race-clean). Running the whole package under -race
// also proves the shadow-vs-enforce config gating (no feature flag) and the
// gate's phase scoping compile and behave. apicover is a separate repo-wide
// gate the audit runs; this predicate asserts the -race half of AC-4.
func TestC652_004_TouchedPackageRaceClean(t *testing.T) {
	out, code := goTest(t, "-race", "-count=1", "./internal/topngate/...")
	if code != 0 {
		t.Errorf("AC-4: `go test -race ./internal/topngate/...` must PASS; got exit=%d\n%s", code, out)
	}
	if strings.Contains(out, "DATA RACE") {
		t.Errorf("AC-4: race detector flagged a data race:\n%s", out)
	}
}
