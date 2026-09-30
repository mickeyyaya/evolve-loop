//go:build acs

package cycle661

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func repoFile(t *testing.T, rel string) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), rel)
}

func runNamedTest(t *testing.T, pkg, name string) (passed bool, out string) {
	t.Helper()
	dir := goDir(t)
	stdout, stderr, _, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-race", "-count=1", "-v",
		"-run", "^"+name+"$", pkg,
	)
	out = stdout + "\n" + stderr
	return strings.Contains(out, "--- PASS: "+name), out
}

func TestC661_001_ThreeSamePatternRetrosCountThreeAndBumpOnce(t *testing.T) {
	passed, out := runNamedTest(t, "./internal/recurrence/...",
		"TestLedger_ThreeSamePatternCountsThreeAndBumpsOnce")
	if !passed {
		t.Fatalf("RED: TestLedger_ThreeSamePatternCountsThreeAndBumpsOnce did not run+PASS.\n"+
			"Builder must create leaf package internal/recurrence whose ledger upsert makes three\n"+
			"same-pattern retro closeouts yield count=3 and ONE idempotent weight bump on the linked\n"+
			"open inbox item.\nOutput:\n%s", out)
	}
}

func TestC661_002_CountGE2NoOpenItemAutofilesExactlyOnce(t *testing.T) {
	passed, out := runNamedTest(t, "./internal/recurrence/...",
		"TestLedger_CountGE2NoOpenItemAutofilesOnce")
	if !passed {
		t.Fatalf("RED: TestLedger_CountGE2NoOpenItemAutofilesOnce did not run+PASS.\n"+
			"Builder must make a count>=2 pattern with no open inbox item hand off to the autofile\n"+
			"seam exactly once (dedup guard) — never zero, never twice.\nOutput:\n%s", out)
	}
}

func TestC661_003_RetroDecisionConsultsLedgerForcesAdapt(t *testing.T) {
	dir := goDir(t)
	if _, errOut, code, err := acsassert.SubprocessOutput(
		"go", "build", "-C", dir, "./internal/core/...",
	); code != 0 || err != nil {
		t.Fatalf("RED: internal/core does not build with the recurrence-ledger consult (exit=%d): %v\n%s",
			code, err, errOut)
	}

	if !acsassert.FileContains(t, repoFile(t, "go/internal/core/decision_branch.go"), "recurrence.") {
		t.Errorf("RED: decision_branch.go does not reference recurrence.* — RetroDecision must consult " +
			"the ledger so count>=2 cannot emit bare 'proceed'")
	}

	passed, out := runNamedTest(t, "./internal/core/...",
		"TestDecideAfterRetro_NthOccurrenceForcesAdapt")
	if !passed {
		t.Fatalf("RED: TestDecideAfterRetro_NthOccurrenceForcesAdapt did not run+PASS.\n"+
			"Builder must gate the RetroDecision 'proceed' branch on a ledger lookup: an\n"+
			"Nth-occurrence (count>=2) pattern forces 'adapt: escalated <item> to <weight>'.\nOutput:\n%s", out)
	}
}

func TestC661_004_LessonsRecurrenceCLIReportSortedByCount(t *testing.T) {
	passed, out := runNamedTest(t, "./cmd/evolve/...",
		"TestLessonsRecurrence_SortedByCountWithFixStatus")
	if !passed {
		t.Fatalf("RED: TestLessonsRecurrence_SortedByCountWithFixStatus did not run+PASS.\n"+
			"Builder must add `evolve lessons recurrence` printing patterns sorted by descending\n"+
			"count with each pattern's fix_item status.\nOutput:\n%s", out)
	}
}

// acs-predicate: config-check
func TestC661_005_RecurrenceGraduatedIntoApicoverEnforce(t *testing.T) {
	enforce := repoFile(t, "go/.apicover-enforce")
	if !acsassert.FileContainsAny(enforce,
		"./internal/recurrence",
		"internal/recurrence",
	) {
		t.Errorf("RED: go/.apicover-enforce does not list internal/recurrence — the new leaf package " +
			"must be graduated into the enforced set in the same commit (new-package obligation, " +
			"4th-recurrence class)")
	}
	if !acsassert.FileContains(t, enforce, "./internal/adapters/statemap") {
		t.Errorf("apicover-enforce sanity: expected the file to still list ./internal/adapters/statemap")
	}
}
