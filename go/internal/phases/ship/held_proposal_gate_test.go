package ship

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/heldproposal"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	heldCommit = "c0ffee00c0ffee00c0ffee00c0ffee00c0ffee00"
	heldTree   = "7ee7ee007ee7ee007ee7ee007ee7ee007ee7ee00"
	heldCycle  = 41
)

type heldFixture struct {
	opts   *Options
	rec    *argvRecorder
	logs   []string
	events []signalcenter.Event
}

func newHeldFixture(t *testing.T, stage string, changed ...string) *heldFixture {
	t.Helper()
	root := t.TempDir()
	if stage != "" {
		writeDoc(t, root, ".evolve/policy.json", `{"held_proposals":{"stage":"`+stage+`"}}`)
	}
	f := &heldFixture{opts: &Options{Class: ClassCycle, CycleID: heldCycle, ProjectRoot: root, RunID: "run-41", Stdout: io.Discard, Stderr: io.Discard}}
	center := signalcenter.New()
	center.Subscribe(func(e signalcenter.Event) { f.events = append(f.events, e) })
	f.opts.Signals = center
	f.rec = newArgvRecorder(f.opts)
	f.rec.on("diff --name-only -z --no-renames main "+heldCommit, scriptedCall{stdout: strings.Join(changed, "\x00")})
	f.rec.on("rev-parse "+heldCommit+"^{tree}", scriptedCall{stdout: heldTree + "\n"})
	return f
}

func (f *heldFixture) claim(t *testing.T, kind string) {
	t.Helper()
	writeDoc(t, f.opts.ProjectRoot, fmt.Sprintf(".evolve/inbox/processing/cycle-%d/item.json", heldCycle), `{"id":"item","kind":"`+kind+`"}`)
}

func (f *heldFixture) check() error {
	logf := func(format string, args ...any) { f.logs = append(f.logs, fmt.Sprintf(format, args...)) }
	return refuseUnapprovedHeldProposal(context.Background(), f.opts, logf, "main", heldCommit)
}

func (f *heldFixture) evolveDir() string { return filepath.Join(f.opts.ProjectRoot, ".evolve") }

func TestHeldProposal_EnforceHoldsAProtectedChangeWithNoApprovalAndRecordsIt(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "enforce", "docs/x.md", "go/internal/guards/role.go")

	err := f.check()

	se := wantShipErr(t, err, shiperr.CodeHeldForOperator, core.ShipClassPrecondition, "evolve landing approve --cycle 41 --tree "+heldTree)
	if se.Debug["tree"] != heldTree || se.Debug["reasons"] != "protected:go/internal/guards/role.go" {
		t.Errorf("debug %v: the hold names the tree and the reason", se.Debug)
	}
	rec, found, rerr := heldproposal.ReadRecord(f.evolveDir(), heldCycle)
	if rerr != nil || !found || rec.Status != heldproposal.StatusHeld || rec.Tree != heldTree || rec.Commit != heldCommit || len(rec.Requires) != 2 {
		t.Errorf("record = %+v, %v, %v, want a held-for-operator record of the tree with both requirements", rec, found, rerr)
	}
}

func TestHeldProposal_EnforceHoldsAPipelineKindItemEvenWithNoProtectedChange(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "enforce", "go/internal/inboxrank/rank.go")
	f.claim(t, "pipeline-repair")

	err := f.check()

	wantShipErr(t, err, shiperr.CodeHeldForOperator, core.ShipClassPrecondition, "kind:pipeline-repair")
}

func TestHeldProposal_AnApprovalLandsOnlyTheTreeThatItNames(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	if err := heldproposal.WriteApproval(f.evolveDir(), heldproposal.Approval{Cycle: heldCycle, Tree: "another-tree", Decision: heldproposal.DecisionApprove}); err != nil {
		t.Fatal(err)
	}
	wantShipErr(t, f.check(), shiperr.CodeHeldForOperator, core.ShipClassPrecondition, "")

	if err := heldproposal.WriteApproval(f.evolveDir(), heldproposal.Approval{Cycle: heldCycle, Tree: heldTree, Decision: heldproposal.DecisionApprove}); err != nil {
		t.Fatal(err)
	}
	if err := f.check(); err != nil {
		t.Fatalf("the approval of this tree must let the landing go on: %v", err)
	}
	if len(f.logs) == 0 || !strings.Contains(f.logs[len(f.logs)-1], "approved tree "+heldTree) {
		t.Errorf("logs = %v, want the approval named", f.logs)
	}
}

func TestHeldProposal_ARejectionStopsTheShip(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	wantShipErr(t, f.check(), shiperr.CodeHeldForOperator, core.ShipClassPrecondition, "")
	if err := heldproposal.WriteApproval(f.evolveDir(), heldproposal.Approval{Cycle: heldCycle, Tree: heldTree, Decision: heldproposal.DecisionReject}); err != nil {
		t.Fatal(err)
	}

	wantShipErr(t, f.check(), shiperr.CodeHeldProposalRejected, core.ShipClassPrecondition, "rejected tree "+heldTree)
}

func TestHeldProposal_AnApprovalWithNoHoldRecordOfItsTreeHoldsAgain(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	if err := heldproposal.WriteApproval(f.evolveDir(), heldproposal.Approval{Cycle: heldCycle, Tree: heldTree, Decision: heldproposal.DecisionApprove}); err != nil {
		t.Fatal(err)
	}

	wantShipErr(t, f.check(), shiperr.CodeHeldForOperator, core.ShipClassPrecondition, "")
}

func TestHeldProposal_TheVerbsTerminalStatusStillLandsTheApprovedTree(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	for _, write := range []error{
		heldproposal.WriteRecord(f.evolveDir(), heldproposal.Record{Cycle: heldCycle, Status: heldproposal.StatusApproved, Tree: heldTree}),
		heldproposal.WriteApproval(f.evolveDir(), heldproposal.Approval{Cycle: heldCycle, Tree: heldTree, Decision: heldproposal.DecisionApprove}),
	} {
		if write != nil {
			t.Fatal(write)
		}
	}
	if err := f.check(); err != nil {
		t.Fatalf("an approved record and approval of the tree must land: %v", err)
	}
	if err := heldproposal.WriteRecord(f.evolveDir(), heldproposal.Record{Cycle: heldCycle, Status: heldproposal.StatusRejected, Tree: heldTree}); err != nil {
		t.Fatal(err)
	}
	wantShipErr(t, f.check(), shiperr.CodeHeldForOperator, core.ShipClassPrecondition, "")
}

func TestHeldProposal_AnUnknownCycleRefusesAtEnforceOnly(t *testing.T) {
	t.Parallel()
	enforce := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	enforce.opts.CycleID = 0
	wantShipErr(t, enforce.check(), core.CodeStateIO, core.ShipClassPrecondition, "needs the shipping cycle")
	shadow := newHeldFixture(t, "shadow", "go/internal/guards/role.go")
	shadow.opts.CycleID = 0
	if err := shadow.check(); err != nil {
		t.Errorf("shadow with no cycle: %v, want no hold", err)
	}
}

func TestHeldProposal_ShadowRecordsWhatItWouldHoldAndLands(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "", "go/internal/guards/role.go")

	if err := f.check(); err != nil {
		t.Fatalf("shadow must land: %v", err)
	}

	rec, found, err := heldproposal.ReadRecord(f.evolveDir(), heldCycle)
	if err != nil || !found || rec.Status != heldproposal.StatusWouldHold {
		t.Errorf("record = %+v, %v, %v, want a would-hold record", rec, found, err)
	}
	if len(f.events) != 1 || f.events[0].Code != CodeHoldShadow || f.events[0].Fields["tree"] != heldTree {
		t.Errorf("events = %+v, want one SHIP_HOLD_SHADOW that names the tree", f.events)
	}
}

func TestHeldProposal_ShadowKeepsLandingWhenTheRecordCannotBeWritten(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "shadow", "go/internal/guards/role.go")
	writeDoc(t, f.opts.ProjectRoot, ".evolve/landing/held", "a file where the directory belongs")

	if err := f.check(); err != nil {
		t.Fatalf("a shadow record failure must not stop the landing: %v", err)
	}
	if len(f.logs) != 1 || !strings.Contains(f.logs[0], "heldproposal") {
		t.Errorf("logs = %v, want the write failure logged", f.logs)
	}
	if len(f.events) != 1 || !strings.Contains(f.events[0].Fields["record_error"], "heldproposal") {
		t.Errorf("events = %+v, want the shadow event to carry the record error", f.events)
	}
}

func TestHeldProposal_NoCheckRunsForOffAManualShipOrAnOrdinaryChange(t *testing.T) {
	t.Parallel()
	off := newHeldFixture(t, "off", "go/internal/guards/role.go")
	manual := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	manual.opts.Class = ClassManual
	ordinary := newHeldFixture(t, "enforce", "go/internal/inboxrank/rank.go")
	ordinary.claim(t, "bug")
	for name, f := range map[string]*heldFixture{"off": off, "manual": manual, "ordinary": ordinary} {
		if err := f.check(); err != nil {
			t.Errorf("%s: %v, want no hold", name, err)
		}
		if _, found, _ := heldproposal.ReadRecord(f.evolveDir(), heldCycle); found {
			t.Errorf("%s: wrote a hold record", name)
		}
	}
}

func TestHeldProposal_ABadPolicyHoldsAsAtEnforce(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "sometimes", "go/internal/guards/role.go")

	wantShipErr(t, f.check(), shiperr.CodeHeldForOperator, core.ShipClassPrecondition, "")
	if len(f.logs) == 0 || !strings.Contains(f.logs[0], "holds as at enforce") {
		t.Errorf("logs = %v, want the policy error logged", f.logs)
	}
}

func TestHeldProposal_EachReadFailureFailsClosed(t *testing.T) {
	t.Parallel()
	state := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	state.opts.CycleID = 0
	writeDoc(t, state.opts.ProjectRoot, ".evolve/cycle-state.json", "{not json")
	wantShipErr(t, state.check(), core.CodeStateIO, core.ShipClassTransient, "read the shipping cycle")

	inbox := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	writeDoc(t, inbox.opts.ProjectRoot, fmt.Sprintf(".evolve/inbox/processing/cycle-%d", heldCycle), "a file where the directory belongs")
	wantShipErr(t, inbox.check(), core.CodeStateIO, core.ShipClassTransient, "claimed inbox items")

	diff := newHeldFixture(t, "enforce")
	diff.rec.set("diff --name-only -z --no-renames main "+heldCommit, scriptedCall{exit: 128})
	if err := diff.check(); err == nil {
		t.Error("a failed diff must fail the check")
	}

	tree := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	tree.rec.set("rev-parse "+heldCommit+"^{tree}", scriptedCall{exit: 128})
	if err := tree.check(); err == nil {
		t.Error("a failed tree read must fail the check")
	}

	prior := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	if err := heldproposal.WriteApproval(prior.evolveDir(), heldproposal.Approval{Cycle: heldCycle, Tree: heldTree, Decision: heldproposal.DecisionApprove}); err != nil {
		t.Fatal(err)
	}
	writeDoc(t, prior.opts.ProjectRoot, fmt.Sprintf(".evolve/landing/held/cycle-%d.json", heldCycle), "{not json")
	wantShipErr(t, prior.check(), core.CodeStateIO, core.ShipClassTransient, "read the hold record")

	approval := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	writeDoc(t, approval.opts.ProjectRoot, fmt.Sprintf(".evolve/landing/approvals/cycle-%d.json", heldCycle), "{not json")
	wantShipErr(t, approval.check(), core.CodeStateIO, core.ShipClassTransient, "operator approval")

	record := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	writeDoc(t, record.opts.ProjectRoot, ".evolve/landing/held", "a file where the directory belongs")
	wantShipErr(t, record.check(), core.CodeStateIO, core.ShipClassTransient, "write the hold record")
}

func TestWorktreeShipLand_AHeldShipNeverMovesMain(t *testing.T) {
	t.Parallel()
	root := docRepo(t)
	writeDoc(t, root, ".evolve/policy.json", `{"held_proposals":{"stage":"enforce"}}`)
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "policy")
	wt := filepath.Join(t.TempDir(), "lane")
	gitIn(t, root, "worktree", "add", "-q", "-b", "lane", wt)
	writeDoc(t, wt, "go/internal/guards/role.go", "package guards\n")
	gitIn(t, wt, "add", "-A")
	pending := gitOut(t, wt, "commit-tree", gitOut(t, wt, "write-tree"), "-p", "HEAD", "-m", "evolve-cycle 41")
	main := gitOut(t, root, "rev-parse", "main")
	opts := &Options{Class: ClassCycle, CycleID: heldCycle, ProjectRoot: root, Stdout: io.Discard, Stderr: io.Discard}
	s := newWorktreeShip(context.Background(), opts, &RunResult{}, "main", wt)
	s.cycleBranch = "lane"

	err := s.land(pending)

	wantShipErr(t, err, shiperr.CodeHeldForOperator, core.ShipClassPrecondition, "")
	if got := gitOut(t, root, "rev-parse", "main"); got != main {
		t.Errorf("main moved to %s: a held landing leaves main alone", got)
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "landing", "held", "cycle-41.json")); err != nil {
		t.Errorf("hold record: %v", err)
	}
	var se *core.ShipError
	if !errors.As(err, &se) {
		t.Fatal("want a ship error")
	}
}
