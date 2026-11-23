package phasecmd

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
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

const (
	tddAuditCode      = "tdd_test_audit_floor"
	tddAuditPredicate = "go/acs/cycle1971/predicates_test.go"
	skipHidesErrorSrc = "package cycle1971\n\nimport \"testing\"\n\nfunc held() (bool, error) { return true, nil }\n\nfunc TestC1971_010_SparesAnExecutableDir(t *testing.T) {\n\tok, err := held()\n\tif err != nil {\n\t\tt.Skip(\"not available\")\n\t}\n\tif !ok {\n\t\tt.Fatal(\"not held\")\n\t}\n}\n"
	cleanAuditSrc     = "package cycle1971\n\nimport \"testing\"\n\nfunc TestC1971_010_SparesAnExecutableDir(t *testing.T) {\n\tif 1+1 != 2 {\n\t\tt.Fatal(\"arithmetic\")\n\t}\n}\n"
)

func tddAuditFixture(t *testing.T, predicateBody string) (workspace, worktree string) {
	t.Helper()
	repo := gittest.Fixture(t)
	frozenPinWrite(t, repo.Dir, "go/go.mod", "module example.com/m\n\ngo 1.23\n")
	frozenPinWrite(t, repo.Dir, "go/internal/testaudit/fakes.json", `{"rows":[]}`)
	frozenPinWrite(t, repo.Dir, "go/internal/testaudit/guards.json", `{"max_rows":1,"rows":[]}`)
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	frozenPinWrite(t, repo.Dir, tddAuditPredicate, predicateBody)
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	return frozenPinWorkspace(t, false, tddAuditPredicate), repo.Dir
}

func TestPhaseVerifyTDD_PredicateThatSkipsToHideAnError_Exit1(t *testing.T) {
	ws, wt := tddAuditFixture(t, skipHidesErrorSrc)

	code, _, errb := runVerify(t, "tdd", "--workspace="+ws, "--worktree="+wt)

	if code != 1 {
		t.Fatalf("exit=%d want 1: the build floor refuses this predicate and TDD alone can edit it; stderr=%s", code, errb)
	}
	for _, want := range []string{tddAuditCode, "skip-hides-error", "TestC1971_010_SparesAnExecutableDir"} {
		if !strings.Contains(errb, want) {
			t.Errorf("stderr omits %q: %q", want, errb)
		}
	}
}

func TestPhaseVerifyTDD_PredicateWithNoTestAuditFinding_Exit0(t *testing.T) {
	ws, wt := tddAuditFixture(t, cleanAuditSrc)

	if code, _, errb := runVerify(t, "tdd", "--workspace="+ws, "--worktree="+wt); code != 0 {
		t.Fatalf("exit=%d want 0; stderr=%s", code, errb)
	}
}

func TestPhaseVerifyTDD_ReportsTheLinesOfTheTddFloorForEveryPackageTddChanged(t *testing.T) {
	ws, wt := tddAuditFixture(t, cleanAuditSrc)
	frozenPinWrite(t, wt, "go/q/q.go", "package q\n\nfunc F() error { return nil }\n")
	frozenPinWrite(t, wt, "go/q/q_test.go", "package q\n\nimport (\n\t\"strings\"\n\t\"testing\"\n)\n\nfunc TestF(t *testing.T) {\n\tif err := F(); err != nil && !strings.Contains(err.Error(), \"x\") {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n")

	code, _, errb := runVerify(t, "tdd", "--workspace="+ws, "--worktree="+wt)

	want := core.TddTestAuditFailures(context.Background(), wt, "HEAD", 1971)
	if code != 1 || len(want) != 1 {
		t.Fatalf("exit=%d, floor lines=%q: want exit 1 and one finding in q, a package that the handoff does not list", code, want)
	}
	if !strings.Contains(errb, want[0]) {
		t.Errorf("stderr omits the line of the floor %q: %q", want[0], errb)
	}
}

func TestPhaseVerifyTDD_TheRunCycleNamesThePredicateDirectoryWhenGitCannotBeRead(t *testing.T) {
	ws, _ := tddAuditFixture(t, cleanAuditSrc)
	wt := t.TempDir()
	frozenPinWrite(t, wt, "go/go.mod", "module example.com/m\n\ngo 1.23\n")
	frozenPinWrite(t, wt, "go/internal/testaudit/fakes.json", `{"rows":[]}`)
	frozenPinWrite(t, wt, "go/internal/testaudit/guards.json", `{"max_rows":1,"rows":[]}`)
	frozenPinWrite(t, wt, tddAuditPredicate, skipHidesErrorSrc)
	frozenPinWrite(t, ws, "run.json", `{"cycle_id":1971}`)

	code, _, errb := runVerify(t, "tdd", "--workspace="+ws, "--worktree="+wt)

	if code != 1 || !strings.Contains(errb, "skip-hides-error") {
		t.Fatalf("exit=%d, stderr=%q: with no readable git state the verb audits acs/cycle1971 of the run cycle, as the TDD floor does", code, errb)
	}
}
