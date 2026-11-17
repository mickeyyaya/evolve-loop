package orchestrator

import (
	"context"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const enforceBase = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

type redCI struct{}

func (redCI) Open(_ context.Context, b integration.Batch) (integration.PR, error) {
	return integration.PR{Number: 1, Tip: b.Tip}, nil
}

func (redCI) Watch(context.Context, integration.PR) (integration.Verdict, error) {
	return integration.Red, nil
}

func (redCI) Rerun(context.Context, integration.PR) error { return nil }

type fixedMain struct{}

func (fixedMain) Head(context.Context) (string, error)      { return enforceBase, nil }
func (fixedMain) FastForward(context.Context, string) error { return nil }

type briefsIn struct{ dir string }

func (b briefsIn) Write(_ context.Context, ej *integration.Ejection) (string, error) {
	return ej.WriteBrief(b.dir)
}

type sendsTo struct{ f *fixture }

func (p sendsTo) Emit(e signalcenter.Event) { p.f.send("ship", e) }

func (f *fixture) enforceActor() integration.EnforceActor {
	return integration.EnforceActor{
		Merger: integration.Merger{
			CI:   redCI{},
			Main: fixedMain{},
			Compose: func(_ context.Context, recs []integration.DeliverableRecord) (integration.Composition, error) {
				return integration.Composition{Batch: integration.Batch{Base: enforceBase, Tip: enforceBase}, Kept: recs}, nil
			},
		},
		Briefs:  briefsIn{dir: f.t.TempDir()},
		Publish: sendsTo{f: f},
	}
}

func TestOversight_AnEnforceEjectionBriefsItsLaneAndKeepsItsCellsHeldUntilTheLandingFreesThem(t *testing.T) {
	f := newFixture(t, StageEnforce)
	b := f.briefs(nil)
	f.addLane("A", 41)
	f.intake.Free("A")
	f.start()
	inFlight := []fleet.Todo{todoIn("A", "inboxrank")}
	if !f.admits(inFlight, todoIn("d", "inboxrank")) {
		t.Fatal("before the ejection, the cells of A are held, want them free")
	}

	res, err := f.enforceActor().Flush(context.Background(), integration.PendingBatch{
		Pool: 1, Index: 1, Base: enforceBase, Members: []integration.DeliverableRecord{{Cycle: 41}},
	})
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}

	if res.Members[41].Status != integration.MemberEjected {
		t.Fatalf("member 41 = %+v, want ejected", res.Members[41])
	}
	if lanes, texts := b.snapshot(); !reflect.DeepEqual(lanes, []string{"A"}) || len(texts) != 1 {
		t.Fatalf("briefs = %v %v, want one brief sent to lane A", lanes, texts)
	}
	if f.admits(inFlight, todoIn("d", "inboxrank")) {
		t.Fatal("after the ejection, the cells of A are free, want them held")
	}
	f.send("ship", landedEvent(41))
	if !f.admits(inFlight, todoIn("d", "inboxrank")) {
		t.Error("after the landing, the cells of A are still held, want them free")
	}
}
