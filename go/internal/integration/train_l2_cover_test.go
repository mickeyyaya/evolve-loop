package integration

import (
	"errors"
	"strings"
	"testing"
)

func TestTrainL2_AnEveryMemberThatFailsItsPackageTestEmptiesTheBatchAndOpensNoBatchPR(t *testing.T) {
	t.Parallel()
	r, s := l2Rig(t)
	s.testFail = func(string, string, int) error { return errors.New("--- FAIL: TestEverything") }
	out, err := r.landAlphaAndBeta(t)
	if err != nil {
		t.Fatalf("Land: %v", err)
	}
	if len(out.Landed) != 0 || len(out.Ejected) != 2 {
		t.Fatalf("Outcome = landed %v ejected %v, want both ejected", out.Landed, out.Ejected)
	}
	if len(r.ci.opened) != 0 {
		t.Errorf("opened %d batch PRs for an empty batch, want 0", len(r.ci.opened))
	}
}

func TestTrainL2_ABatchBranchThatMovedDuringTheTestIsAnErrorAndOpensNoBatchPR(t *testing.T) {
	t.Parallel()
	r, s := l2Rig(t)
	s.testFail = func(string, string, int) error {
		for _, ref := range strings.Fields(r.f.g.Git("for-each-ref", "--format=%(refname)", "refs/heads/")) {
			if ref != "refs/heads/main" {
				r.f.g.Git("update-ref", "-d", ref)
			}
		}
		return nil
	}
	_, err := r.landAlphaAndBeta(t)
	if err == nil || !strings.Contains(err.Error(), "integration: move") {
		t.Fatalf("Land = %v, want the move failure", err)
	}
	if len(r.ci.opened) != 0 {
		t.Errorf("opened %d batch PRs after a failed move, want 0", len(r.ci.opened))
	}
}
