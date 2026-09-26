package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
)

func TestRecoverFromShipError_IdenticalRebaseSkipsBuild(t *testing.T) {
	fx := newShippedLane(t, "lane.txt", addLaneFile, addPeerFile)
	doc, err := os.ReadFile(filepath.Join(fx.worktree, filepath.FromSlash(fx.doc)))
	if err != nil {
		t.Fatal(err)
	}
	storage := &fakeStorage{}

	next, recovering, cs := fx.recover(storage, fx.auditRow())

	if !recovering || next != PhaseAudit {
		t.Fatalf("recovery=(%s,%v), want Audit without a Build", next, recovering)
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
	if after, _ := os.ReadFile(filepath.Join(fx.worktree, filepath.FromSlash(fx.doc))); string(after) != string(doc) {
		t.Fatal("the Builder's document changed")
	}
}

func TestRecoverFromShipError_NonIdenticalRebaseStillReturnsToBuild(t *testing.T) {
	fx := newShippedLane(t, "shared.txt", editSharedLine(1, "lane edit"), editSharedLine(12, "peer edit"))
	storage := &fakeStorage{}

	next, recovering, cs := fx.recover(storage, fx.auditRow())

	if !recovering || next != PhaseBuild {
		t.Fatalf("recovery=(%s,%v), want Build: the peer changed a lane file", next, recovering)
	}
	fx.requirePendingLaneChangeOn(fx.newBase)
	if cs.WorktreeBaseSHA != fx.newBase || storage.cycleState.WorktreeBaseSHA != fx.newBase {
		t.Fatalf("rebased base not persisted: memory=%q storage=%q want=%q", cs.WorktreeBaseSHA, storage.cycleState.WorktreeBaseSHA, fx.newBase)
	}
	if _, err := explanationdocs.LoadSnapshot(fx.bindingAt(fx.newBase)); err == nil {
		t.Fatal("the pre-rebase snapshot stayed loadable, so Build would not re-author it")
	}
}

func TestRecoverFromShipError_WithoutAnAuditedTreeRebasesTheShipCommit(t *testing.T) {
	fx := newShippedLane(t, "lane.txt", addLaneFile, addPeerFile)

	next, recovering, _ := fx.recover(&fakeStorage{})

	if !recovering || next != PhaseBuild {
		t.Fatalf("recovery=(%s,%v), want Build", next, recovering)
	}
	if parent := fx.git("rev-parse", "HEAD^"); parent != fx.newBase {
		t.Fatalf("ship commit not replayed onto the new base: HEAD^ = %s, want %s", parent, fx.newBase)
	}
	if _, err := os.Stat(filepath.Join(fx.worktree, inboxItem)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("no audited tree to restore, yet the consumption was undone: %v", err)
	}
}

func TestRecoverFromShipError_APredictedConflictIsNotUnwound(t *testing.T) {
	fx := newShippedLane(t, "shared.txt", editSharedLine(1, "lane edit"), editSharedLine(1, "peer edit"))

	fx.recover(&fakeStorage{}, fx.auditRow())

	if head := fx.git("rev-parse", "HEAD"); head != fx.shipCommit {
		t.Fatalf("HEAD = %s, want ship's commit %s left as it was: the pre-screen predicted the conflict", head, fx.shipCommit)
	}
	if _, err := os.Stat(filepath.Join(fx.worktree, ".evolve/inbox/consumed/item.json")); err != nil {
		t.Fatalf("ship's consumption was undone although no unwind should run: %v", err)
	}
}

func TestRecoverFromShipError_ALegacyCycleIsNotUnwound(t *testing.T) {
	fx := newShippedLane(t, "lane.txt", addLaneFile, addPeerFile)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{entries: []LedgerEntry{fx.auditRow()}}, buildRunners(nil))
	cs := fx.cycleState()
	cs.ExplanationDocumentationVersion = 0

	o.recoverFromShipError(context.Background(), fx.root, unwindCycle, &cs,
		NewShipError(CodeGitFleetRebaseNeeded, ShipClassTransient, StageAtomicShip, "peer moved main"), 0, 2)

	if parent := fx.git("rev-parse", "HEAD^"); parent != fx.newBase {
		t.Fatalf("HEAD^ = %s, want ship's commit replayed onto %s as before B1", parent, fx.newBase)
	}
	if _, err := os.Stat(filepath.Join(fx.worktree, ".evolve/inbox/consumed/item.json")); err != nil {
		t.Fatalf("a legacy cycle was unwound: %v", err)
	}
}
