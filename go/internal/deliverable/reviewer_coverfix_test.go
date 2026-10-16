package deliverable

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable/gatesignal"
)

func TestVerifyForClassification_AnUnrecoverableBadVerdictAtEnforceIsReturnedUnsalvagedAndUnchanged(t *testing.T) {
	t.Parallel()
	ws, root := writeAuditReport(t, "# Audit Report\n\n## Verdict\nno machine-readable verdict anywhere\n")
	r := newTestReviewer(config.StageEnforce, filepath.Join(t.TempDir(), "breaker.json"), 3)
	in := core.ReviewInput{Phase: "audit", Workspace: ws, ProjectRoot: root, Cycle: 7, RunID: "r7"}

	res, err := r.VerifyForClassification(gatesignal.Check{Cycle: 7, RunID: "r7", Phase: "audit"}, "audit", RootsFor(in))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.OK {
		t.Fatalf("an unrecoverable verdict must stay a violation, got OK")
	}
	if !hasCode(res, CodeBadVerdict) {
		t.Fatalf("the original bad_verdict violation must survive, got %+v", res.Violations)
	}
}

func TestVerifyForClassification_ARepairThatCannotBePersistedIsRefusedAndTheOriginalReturned(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root ignores the write-permission bit this fixture relies on")
	}
	ws, pr, _ := salvageFixture(t)
	if err := os.Chmod(ws, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ws, 0o755) })
	r := newTestReviewerPhaseIO(config.StageEnforce, config.StageEnforce, filepath.Join(t.TempDir(), "b.json"), 3)
	r.logf = func(string, ...any) {}
	in := reviewInput("audit", ws, pr)

	res, err := r.VerifyForClassification(gatesignal.Check{Phase: "audit"}, "audit", RootsFor(in))

	if err != nil || res.OK {
		t.Fatalf("an unpersistable salvage must not be approved: ok=%v err=%v", res.OK, err)
	}
	if !hasCode(res, CodeBadVerdict) {
		t.Fatalf("the original bad_verdict must be returned, got %+v", res.Violations)
	}
}
