package core

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
)

func (h *carryHarness) recover(t *testing.T, storage *fakeStorage, rows ...LedgerEntry) (Phase, bool, CycleState) {
	t.Helper()
	o := NewOrchestrator(storage, &fakeLedger{entries: rows}, buildRunners(nil),
		WithSignalCenter(h.signals),
		WithStderr(h.stderr),
		WithCompositionSnapshot(func(context.Context, string, string) (CompositionAuditSnapshot, error) {
			return CompositionAuditSnapshot{LaneAuditRef: "audit-ref"}, nil
		}),
		WithCompositionGateRunner(func(_ context.Context, worktree string) map[string]ciparity.GateOutcome {
			h.gatesOn = worktree
			if h.duringGates != nil {
				h.duringGates()
			}
			return h.gates
		}),
		WithCompositionVerdictWriter(func(path string, in CompositionVerdictInput) error {
			h.writtenTo = path
			h.written = append(h.written, in)
			if h.persist != nil {
				return h.persist(path, in)
			}
			return h.writeErr
		}),
	)
	cs := h.fx.cycleState()
	next, recovering := o.recoverFromShipError(context.Background(), h.fx.root, unwindCycle, &cs,
		NewShipError(CodeGitFleetRebaseNeeded, ShipClassTransient, StageAtomicShip, "peer moved main"), 0, 2)
	return next, recovering, cs
}

func TestCycle1637Replay_ShipsOnFirstRecovery(t *testing.T) {
	t.Parallel()

	fx := newShippedLane(t, "lane.txt", addLaneFile, addPeerFile)
	fx.newBase = fx.landOnMain("peer2.txt", "second peer landing\n")
	h := &carryHarness{fx: fx, gates: allComposedGatesPass()}
	storage := &fakeStorage{}

	next, recovering, cs := h.recover(t, storage, h.auditRow())

	if !recovering || next == PhaseBuild || next == PhaseAudit || next != PhaseShip {
		t.Fatalf("recovery=(%s,%v), want PhaseShip on first recovery without a Build or Audit chase", next, recovering)
	}
	fx.requirePendingLaneChangeOn(fx.newBase)
	if cs.WorktreeBaseSHA != fx.newBase || storage.cycleState.WorktreeBaseSHA != fx.newBase {
		t.Fatalf("rebound base not persisted: memory=%q storage=%q want=%q", cs.WorktreeBaseSHA, storage.cycleState.WorktreeBaseSHA, fx.newBase)
	}
	view, ok, err := explanationdocs.Verify(context.Background(), fx.bindingAt(fx.newBase))
	if err != nil || !ok {
		t.Fatalf("the rebound explanation does not verify on the new base: ok=%v err=%v", ok, err)
	}
	if view.AuthoredBaseSHA != fx.base {
		t.Fatalf("authored base = %q, want %q", view.AuthoredBaseSHA, fx.base)
	}
	tree1 := fx.git("write-tree")
	if h.gatesOn != fx.worktree || len(h.written) != 1 || h.writtenTo != filepath.Join(fx.root, ".evolve", "ledger.jsonl") {
		t.Fatalf("gates ran on %q, written=%d to %q", h.gatesOn, len(h.written), h.writtenTo)
	}
	rec := h.written[0]
	if rec.Method != identicalRebaseMethod || rec.LaneAuditRef != "audit-ref" || rec.AuditedTreeSHA != fx.auditedTree || rec.TreeStateSHA != tree1 ||
		rec.AuditedBase != fx.base || rec.GitHead != fx.newBase || rec.Cycle != unwindCycle || !bytes.Equal(rec.AuditedDiff, rec.ComposedDiff) || rec.PatchID == "" {
		t.Errorf("carry record mismatch: %+v", rec)
	}
}

func TestCycle1637Replay_ByteDivergentRebaseDeclinesCarry(t *testing.T) {
	t.Parallel()

	fx := newShippedLane(t, "shared.txt", editSharedLine(1, "lane edit"), editSharedLine(12, "peer edit"))
	h := &carryHarness{fx: fx, gates: allComposedGatesPass()}
	storage := &fakeStorage{}

	next, recovering, cs := h.recover(t, storage, h.auditRow())

	if !recovering || next != PhaseBuild {
		t.Fatalf("recovery=(%s,%v), want PhaseBuild when a peer edit makes the rebased change diverge", next, recovering)
	}
	fx.requirePendingLaneChangeOn(fx.newBase)
	if cs.WorktreeBaseSHA != fx.newBase || storage.cycleState.WorktreeBaseSHA != fx.newBase {
		t.Fatalf("rebased base not persisted: memory=%q storage=%q want=%q", cs.WorktreeBaseSHA, storage.cycleState.WorktreeBaseSHA, fx.newBase)
	}
	if len(h.written) != 0 {
		t.Fatalf("divergent rebase wrote carry records: %+v", h.written)
	}
}
