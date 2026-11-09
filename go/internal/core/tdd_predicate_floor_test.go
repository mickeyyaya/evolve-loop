package core

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const tddFloorExistsHelper = "package %s\n\nimport \"os\"\n\nfunc %s(p string) bool {\n\t_, err := os.Stat(p)\n\treturn err == nil\n}\n"

const tddFloorOtherHelper = "package %s\n\nimport \"os\"\n\nfunc %s(p string) []byte {\n\tdata, _ := os.ReadFile(p)\n\treturn data\n}\n"

func tddFloorWorktree(t *testing.T, predicateHelper string) string {
	t.Helper()
	wt := t.TempDir()
	writeFileAt(t, filepath.Join(wt, "go", ".test-dup-baseline"), "count=0\ndate=2026-11-09\n")
	writeFileAt(t, filepath.Join(wt, "go", "internal", "a", "a_test.go"), fmt.Sprintf(tddFloorExistsHelper, "a", "exists"))
	writeFileAt(t, filepath.Join(wt, "go", "acs", "cycle9", "predicates_test.go"), fmt.Sprintf(predicateHelper, "cycle9", "onDisk"))
	return wt
}

func tddFloorInput(wt string, stderr *bytes.Buffer) ReviewInput {
	return ReviewInput{Phase: string(PhaseTDD), Cycle: 9, Worktree: wt, Stderr: stderr}
}

func TestTddPredicateFloor_TheOrchestratorSendsAPredicateThatCopiesAHelperBackToTdd(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	wt := tddFloorWorktree(t, tddFloorExistsHelper)

	res := NewOrchestrator(nil, nil, nil).performEffectsAndReview(context.Background(), tddFloorInput(wt, &stderr))

	if res.Approve || !res.Retry {
		t.Fatalf("Review = %+v: TDD is the only phase that can change go/acs/cycle9/, so the host must send the copy back to it", res)
	}
	want := "acs/cycle9/predicates_test.go:onDisk = internal/a/a_test.go:exists"
	if len(res.Failures) != 1 || res.Failures[0] != want {
		t.Errorf("Failures = %q, want [%q]", res.Failures, want)
	}
	for _, part := range []string{tddFloorReasonHead, "go/acs/cycle9/", "pkg/acsassert", want} {
		if !strings.Contains(res.Reason, part) {
			t.Errorf("Reason omits %q, so TDD cannot act on it: %q", part, res.Reason)
		}
	}
}

func TestTddPredicateFloor_AComposedReviewerChainStillRunsTheFloor(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	wt := tddFloorWorktree(t, tddFloorExistsHelper)
	o := NewOrchestrator(nil, nil, nil, WithReviewer(ChainReviewers(noopReviewer{})))

	res := o.performEffectsAndReview(context.Background(), tddFloorInput(wt, &stderr))

	if res.Approve || !res.Retry || len(res.Failures) != 1 {
		t.Fatalf("Review = %+v: the composition root mounts its own chain through WithReviewer, and the TDD floor must stay in front of it", res)
	}
}

func TestTddPredicateFloor_ApprovesAPredicateWithItsOwnHelperBody(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	wt := tddFloorWorktree(t, tddFloorOtherHelper)

	if res := (tddPredicateFloor{}).Review(context.Background(), tddFloorInput(wt, &stderr)); !res.Approve {
		t.Fatalf("Review = %+v, want approve: the predicate copies no helper body", res)
	}
}

func TestTddPredicateFloor_JudgesOnlyTheTddHandoffOfACycleWithAWorktree(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	wt := tddFloorWorktree(t, tddFloorExistsHelper)
	cases := map[string]ReviewInput{
		"build phase": {Phase: string(PhaseBuild), Cycle: 9, Worktree: wt, Stderr: &stderr},
		"no worktree": {Phase: string(PhaseTDD), Cycle: 9, Stderr: &stderr},
		"no cycle":    {Phase: string(PhaseTDD), Worktree: wt, Stderr: &stderr},
	}
	for name, in := range cases {
		if res := (tddPredicateFloor{}).Review(context.Background(), in); !res.Approve {
			t.Errorf("%s: Review = %+v, want approve", name, res)
		}
	}
}

func TestTddPredicateFloor_ATreeWithoutTheRatchetIsNotJudged(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	wt := tddFloorWorktree(t, tddFloorExistsHelper)
	if err := os.Remove(filepath.Join(wt, "go", ".test-dup-baseline")); err != nil {
		t.Fatal(err)
	}

	res := (tddPredicateFloor{}).Review(context.Background(), tddFloorInput(wt, &stderr))

	if !res.Approve || stderr.Len() != 0 {
		t.Fatalf("Review = %+v, stderr = %q: a project without go/.test-dup-baseline has no ratchet to apply", res, stderr.String())
	}
}

func TestTddPredicateFloor_AnUnreadableRatchetWarnsAndLeavesTheCountToShip(t *testing.T) {
	t.Parallel()
	cases := map[string]func(t *testing.T, wt string){
		"malformed baseline": func(t *testing.T, wt string) {
			writeFileAt(t, filepath.Join(wt, "go", ".test-dup-baseline"), "count=many\n")
		},
		"baseline is a directory": func(t *testing.T, wt string) {
			path := filepath.Join(wt, "go", ".test-dup-baseline")
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
		},
		"test file does not parse": func(t *testing.T, wt string) {
			writeFileAt(t, filepath.Join(wt, "go", "internal", "b", "b_test.go"), "package b\n\nfunc broken( {\n")
		},
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var stderr bytes.Buffer
			wt := tddFloorWorktree(t, tddFloorExistsHelper)
			breakIt(t, wt)

			res := (tddPredicateFloor{}).Review(context.Background(), tddFloorInput(wt, &stderr))

			if !res.Approve || !strings.Contains(stderr.String(), "[tdd-floor] WARN") {
				t.Fatalf("Review = %+v, stderr = %q: a check that cannot read its inputs warns loudly and leaves the count to the ship ratchet", res, stderr.String())
			}
		})
	}
}
