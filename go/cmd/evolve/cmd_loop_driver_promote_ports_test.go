package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
)

const (
	promoteCycle    = 43
	promoteItem     = "train-promote-journey"
	promoteDeferred = "train-promote-deferred"
	promoteBase     = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	promoteTip      = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)

type promoteFixture struct {
	actor         integration.EnforceActor
	inbox         string
	deliveredFile string
	deferredFile  string
	batch         integration.PendingBatch
}

func newPromoteFixture(t *testing.T) promoteFixture {
	t.Helper()
	b, _ := driverCoordinator(t)
	root := b.cfg.ProjectRoot
	inbox := filepath.Join(root, ".evolve", "inbox")
	deliveredFile := filepath.Join(inbox, "delivered", "2026-11-25T15-40-00Z-"+promoteItem+".json")
	deferredFile := filepath.Join(inbox, "2026-11-25T15-41-00Z-"+promoteDeferred+".json")
	if err := os.MkdirAll(filepath.Dir(deliveredFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(deliveredFile, []byte(`{"id":"`+promoteItem+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(deferredFile, []byte(`{"id":"`+promoteDeferred+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := committedset.WriteRecord(root, committedset.Record{Cycle: promoteCycle, ItemIDs: []string{promoteItem, promoteDeferred}}); err != nil {
		t.Fatal(err)
	}
	built, ok := b.productionEnforceActor(integration.Repo{Dir: root}, 7).(integration.EnforceActor)
	if !ok {
		t.Fatalf("productionEnforceActor did not return an integration.EnforceActor")
	}
	member := integration.DeliverableRecord{Cycle: promoteCycle, Wave: "7"}
	actor := integration.EnforceActor{
		Repo: built.Repo,
		Merger: integration.Merger{
			CI:   greenCI{},
			Main: &fixedMain{head: promoteBase},
			Compose: func(context.Context, []integration.DeliverableRecord) (integration.Composition, error) {
				return integration.Composition{Batch: integration.Batch{Base: promoteBase, Tip: promoteTip}, Kept: []integration.DeliverableRecord{member}}, nil
			},
		},
		Requeue: built.Requeue,
		Briefs:  built.Briefs,
		Promote: built.Promote,
	}
	batch := integration.PendingBatch{Pool: 7, Index: 1, Base: promoteBase, Members: []integration.DeliverableRecord{member}}
	return promoteFixture{actor: actor, inbox: inbox, deliveredFile: deliveredFile, deferredFile: deferredFile, batch: batch}
}

func TestProductionEnforceActor_APromoterIsWiredSoMissingNamesNoPromote(t *testing.T) {
	t.Parallel()
	b, _ := driverCoordinator(t)
	actor, ok := b.productionEnforceActor(integration.Repo{Dir: b.cfg.ProjectRoot}, 7).(integration.EnforceActor)
	if !ok {
		t.Fatal("productionEnforceActor does not return an EnforceActor")
	}
	if actor.Promote == nil {
		t.Errorf("the production enforce actor holds no Promoter")
	}
	if slices.Contains(actor.Missing(), "Promote") {
		t.Errorf("Missing() = %v, want no Promote", actor.Missing())
	}
}

func TestProductionEnforceActor_AMergedMemberMovesItsDeliveredItemToProcessedAndAnOpenDeferredIdStays(t *testing.T) {
	t.Parallel()
	fx := newPromoteFixture(t)

	res, err := fx.actor.Flush(context.Background(), fx.batch)

	if err != nil {
		t.Fatalf("Flush = %v", err)
	}
	if got := res.Members[promoteCycle]; got.Status != integration.MemberMerged {
		t.Fatalf("member = %+v, want merged", got)
	}
	if _, err := os.Stat(fx.deliveredFile); !os.IsNotExist(err) {
		t.Errorf("the item still sits in delivered/ after its member merged: %v", err)
	}
	processed, err := filepath.Glob(filepath.Join(fx.inbox, "processed", "cycle-43", "*"+promoteItem+".json"))
	if err != nil || len(processed) != 1 {
		t.Fatalf("processed files for the item = %v, %v; want exactly one under processed/cycle-43/", processed, err)
	}
	if _, err := os.Stat(fx.deferredFile); err != nil {
		t.Errorf("the deferred id left the inbox root: %v", err)
	}
	if slices.ContainsFunc(res.Failures, func(f integration.DriverFailure) bool { return strings.Contains(f.Step, "promote") }) {
		t.Errorf("failures = %+v, want no promote failure", res.Failures)
	}
}
