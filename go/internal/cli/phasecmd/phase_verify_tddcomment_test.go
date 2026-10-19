package phasecmd

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const (
	tddCommentPredicate = "go/acs/cycle1971/predicates_test.go"
	tddCommentCode      = "tdd_comment_floor"
)

const (
	predicateWithNarrative    = "//go:build acs\n\npackage cycle1971\n\n// acs-predicate: config-check\n// pins the retry budget.\nfunc TestC1971_001_RetryBudget() {}\n"
	predicateWithoutNarrative = "//go:build acs\n\npackage cycle1971\n\n// acs-predicate: config-check\nfunc TestC1971_001_RetryBudget() {}\n"
)

func tddCommentFixture(t *testing.T, policyJSON, predicateBody string) (workspace, worktree string) {
	t.Helper()
	repo := gittest.Fixture(t)
	frozenPinWrite(t, repo.Dir, "go/p/a.go", "package p\n\nfunc a() {}\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	frozenPinWrite(t, repo.Dir, tddCommentPredicate, predicateBody)
	root := t.TempDir()
	if policyJSON != "" {
		frozenPinWrite(t, root, ".evolve/policy.json", policyJSON)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	return frozenPinWorkspace(t, false, tddCommentPredicate), repo.Dir
}

func TestPhaseVerifyTDD_NarrativeCommentInThePredicateFile_Exit1(t *testing.T) {
	ws, wt := tddCommentFixture(t, `{"comment_floor":{"stage":"enforce"}}`, predicateWithNarrative)

	code, _, errb := runVerify(t, "tdd", "--workspace="+ws, "--worktree="+wt)

	if code != 1 {
		t.Fatalf("exit=%d want 1 (a narrative comment in the TDD-owned predicate file); stderr=%s", code, errb)
	}
	for _, want := range []string{tddCommentCode, tddCommentPredicate + ": // pins the retry budget."} {
		if !strings.Contains(errb, want) {
			t.Errorf("stderr omits %q, so TDD cannot tell which line to delete; got %q", want, errb)
		}
	}
	if strings.Contains(errb, "acs-predicate") {
		t.Errorf("the machine-read acs-predicate marker is not a narrative comment; got %q", errb)
	}
}

func TestPhaseVerifyTDD_NarrativeCommentFailsWithoutAPolicyBlockToo(t *testing.T) {
	ws, wt := tddCommentFixture(t, "", predicateWithNarrative)

	code, _, errb := runVerify(t, "tdd", "--workspace="+ws, "--worktree="+wt)

	if code != 1 || !strings.Contains(errb, tddCommentCode) {
		t.Fatalf("exit=%d want 1 with %q: TDD corrects its own comments while the build floor still shadows; stderr=%s", code, tddCommentCode, errb)
	}
}

func TestPhaseVerifyTDD_CorrectedPredicateFile_Exit0(t *testing.T) {
	ws, wt := tddCommentFixture(t, `{"comment_floor":{"stage":"enforce"}}`, predicateWithoutNarrative)

	if code, _, errb := runVerify(t, "tdd", "--workspace="+ws, "--worktree="+wt); code != 0 {
		t.Fatalf("exit=%d want 0 once the narrative comment is deleted; stderr=%s", code, errb)
	}
}

func TestPhaseVerifyTDD_CommentFloorOffIsSilent(t *testing.T) {
	ws, wt := tddCommentFixture(t, `{"comment_floor":{"stage":"off"}}`, predicateWithNarrative)

	if code, _, errb := runVerify(t, "tdd", "--workspace="+ws, "--worktree="+wt); code != 0 {
		t.Fatalf("exit=%d want 0 with comment_floor off; stderr=%s", code, errb)
	}
}

func TestPhaseVerifyTDD_CommentInAFileTddDoesNotOwnIsNotItsFloor(t *testing.T) {
	ws, wt := tddCommentFixture(t, `{"comment_floor":{"stage":"enforce"}}`, predicateWithoutNarrative)
	frozenPinWrite(t, wt, "go/p/b.go", "package p\n\n// restates the helper.\nfunc b() {}\n")

	if code, _, errb := runVerify(t, "tdd", "--workspace="+ws, "--worktree="+wt); code != 0 {
		t.Fatalf("exit=%d want 0: the TDD floor judges only the files TDD wrote; stderr=%s", code, errb)
	}
}
