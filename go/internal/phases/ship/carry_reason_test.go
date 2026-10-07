package ship

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestVerifyExecutionTree_NamesWhyACarryWasNotReProven(t *testing.T) {
	for name, tc := range map[string]struct {
		arrange func(t *testing.T, l carriedLane)
		want    string
	}{
		"no carry names the audit": {
			func(*testing.T, carriedLane) {},
			"carry not re-proven: no identical-rebase carry names audit audit-ref",
		},
		"the ledger does not verify": {
			func(t *testing.T, l carriedLane) {
				writeCarry(t, l, "audit-ref", l.tree0)
				appendACopyOutsideTheChain(t, l)
			},
			"carry not re-proven: the ledger does not verify: ",
		},
		"the record names another tree": {
			func(t *testing.T, l carriedLane) {
				writeCarryStating(t, l, "audit-ref", l.tree0, strings.Repeat("0", 40))
			},
			"carry not re-proven: the carry of cycle 1715 names the tree " + strings.Repeat("0", 40),
		},
	} {
		t.Run(name, func(t *testing.T) {
			l := rebasedLane(t)
			tc.arrange(t, l)

			err := verifyExecutionTree(context.Background(), boundTo(t, l, l.tree0, "audit-ref"), &RunResult{}, l.repo)

			wantShipErr(t, err, core.CodeAuditBindingTreeMismatch, core.ShipClassPrecondition, tc.want)
		})
	}
}

func TestCarrySatisfied_SignalsACarryRecordItCouldNotReProve(t *testing.T) {
	l := rebasedLane(t)
	writeCarryStating(t, l, "audit-ref", l.tree0, strings.Repeat("0", 40))
	opts := boundTo(t, l, l.tree0, "audit-ref")
	center, got := recordingCenter()
	opts.Signals = center

	ok, _ := carrySatisfied(context.Background(), opts, l.repo, l.tree1)

	if ok {
		t.Fatal("a record naming another tree never carries")
	}
	if len(*got) != 1 {
		t.Fatalf("events = %+v, want exactly the one declined carry", *got)
	}
	e := (*got)[0]
	if e.Code != codeCarryNotReProven || e.Kind != signalcenter.KindShipWarning || e.Severity != signalcenter.SeverityWarn ||
		e.Module != signalcenter.ModuleShip || e.Fields["carry_cycle"] != "1715" || !strings.Contains(e.Reason, "names the tree "+strings.Repeat("0", 40)) {
		t.Fatalf("event = %+v, want a ship warning naming the carry and why it was not re-proven", e)
	}
}

func TestCarrySatisfied_SignalsNothingWhenNoCarryNamesTheAudit(t *testing.T) {
	l := rebasedLane(t)
	opts := boundTo(t, l, l.tree0, "audit-ref")
	center, got := recordingCenter()
	opts.Signals = center

	if ok, _ := carrySatisfied(context.Background(), opts, l.repo, l.tree1); ok {
		t.Fatal("no record, no carry")
	}
	if len(*got) != 0 {
		t.Fatalf("events = %+v, want none: a drift no carry claims is the tree-binding refusal's to report", *got)
	}
}

func TestCarrySatisfied_SignalsARecordThatNamesAnotherAuditedTree(t *testing.T) {
	l := rebasedLane(t)
	writeCarryStating(t, l, "audit-ref", strings.Repeat("1", 40), l.tree1)
	opts := boundTo(t, l, l.tree0, "audit-ref")
	center, got := recordingCenter()
	opts.Signals = center

	ok, _ := carrySatisfied(context.Background(), opts, l.repo, l.tree1)

	if ok {
		t.Fatal("a record naming another audited tree never carries")
	}
	if len(*got) != 1 || (*got)[0].Code != codeCarryNotReProven || !strings.Contains((*got)[0].Reason, "names the audited tree "+strings.Repeat("1", 40)) {
		t.Fatalf("events = %+v, want one SHIP_CARRY_NOT_REPROVEN naming the audited tree the record claimed", *got)
	}
}
