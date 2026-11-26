package inboxmover

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

const deliveredID = "delivered-item"

func deliveredFixture(t *testing.T) (Options, string) {
	t.Helper()
	opts, root := claimsFixture(t)
	opts.Ledger = &recordingAppender{}
	writeFile(t, deliveredFile(root, deliveredID), `{"id":"`+deliveredID+`","weight":0.5}`)
	return opts, root
}

func deliveredFile(root, id string) string {
	return filepath.Join(root, ".evolve", "inbox", "delivered", "2026-09-30T11-07-44Z-"+id+".json")
}

func mustExist(t *testing.T, path, why string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("%s: %v", why, err)
	}
}

type deliveredReader struct {
	name  string
	files []string
	check func(t *testing.T, opts Options, root string)
}

var deliveredReaders = []deliveredReader{
	{"menu builder", []string{"dispatchability.go"}, func(t *testing.T, opts Options, root string) {
		menu := PartitionLaneMenu(opts, []inboxbatch.Item{{ID: deliveredID}}, nil)
		if len(menu.Ready) != 0 || len(menu.Waiting) != 1 {
			t.Errorf("menu = ready %v waiting %v, want the delivered item waiting only", menu.Ready, menu.Waiting)
		}
		dependent := inboxbatch.Item{ID: "dependent", Deps: []string{deliveredID}}
		if place, reason := PlaceOnLaneMenu(opts, dependent, nil); place != MenuWaiting || !strings.Contains(reason, deliveredID) {
			t.Errorf("a dependent of a delivered item = %v %q, want waiting on %s", place, reason, deliveredID)
		}
	}},
	{"stale-claim sweep", []string{"claims.go"}, func(t *testing.T, opts Options, root string) {
		writeHolderRun(t, root, holderRun{cycle: 1828, goal: "g", phase: "build"})
		claim := claimItem(t, root, 1828, deliveredID)
		released, err := ReleaseStaleClaims(opts, "boundary")
		if err != nil || len(released) != 0 {
			t.Errorf("sweep = %+v, %v; want nothing released", released, err)
		}
		mustExist(t, claim, "the sweep must keep the claim of a delivered item")
	}},
	{"dispatchability", []string{"dispatchability.go", "dispatchstate.go", "inboxmover.go"}, func(t *testing.T, opts Options, root string) {
		if got := ResolveDispatchState(opts, deliveredID).State; got != StateDelivered {
			t.Errorf("state = %q, want %q", got, StateDelivered)
		}
		if d := ResolveDispatchability(opts, deliveredID); d.Dispatchable || !strings.Contains(d.Reason, "delivered") {
			t.Errorf("dispatchability = %+v, want not dispatchable and a delivered reason", d)
		}
	}},
	{"claim state", []string{"claimstate.go"}, func(t *testing.T, opts Options, root string) {
		opts.resolveOpts()
		if holder, held := HeldBy(opts.InboxDir, deliveredID); !held || holder == "" {
			t.Errorf("HeldBy = %q, %v; want a held claim", holder, held)
		}
	}},
	{"continuation release", []string{"continuation_release.go"}, func(t *testing.T, opts Options, root string) {
		seedRegistryOnly(t, root, deliveredID, 1828)
		if _, released, err := ReleaseContinuationBinding(opts, deliveredID, "test", "test"); err != nil || released {
			t.Errorf("release = %v, %v; want the binding kept", released, err)
		}
		if !stillBound(t, root, deliveredID) {
			t.Error("the binding of a delivered item must stay")
		}
	}},
	{"continuation reconcile", []string{"continuation_retire.go"}, func(t *testing.T, opts Options, root string) {
		seedRegistryOnly(t, root, deliveredID, 1828)
		writeFile(t, filepath.Join(root, ".evolve", "inbox", "consumed", "2026-09-30T11-07-44Z-"+deliveredID+".json"), `{"id":"`+deliveredID+`"}`)
		if released, err := ReconcileConsumedBindings(opts); err != nil || len(released) != 0 {
			t.Errorf("reconcile = %v, %v; want nothing released", released, err)
		}
		if !stillBound(t, root, deliveredID) {
			t.Error("the binding of a delivered item must stay")
		}
	}},
	{"retire unbacked", []string{"retire_unbacked.go"}, func(t *testing.T, opts Options, root string) {
		retired, err := RetireUnbacked(opts, 1828, StateProcessed, "r", "", []string{deliveredID})
		if err != nil || len(retired) != 0 {
			t.Errorf("retired = %v, %v; want none", retired, err)
		}
	}},
	{"outcome", []string{"outcome.go"}, func(t *testing.T, opts Options, root string) {
		for _, oc := range []CycleOutcome{
			{Cycle: 1828, Reason: "failed"},
			{Cycle: 1828, Passed: true, CommittedIDs: []string{deliveredID}},
		} {
			res, err := ApplyCycleOutcome(opts, oc)
			if err != nil || len(res.Released) != 0 || len(res.Promoted) != 0 || len(res.RetiredUnbacked) != 0 {
				t.Errorf("outcome %+v = %+v, %v; want no release of a delivered item", oc, res, err)
			}
		}
		mustExist(t, deliveredFile(root, deliveredID), "the delivered item stays")
	}},
	{"claim pending", []string{"claim_pending.go"}, func(t *testing.T, opts Options, root string) {
		pendingItemAtRoot(t, root, deliveredID, "")
		if err := ClaimPending(opts, 1900, []string{deliveredID}); err != nil {
			t.Errorf("ClaimPending = %v, want a quiet refusal", err)
		}
		if _, err := ClaimDispatchable(opts, deliveredID, "1900"); err == nil {
			t.Error("ClaimDispatchable of a delivered item must refuse")
		}
		if _, err := os.Stat(filepath.Join(root, ".evolve", "inbox", "processing", "cycle-1900")); err == nil {
			t.Error("a delivered item must not be claimed")
		}
	}},
	{"fix item", []string{"fix_item.go"}, func(t *testing.T, opts Options, root string) {
		if err := ConsumeFixItem(opts, deliveredID, PromoteOpts{}); err != nil {
			t.Errorf("ConsumeFixItem = %v", err)
		}
		mustExist(t, deliveredFile(root, deliveredID), "the delivered item stays")
	}},
}

func TestDeliveredState_EachReaderHandlesIt(t *testing.T) {
	for _, r := range deliveredReaders {
		t.Run(r.name, func(t *testing.T) {
			opts, root := deliveredFixture(t)
			r.check(t, opts, root)
		})
	}
}

var itemStateReaderTokens = []string{
	"ResolveDispatchState(", "ResolveDispatchability(", "PendingDispatchability(", "claimHolding(",
	"Locate(", "retirementStates", "ListClaims(", "FindFileByTaskID(", "PlaceOnLaneMenu(", "PartitionLaneMenu(",
}

func TestDeliveredState_EveryItemStateReaderIsInTheTable(t *testing.T) {
	covered := map[string]bool{}
	for _, r := range deliveredReaders {
		for _, f := range r.files {
			covered[f] = true
		}
	}
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, src := range sources {
		if strings.HasSuffix(src, "_test.go") || covered[src] {
			continue
		}
		body, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		for _, tok := range itemStateReaderTokens {
			if strings.Contains(string(body), tok) {
				t.Errorf("%s reads the item state (%s) but no row of deliveredReaders covers it", src, tok)
				break
			}
		}
	}
}
