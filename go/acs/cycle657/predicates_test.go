//go:build acs

// Package cycle657 encodes the acceptance criteria for
// retro-preventive-actions-autofile-inbox (weight 0.95, operator-restored
// 2026-07-10 — "the highest-leverage defect in the system"). The feature
// closes the learning→action loop: a retro's structured "preventive_actions"
// section is auto-filed as weighted .evolve/inbox todos on the SAME
// deterministic seam the FAILED_UNEXPLAINED classifier already uses
// (cmd/evolve/cmd_loop_outcome.go:fileUnexplainedOutcomeDefect), with a dedup
// guard so recurrences don't spam. Today 238 lessons exist vs ZERO
// retro-originated inbox items; the gap let auditor-PASS-vs-EGPS-FAIL recur 3+
// times with the lesson on file each time.
//
// These predicates are BEHAVIORAL: 001-003 shell `go test -race` against the
// real system-under-test (internal/retrofile + internal/policy white-box
// suites the TDD phase froze) rather than grepping source — a predicate greens
// only when the injector actually files/deduplicates/weights the right items.
// 004 pins the retro deliverable-contract FORMAT change (config-check waiver:
// the agent doc is the phase-behavior config). 005 shells the whole-module
// `go vet` and pins the new package's apicover graduation.
//
// The white-box suites live at go/internal/retrofile/retrofile_test.go and
// go/internal/policy/retro_autofile_test.go. RED until the Builder writes
// retrofile.go, the policy accessor, and updates the retro contract.
package cycle657

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goTest runs `go test <args...>` inside the worktree's go/ module dir and
// returns combined output plus the exit code. Exit 0 == the named tests passed.
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

// TestC657_001_AutofileFromFixtureRetro — AC1: a FAIL retro emitting structured
// preventive_actions results in auto-filed inbox todos. Exercised by the real
// parse→file suite (ParsePreventiveActions + FileActions end-to-end over a
// fixture retro report), run under -race. RED until retrofile.go exists.
func TestC657_001_AutofileFromFixtureRetro(t *testing.T) {
	out, code := goTest(t, "-race", "-count=1",
		"-run", "TestParsePreventiveActions_FromRetroReport|TestParsePreventiveActions_AbsentReturnsNil|TestFileActions_EndToEndFromFixtureRetro",
		"./internal/retrofile/")
	if code != 0 {
		t.Errorf("AC1: the retro→inbox parse+file suite must PASS; got exit=%d\n%s", code, out)
	}
}

// TestC657_002_DedupFilesOnceWhileOpen — AC2: the same preventive action across
// two consecutive FAILs (and against an already-processed item) files exactly
// once while open. This is the anti-no-op / anti-spam negative axis — an
// injector that always writes would fail it. RED until dedup is implemented.
func TestC657_002_DedupFilesOnceWhileOpen(t *testing.T) {
	out, code := goTest(t, "-race", "-count=1",
		"-run", "TestFileActions_DedupSkipsExistingOpenItem|TestFileActions_DedupSkipsExistingProcessedItem|TestFileActions_DedupAcrossTwoConsecutiveFailsFilesOnce",
		"./internal/retrofile/")
	if code != 0 {
		t.Errorf("AC2: the dedup suite must PASS (files once while open, skips processed); got exit=%d\n%s", code, out)
	}
}

// TestC657_003_WeightFromPolicyWithRecurrenceHint — AC3: weight defaults from
// policy.json; recurrence-flagged items carry the higher hint. Exercised by the
// retrofile weight suite AND the internal/policy accessor test (the default is
// config-sourced, not a Go literal). RED until both land.
func TestC657_003_WeightFromPolicyWithRecurrenceHint(t *testing.T) {
	out, code := goTest(t, "-race", "-count=1",
		"-run", "TestFileActions_UsesDefaultWeightWhenNoHint|TestFileActions_UsesHintForRecurrenceFlagged",
		"./internal/retrofile/")
	if code != 0 {
		t.Errorf("AC3a: the weight-default/hint suite must PASS; got exit=%d\n%s", code, out)
	}
	out2, code2 := goTest(t, "-count=1",
		"-run", "TestRetroAutofileDefaultWeight_DefaultAndOverride", "./internal/policy/")
	if code2 != 0 {
		t.Errorf("AC3b: the policy default-weight accessor test must PASS (weight sourced from policy.json); got exit=%d\n%s", code2, out2)
	}
}

// TestC657_004_RetroContractHasStructuredPreventiveActions — AC4: the retro
// deliverable contract / agent doc gains a MACHINE-READABLE preventive_actions
// section (id/title/weight_hint/files/evidence) — the FORMAT change the
// injector parses. Config-check: the agent doc IS the phase-behavior contract,
// so a structured-section presence assertion is the legitimate inherent
// config-presence check.
//
// acs-predicate: config-check
func TestC657_004_RetroContractHasStructuredPreventiveActions(t *testing.T) {
	root := acsassert.RepoRoot(t)
	doc := filepath.Join(root, "agents", "evolve-retrospective.md")
	// The structured block must name the machine-readable schema fields, not
	// merely the existing prose "Recommended preventive actions" heading.
	if !acsassert.LineContainsAll(doc, "preventive_actions") {
		t.Errorf("AC4: %s must document a structured preventive_actions section for the autofiler", doc)
	}
	if !acsassert.FileContainsAny(doc, "weight_hint") {
		t.Errorf("AC4: %s must document the weight_hint field of the structured preventive_actions schema", doc)
	}
}

// TestC657_005_ModuleVetsAndPackageGraduated — AC5: `go vet ./...` is clean with
// the new package wired, and internal/retrofile is graduated into
// go/.apicover-enforce (the 3rd-recurrence new-package-graduation obligation
// named in the item). `go vet` compiles the whole module, so it RED-fails while
// retrofile.go is absent and greens only once the package builds and wires.
func TestC657_005_ModuleVetsAndPackageGraduated(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cmd := exec.Command("go", "vet", "./...")
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("AC5: `go vet ./...` must be clean with the new package wired; got err=%v\n%s", err, out)
	}
	// Graduation: the new leaf package must be listed in the apicover-enforce
	// single-source file (config-check — the enforce list is inherent config).
	enforce := filepath.Join(root, "go", ".apicover-enforce")
	if !acsassert.FileContains(t, enforce, "./internal/retrofile") {
		t.Errorf("AC5: internal/retrofile must be graduated into go/.apicover-enforce (new-package obligation)")
	}
}
