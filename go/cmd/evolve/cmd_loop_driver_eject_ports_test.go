package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
)

const (
	ejectCycle = 41
	ejectItem  = "train-eject-journey"
	ejectBase  = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	ejectTip   = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)

type fixedMain struct{ head string }

func (m *fixedMain) Head(context.Context) (string, error)          { return m.head, nil }
func (m *fixedMain) FastForward(_ context.Context, t string) error { m.head = t; return nil }

type ejectFixture struct {
	actor         integration.EnforceActor
	inbox         string
	deliveredFile string
	batch         integration.PendingBatch
}

func newEjectFixture(t *testing.T) ejectFixture {
	t.Helper()
	b, _ := driverCoordinator(t)
	root := b.cfg.ProjectRoot
	inbox := filepath.Join(root, ".evolve", "inbox")
	deliveredFile := filepath.Join(inbox, "delivered", "2026-11-25T15-40-00Z-"+ejectItem+".json")
	if err := os.MkdirAll(filepath.Dir(deliveredFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(deliveredFile, []byte(`{"id":"`+ejectItem+`","title":"the original title","weight":0.5,"failure_count":0}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := committedset.WriteRecord(root, committedset.Record{Cycle: ejectCycle, ItemIDs: []string{ejectItem}}); err != nil {
		t.Fatal(err)
	}
	built, ok := b.productionEnforceActor(integration.Repo{Dir: root}, 7).(integration.EnforceActor)
	if !ok {
		t.Fatalf("productionEnforceActor did not return an integration.EnforceActor")
	}
	ejection := &integration.Ejection{Cycle: ejectCycle, Cause: integration.CauseConflict, Paths: []string{"a.txt"}}
	actor := integration.EnforceActor{
		Repo: built.Repo,
		Merger: integration.Merger{
			CI:   greenCI{},
			Main: &fixedMain{head: ejectBase},
			Compose: func(context.Context, []integration.DeliverableRecord) (integration.Composition, error) {
				return integration.Composition{Batch: integration.Batch{Base: ejectBase, Tip: ejectTip}, Ejected: []*integration.Ejection{ejection}}, nil
			},
		},
		Requeue: built.Requeue,
		Briefs:  built.Briefs,
		Promote: okPorts{},
	}
	batch := integration.PendingBatch{Pool: 7, Index: 1, Base: ejectBase, Members: []integration.DeliverableRecord{{Cycle: ejectCycle, Wave: "7"}}}
	return ejectFixture{actor: actor, inbox: inbox, deliveredFile: deliveredFile, batch: batch}
}

func TestProductionEnforceActor_AnEjectedMemberLeavesDeliveredForTheQueueWithItsBrief(t *testing.T) {
	t.Parallel()
	fx := newEjectFixture(t)

	res, err := fx.actor.Flush(context.Background(), fx.batch)

	if err != nil {
		t.Fatalf("Flush = %v, want the ejection to requeue without an error", err)
	}
	if got := res.Members[ejectCycle]; got.Status != integration.MemberEjected {
		t.Fatalf("member = %+v, want ejected", got)
	}
	if _, err := os.Stat(fx.deliveredFile); !os.IsNotExist(err) {
		t.Errorf("the item still sits in delivered/ after its ejection: %v", err)
	}
	queued, err := filepath.Glob(filepath.Join(fx.inbox, "*"+ejectItem+".json"))
	if err != nil || len(queued) != 1 {
		t.Fatalf("queue files for the item = %v, %v; want exactly one at the inbox root", queued, err)
	}
	raw, err := os.ReadFile(queued[0])
	if err != nil {
		t.Fatal(err)
	}
	var item map[string]any
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatal(err)
	}
	brief, _ := item["brief_path"].(string)
	if brief == "" || !strings.HasSuffix(brief, "brief-41.txt") {
		t.Fatalf("queued item brief_path = %q, want the path of brief-41.txt", brief)
	}
	if text, err := integration.ReadBrief(brief); err != nil || !strings.Contains(text, "a.txt") {
		t.Errorf("brief at %s = %q, %v; want the brief text that names the conflicted path", brief, text, err)
	}
	if item["title"] != "the original title" {
		t.Errorf("queued title = %v, want the original item text kept", item["title"])
	}
	if got, _ := item["failure_count"].(float64); got != 1 {
		t.Errorf("queued failure_count = %v, want 1 so that the convergence count bounds the retries", item["failure_count"])
	}
}

func TestProductionEnforceActor_ABriefThatCannotBeWrittenKeepsTheItemDelivered(t *testing.T) {
	t.Parallel()
	fx := newEjectFixture(t)
	waveDir := integration.WaveDir(filepath.Dir(filepath.Dir(fx.inbox)), "7")
	if err := os.MkdirAll(filepath.Dir(waveDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(waveDir, []byte("a file where the wave directory belongs"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := fx.actor.Flush(context.Background(), fx.batch)

	if err == nil {
		t.Errorf("Flush = nil, want the brief failure returned")
	}
	if !slices.ContainsFunc(res.Failures, func(f integration.DriverFailure) bool { return f.Step == "brief" }) {
		t.Errorf("failures = %+v, want one at step brief", res.Failures)
	}
	if _, statErr := os.Stat(fx.deliveredFile); statErr != nil {
		t.Errorf("the item left delivered/ although its brief was never written: %v", statErr)
	}
	if queued, _ := filepath.Glob(filepath.Join(fx.inbox, "*"+ejectItem+".json")); len(queued) != 0 {
		t.Errorf("the item reached the queue without a brief: %v", queued)
	}
}
