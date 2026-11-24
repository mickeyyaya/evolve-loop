package ship

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/heldaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/heldproposal"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

func TestHeldAuditPort_ReceivesTheEvolveDirTheRunWorkspaceAndTheHeldRecord(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	wantShipErr(t, f.check(), shiperr.CodeHeldForOperator, core.ShipClassPrecondition, "")
	var gotDir, gotWorkspace string
	var gotRecord heldproposal.Record
	var port HeldAuditPort = func(_ context.Context, evolveDir, workspace string, rec heldproposal.Record) error {
		gotDir, gotWorkspace, gotRecord = evolveDir, workspace, rec
		return nil
	}
	f.opts.HeldAudit = port
	res := &RunResult{}

	dispatchHeldAudit(context.Background(), f.opts, res, heldShipErr())

	if gotDir != f.evolveDir() || !strings.HasSuffix(gotWorkspace, "cycle-41") || gotRecord.Status != heldproposal.StatusHeld || gotRecord.Cycle != heldCycle {
		t.Errorf("port got dir %q workspace %q record %+v, want the evolve dir, the run workspace and the held record", gotDir, gotWorkspace, gotRecord)
	}
	if len(res.Logs) != 0 {
		t.Errorf("logs = %v, want none when the port succeeds", res.Logs)
	}
}

func TestHeldAuditPlanPort_WorkspaceAndRecordReachThePortAndItsPlanIsRecordedInShadow(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "", "go/internal/guards/role.go")
	var gotWorkspace string
	var gotCycle int
	var plan HeldAuditPlanPort = func(workspace string, rec heldproposal.Record) heldaudit.Plan {
		gotWorkspace, gotCycle = workspace, rec.Cycle
		return heldaudit.Plan{CLI: "agy-tmux", Family: "gemini", Tier: "deep"}
	}
	f.opts.HeldAuditPlan = plan

	fields, err := shadowEvent(t, f)

	if err != nil || gotCycle != heldCycle || !strings.HasSuffix(gotWorkspace, "cycle-41") || fields["audit_plan"] != "gemini/agy-tmux/deep" {
		t.Errorf("err %v cycle %d workspace %q fields %v, want the plan of the port in the shadow event", err, gotCycle, gotWorkspace, fields)
	}
}
