package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

func (p pausedLane) writeScope(t *testing.T, ids ...string) {
	t.Helper()
	body, _ := json.Marshal(core.LaneScope{TodoIDs: ids, GoalHash: "paused-goal"})
	if err := os.WriteFile(filepath.Join(paths.RunWorkspace(p.root, p.cycle), core.LaneScopeFile), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLaneResume_TheSettleStepWalksTheScopeOrderOfTheGateNotTheSortedFleetScope(t *testing.T) {
	t.Parallel()
	p := seedPausedScope(t, []string{"item-a", "item-b"}, 2110, 0)
	p.writeScope(t, "item-b", "item-a")
	p.bind(t, "item-a", 2093)
	p.bind(t, "item-b", 2094)
	p.ancestorLedger(t, 2093, owedLedger)
	p.ancestorLedger(t, 2094, owedLedger)

	p.resume(t, &bytes.Buffer{}, core.ScopeIDs("item-b,item-a")...)

	m, ok, _ := continuation.ReadManifest(paths.RunWorkspace(p.root, 2110))
	if !ok || m.Cycle != 2094 {
		t.Fatalf("manifest = %+v, %v; want the first binding of the gate order, cycle 2094", m, ok)
	}
	p.requireNoBookkeepingBlock(t)
}

func TestLaneResume_AFleetLaneWithAnUnsortedScopeFileAgreesWithTheGate(t *testing.T) {
	p := seedPausedScope(t, []string{"item-a", "item-b"}, 2110, 0)
	p.writeScope(t, "item-b", "item-a")
	p.bind(t, "item-a", 2093)
	p.bind(t, "item-b", 2094)
	p.ancestorLedger(t, 2093, owedLedger)
	p.ancestorLedger(t, 2094, owedLedger)
	t.Setenv(ipcenv.FleetKey, "1")
	t.Setenv(ipcenv.FleetScopeKey, "item-b,item-a")
	var got startedCycle
	lane := laneResume{projectRoot: p.root, evolveDir: p.evolveDir, scope: fleetLaneScope(), record: checkpoint.RecordResume,
		claim: hostInboxClaimer(nil, nil, nil), stderr: &bytes.Buffer{}, release: laneClaimReleaser(p.root, &bytes.Buffer{}, nil, nil)}

	if _, err := lane.run(context.Background(), recordingStarter{&got}, laneRequest()); err != nil || got.resumed == nil {
		t.Fatalf("run: %v, resumed %+v", err, got.resumed)
	}

	p.requireNoBookkeepingBlock(t)
}

func TestLaneResume_ARefusedResumeLeavesTheBindingAndTheWorkspaceUntouched(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2109)
	p.bind(t, "item-a", 2092)
	lane := p.lane(&bytes.Buffer{}, nil, "item-a")
	lane.record = func(string, bool) (checkpoint.Checkpoint, bool, error) { return checkpoint.Checkpoint{}, false, nil }
	var got startedCycle

	if _, err := lane.run(context.Background(), recordingStarter{&got}, laneRequest()); err != nil || got.resumed != nil {
		t.Fatalf("run: %v, resumed %+v; want a fresh cycle", err, got.resumed)
	}

	if _, ok, _ := continuation.ReadRegistryEntry(p.root, "item-a"); !ok {
		t.Fatal("a refused resume released the binding")
	}
	if _, ok, _ := continuation.ReadManifest(paths.RunWorkspace(p.root, 2109)); ok {
		t.Fatal("a refused resume wrote a manifest")
	}
}

func TestLaneResume_AnUnboundIdFirstDoesNotEndTheWalk(t *testing.T) {
	t.Parallel()
	p := seedPausedScope(t, []string{"item-free", "item-a"}, 2108, 0)
	p.bind(t, "item-a", 2093)
	p.ancestorLedger(t, 2093, owedLedger)

	p.resume(t, &bytes.Buffer{}, "item-free", "item-a")

	if m, ok, _ := continuation.ReadManifest(paths.RunWorkspace(p.root, 2108)); !ok || m.Cycle != 2093 {
		t.Fatalf("manifest = %+v, %v; want the binding behind the unbound id recorded", m, ok)
	}
}

func TestLaneResume_AReleasedBindingLetsTheWalkReachTheNextOne(t *testing.T) {
	t.Parallel()
	p := seedPausedScope(t, []string{"item-a", "item-b"}, 2110, 0)
	p.bind(t, "item-a", 2092)
	p.bind(t, "item-b", 2093)
	p.ancestorLedger(t, 2093, owedLedger)

	p.resume(t, &bytes.Buffer{}, "item-a", "item-b")

	if _, ok, _ := continuation.ReadRegistryEntry(p.root, "item-a"); ok {
		t.Error("item-a stayed bound; want it released")
	}
	if m, ok, _ := continuation.ReadManifest(paths.RunWorkspace(p.root, 2110)); !ok || m.Cycle != 2093 {
		t.Fatalf("manifest = %+v, %v; want the next binding recorded after the release", m, ok)
	}
	p.requireNoBookkeepingBlock(t)
}

func TestLaneResume_TheRecordedManifestIsTheWholeRegistryBinding(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2108)
	want := p.bind(t, "item-a", 2093)
	p.ancestorLedger(t, 2093, owedLedger)

	p.resume(t, &bytes.Buffer{}, "item-a")

	got, ok, err := continuation.ReadManifest(paths.RunWorkspace(p.root, 2108))
	if err != nil || !ok || got != want {
		t.Fatalf("manifest = %+v, %v, %v; want the registry binding %+v whole", got, ok, err, want)
	}
}

func (p pausedLane) settleEvents(t *testing.T, scope ...string) []signalcenter.Event {
	t.Helper()
	signals := signalcenter.New()
	var events []signalcenter.Event
	signals.Subscribe(func(e signalcenter.Event) {
		if e.Code == core.CodeLaneBindingSettled {
			events = append(events, e)
		}
	})
	var got startedCycle
	if _, err := p.lane(&bytes.Buffer{}, signals, scope...).run(context.Background(), recordingStarter{&got}, laneRequest()); err != nil || got.resumed == nil {
		t.Fatalf("resume: %v, resumed %+v", err, got.resumed)
	}
	return events
}

func TestLaneResume_ADroppedBindingWithNoAncestorLedgerLeavesATraceAndNamesTheStanding(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2109)
	p.bind(t, "item-a", 2092)

	events := p.settleEvents(t, "item-a")

	if len(events) != 1 || events[0].Fields["action"] != "released" || events[0].Fields["standing"] != "absent" || events[0].Fields["ancestor_cycle"] != "2092" {
		t.Fatalf("events = %+v; want one released event with standing absent", events)
	}
	loc, _ := inboxmover.Locate(filepath.Join(p.evolveDir, "inbox"), "item-a")
	if body, _ := os.ReadFile(loc.Path); !strings.Contains(string(body), "owes no defect (ledger absent)") {
		t.Errorf("item = %s; want the reason to name the absent ledger", body)
	}
}

func TestLaneResume_ASettledAncestorLedgerReleasesWithTheSettledStanding(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2109)
	p.bind(t, "item-a", 2092)
	p.ancestorLedger(t, 2092, `{"origin_cycle":2092,"entries":[{"id":"d1","text":"x","status":"FIXED","source":"cycle-2091","evidence":"go/x.go"}]}`)

	events := p.settleEvents(t, "item-a")

	if len(events) != 1 || events[0].Fields["standing"] != "settled" || events[0].Fields["action"] != "released" {
		t.Fatalf("events = %+v; want one released event with standing settled", events)
	}
	if _, ok, _ := continuation.ReadRegistryEntry(p.root, "item-a"); ok {
		t.Error("registry still binds item-a; want a settled ancestor released")
	}
	loc, _ := inboxmover.Locate(filepath.Join(p.evolveDir, "inbox"), "item-a")
	if body, _ := os.ReadFile(loc.Path); !strings.Contains(string(body), "(ledger settled)") {
		t.Errorf("item = %s; want the reason to name the settled ledger", body)
	}
}

func TestLaneResume_AnUnreadableAncestorLedgerReachesTheEventAsUnreadable(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2109)
	p.bind(t, "item-a", 2092)
	p.ancestorLedger(t, 2092, `{not json`)

	events := p.settleEvents(t, "item-a")

	if len(events) != 1 || events[0].Fields["action"] != "recorded" || events[0].Fields["standing"] != "unreadable" {
		t.Fatalf("events = %+v; want one recorded event with standing unreadable", events)
	}
}

func TestLaneResume_ANewerBindingReachesTheEventAsNewer(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2090)
	p.bind(t, "item-a", 2107)

	events := p.settleEvents(t, "item-a")

	if len(events) != 1 || events[0].Fields["action"] != "recorded" || events[0].Fields["standing"] != "newer" {
		t.Fatalf("events = %+v; want one recorded event with standing newer", events)
	}
}

func TestScopeBinding_AFailedReleaseEmitsOnlyTheRecordedEvent(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2109)
	p.bind(t, "item-a", 2092)
	signals := signalcenter.New()
	var actions []string
	signals.Subscribe(func(e signalcenter.Event) {
		if e.Code == core.CodeLaneBindingSettled {
			actions = append(actions, e.Fields["action"])
		}
	})
	b := scopeBinding{projectRoot: p.root, stderr: &bytes.Buffer{}, signals: signals,
		release: func(inboxmover.ReleaseRequest) (bool, error) { return false, errors.New("boom") }}

	b.settle(2109)

	if len(actions) != 1 || actions[0] != "recorded" {
		t.Fatalf("actions = %v; want exactly one recorded event after a failed release", actions)
	}
}

func TestLaneResume_AManifestThatCannotBeWrittenEmitsNoEvent(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2108)
	p.bind(t, "item-a", 2093)
	p.ancestorLedger(t, 2093, owedLedger)
	if err := os.MkdirAll(filepath.Join(paths.RunWorkspace(p.root, 2108), continuation.ManifestName+".tmp"), 0o755); err != nil {
		t.Fatal(err)
	}

	events := p.settleEvents(t, "item-a")

	if len(events) != 0 {
		t.Fatalf("events = %+v; want none when no record was written", events)
	}
}

func TestLaneResume_AFailedReclaimStartsAFreshCycleAndKeepsTheBinding(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2109)
	p.bind(t, "item-a", 2092)
	lane := p.lane(&bytes.Buffer{}, nil, "item-a")
	lane.claim = func(string, int, []string) error { return errors.New("claim boom") }
	var got startedCycle

	if _, err := lane.run(context.Background(), recordingStarter{&got}, laneRequest()); err != nil || got.resumed != nil {
		t.Fatalf("run: %v, resumed %+v; want a fresh cycle", err, got.resumed)
	}

	if _, ok, _ := continuation.ReadRegistryEntry(p.root, "item-a"); !ok {
		t.Fatal("a failed reclaim released the binding; the debt would be lost with no cycle carrying it")
	}
}

func TestLaneResume_ARecordedBindingLeavesATraceOfTheMissingManifest(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2108)
	p.bind(t, "item-a", 2093)
	p.ancestorLedger(t, 2093, owedLedger)

	events := p.settleEvents(t, "item-a")

	if len(events) != 1 || events[0].Fields["action"] != "recorded" || events[0].Fields["standing"] != "owed" {
		t.Fatalf("events = %+v; want one recorded event with standing owed", events)
	}
}

func TestScopeBinding_AReleaseThatDeletesNothingRecordsTheBinding(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2109)
	c := p.bind(t, "item-a", 2092)
	var stderr bytes.Buffer
	b := scopeBinding{projectRoot: p.root, stderr: &stderr, release: func(inboxmover.ReleaseRequest) (bool, error) { return false, nil }}

	b.settle(2109)

	if m, ok, _ := continuation.ReadManifest(paths.RunWorkspace(p.root, 2109)); !ok || m != c {
		t.Fatalf("manifest = %+v, %v; want the binding recorded after a release that deleted nothing", m, ok)
	}
	if !strings.Contains(stderr.String(), "(released false, err <nil>)") {
		t.Errorf("stderr = %q; want the no-op release reported", stderr.String())
	}
}

func TestScopeBinding_TheReleaseNamesTheCycleItCheckedSoARebindIsNotReleased(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2109)
	p.bind(t, "item-a", 2092)
	var got inboxmover.ReleaseRequest
	b := scopeBinding{projectRoot: p.root, stderr: &bytes.Buffer{}, release: func(r inboxmover.ReleaseRequest) (bool, error) { got = r; return false, nil }}

	b.settle(2109)

	if got.ScopeID != "item-a" || got.Cycle != 2092 {
		t.Fatalf("release request = %+v; want item-a pinned to the checked cycle 2092", got)
	}
}

func TestLaneResume_AnUnreadableAncestorLedgerIsNamedInTheLog(t *testing.T) {
	t.Parallel()
	p := seedPausedLane(t, "item-a", 2109)
	p.bind(t, "item-a", 2092)
	p.ancestorLedger(t, 2092, `{not json`)
	var stderr bytes.Buffer

	p.resume(t, &stderr, "item-a")

	if !strings.Contains(stderr.String(), "cannot read the ledger of cycle-2092") {
		t.Errorf("stderr = %q; want the unreadable ledger named", stderr.String())
	}
}

func (p pausedLane) requireNoBookkeepingBlock(t *testing.T) {
	t.Helper()
	for _, d := range p.gate().Diagnostics {
		if strings.Contains(d.Message, "holds no continuation manifest") || strings.Contains(d.Message, "registry binds this lane") {
			t.Fatalf("gate diagnostic = %q; want the manifest to agree with the registry", d.Message)
		}
	}
}
