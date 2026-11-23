package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func (p pausedLane) freshSettle(t *testing.T) (*continuation.Continuation, *bytes.Buffer, []signalcenter.Event) {
	t.Helper()
	signals := signalcenter.New()
	var events []signalcenter.Event
	signals.Subscribe(func(e signalcenter.Event) {
		if e.Code == core.CodeLaneBindingSettled {
			events = append(events, e)
		}
	})
	var stderr bytes.Buffer
	got := freshBindingSettler(p.root, p.evolveDir, &stderr, signals)(p.cycle, paths.RunWorkspace(p.root, p.cycle))
	return got, &stderr, events
}

func (p pausedLane) freshGate() (defectledger.Verdict, *[]signalcenter.Event) {
	signals := signalcenter.New()
	events := &[]signalcenter.Event{}
	signals.Subscribe(func(e signalcenter.Event) { *events = append(*events, e) })
	gate := defectledger.New(core.LaneScopeIDs, func(string, defectledger.Request) (bool, string) { return false, "stub" },
		defectledger.WithSignals(func() *signalcenter.Center { return signals }))
	ws := paths.RunWorkspace(p.root, p.cycle)
	return gate.Reconcile(defectledger.Request{Cycle: p.cycle, Workspace: ws, ProjectRoot: p.root}), events
}

func TestFreshBindingSettler_ReleasesBindingOfLedgerlessAncestor(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2109)
	p.bind(t, "item-a", 2092)
	if v, _ := p.freshGate(); !v.Blocked {
		t.Fatalf("gate = %+v; want the unsettled binding to block before the settler runs", v)
	}

	got, stderr, events := p.freshSettle(t)

	if got != nil {
		t.Errorf("settler result = %+v; want nil after a release", got)
	}
	if _, ok, _ := continuation.ReadRegistryEntry(p.root, "item-a"); ok {
		t.Error("registry still binds item-a; want the binding released")
	}
	loc, err := inboxmover.Locate(filepath.Join(p.evolveDir, "inbox"), "item-a")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(loc.Path)
	if !strings.Contains(string(body), "runtime (fresh start)") || !strings.Contains(string(body), "fresh-start-cycle-2109") {
		t.Errorf("item = %s; want the release recorded by fresh-start with its reason", body)
	}
	if len(events) != 1 || events[0].Fields["action"] != "released" || events[0].Fields["standing"] != "absent" || events[0].Fields["origin"] != "fresh-start" {
		t.Errorf("events = %+v; want one released event with standing absent and origin fresh-start", events)
	}
	if !strings.Contains(stderr.String(), "RELEASED the binding of item-a to cycle-2092") {
		t.Errorf("stderr = %q; want the release line", stderr.String())
	}
	if v, _ := p.freshGate(); v.Blocked {
		t.Fatalf("gate = %+v; want no block after the release", v)
	}
}

func TestFreshBindingSettler_RecordsBindingOfAncestorThatOwes(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2108)
	want := p.bind(t, "item-a", 2093)
	p.ancestorLedger(t, 2093, owedLedger)

	got, stderr, events := p.freshSettle(t)

	if got == nil || *got != want {
		t.Fatalf("settler result = %+v; want the recorded binding %+v", got, want)
	}
	ws := paths.RunWorkspace(p.root, 2108)
	if m, ok, _ := continuation.ReadManifest(ws); !ok || m.Cycle != 2093 {
		t.Fatalf("manifest = %+v, %v; want the binding to cycle-2093 recorded", m, ok)
	}
	if len(events) != 1 || events[0].Fields["action"] != "recorded" || events[0].Fields["origin"] != "fresh-start" {
		t.Errorf("events = %+v; want one recorded event from fresh-start", events)
	}
	if !strings.Contains(stderr.String(), "RECORDED the binding of item-a to cycle-2093 in its manifest before its audit") {
		t.Errorf("stderr = %q; want the record line", stderr.String())
	}

	core.SeedDispositionSkeleton(ws, p.root, got.Cycle)
	v, events2 := p.freshGate()
	if !v.Blocked {
		t.Fatalf("gate = %+v; want the seeded but unfilled dispositions to block", v)
	}
	var codes []signalcenter.Code
	for _, e := range *events2 {
		codes = append(codes, e.Code)
	}
	if !containsCode(codes, defectledger.CodeDefectsUnaccounted) || containsCode(codes, defectledger.CodeManifestMissing) {
		t.Errorf("codes = %v; want unaccounted defects and no missing manifest", codes)
	}

	filled := `{"dispositions":[{"id":"d1","status":"DEFERRED","reason":"owned by a later cycle"}]}`
	if err := os.WriteFile(filepath.Join(ws, defectledger.DispositionsFile), []byte(filled), 0o644); err != nil {
		t.Fatal(err)
	}
	if v, _ := p.freshGate(); v.Blocked {
		t.Fatalf("gate = %+v; want no block once the builder accounts for each row", v)
	}
}

func containsCode(codes []signalcenter.Code, want signalcenter.Code) bool {
	for _, c := range codes {
		if c == want {
			return true
		}
	}
	return false
}

func TestFreshBindingSettler_KeepsAnExistingManifest(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2109)
	p.bind(t, "item-a", 2092)
	ws := paths.RunWorkspace(p.root, 2109)
	if err := continuation.WriteManifest(ws, continuation.Continuation{Cycle: 2092, SnapshotSHA: "deadbeef"}); err != nil {
		t.Fatal(err)
	}

	got, _, events := p.freshSettle(t)

	if got != nil || len(events) != 0 {
		t.Errorf("result %+v, events %+v; want nothing settled", got, events)
	}
	if _, ok, _ := continuation.ReadRegistryEntry(p.root, "item-a"); !ok {
		t.Error("a binding the workspace already recorded was released")
	}
}

func TestWireOrchestratorDeps_RegistersTheFreshBindingSettler(t *testing.T) {
	p := seedPausedLane(t, "item-a", 2109)
	p.bind(t, "item-a", 2092)

	deps := wireOrchestratorDeps(p.root, p.evolveDir, io.Discard, routingRun{})
	got, wired := deps.Orchestrator.BindingSettleProbe(p.cycle, paths.RunWorkspace(p.root, p.cycle))

	if !wired || got != nil {
		t.Errorf("settled = %+v, wired = %v; the composition root must register the binding settler", got, wired)
	}
	if _, ok, _ := continuation.ReadRegistryEntry(p.root, "item-a"); ok {
		t.Error("registry still binds item-a; the wired settler must release the binding of a ledgerless ancestor")
	}
}

func TestFreshBindingSettler_AFailedRecordReturnsNothing(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2109)
	p.bind(t, "item-a", 2093)
	p.ancestorLedger(t, 2093, owedLedger)
	ws := paths.RunWorkspace(p.root, 2109)
	if err := os.Chmod(ws, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ws, 0o755) })

	got, stderr, events := p.freshSettle(t)

	if got != nil || len(events) != 0 {
		t.Errorf("settler result = %+v, events = %d; want nil and no event when the manifest cannot be written", got, len(events))
	}
	if !strings.Contains(stderr.String(), "cannot record the binding") {
		t.Errorf("stderr = %q; want the record failure line", stderr.String())
	}
}
