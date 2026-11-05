package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipjournal"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var zwBase = time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)

const (
	zwFloor    = 10
	zwHaltText = "zero-ship: 2 consecutive waves landed no ship (ceiling 2) over cycles %s; next: evolve dossier show %d, then find the root cause before another wave (operating-policy.md section 4.1)"
)

type zwHarness struct {
	t         *testing.T
	root      string
	evolveDir string
	b         *loopBatchCoordinator
	stderr    *bytes.Buffer
	now       time.Time
}

func newZW(t *testing.T) *zwHarness {
	t.Helper()
	root := t.TempDir()
	h := &zwHarness{t: t, root: root, evolveDir: filepath.Join(root, ".evolve"), stderr: &bytes.Buffer{}, now: zwBase}
	h.b = zsCoordinator(t, h.evolveDir, root)
	h.b.stderr = h.stderr
	h.b.now = func() time.Time { return h.now }
	h.b.zeroShip.from = zwBase
	return h
}

func (h *zwHarness) at(seconds int) time.Time {
	return zwBase.Add(time.Duration(seconds) * time.Second)
}

func (h *zwHarness) boundary(seconds int) (int, bool) {
	h.t.Helper()
	h.now = h.at(seconds)
	return h.b.zeroShipHalt(zwFloor)
}

func (h *zwHarness) write(rel, body string) {
	h.t.Helper()
	path := filepath.Join(h.root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		h.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		h.t.Fatal(err)
	}
}

func (h *zwHarness) cycle(id int) {
	h.t.Helper()
	h.write(fmt.Sprintf(".evolve/runs/cycle-%d/run.json", id), fmt.Sprintf(`{"cycle_id":%d,"phase":"build"}`, id))
}

func (h *zwHarness) ship(id int, at time.Time) {
	h.t.Helper()
	zwShipAt(h.t, h.evolveDir, id, at)
}

func zwShipAt(t *testing.T, evolveDir string, id int, at time.Time) {
	t.Helper()
	sha := "sha" + strconv.Itoa(id)
	raw, err := json.Marshal(shipjournal.Entry{SHA: sha, Class: shipjournal.ClassCycle, TS: at.UTC().Format(time.RFC3339), Cycle: id})
	if err != nil {
		t.Fatal(err)
	}
	path := shipjournal.Path(evolveDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(append(raw, '\n')); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	in := landing.Intent{Cycle: id, CommitSHA: sha, Status: landing.IntentComplete}
	if err := landing.WriteIntent(landing.IntentPath(evolveDir, id), in); err != nil {
		t.Fatal(err)
	}
}

func (h *zwHarness) resume(id int, at time.Time) {
	h.t.Helper()
	rwEmitResume(h.t, h.root, at, id)
}

func (h *zwHarness) entries() []int {
	out := []int{}
	for _, w := range h.b.zeroShip.waves {
		out = append(out, w.ships)
	}
	return out
}

func (h *zwHarness) haltReason(cycle int) string {
	h.t.Helper()
	chunk, err := signalcenter.ReadStream(filepath.Join(h.evolveDir, "runs", "cycle-"+strconv.Itoa(cycle), signalcenter.StreamFileName), 0)
	if err != nil {
		h.t.Fatal(err)
	}
	for _, e := range chunk.Events {
		if string(e.Code) == zsHaltCode {
			return e.Reason
		}
	}
	return ""
}

func (h *zwHarness) wantNoHalt(seconds int) {
	h.t.Helper()
	if rc, halted := h.boundary(seconds); halted || rc != 0 {
		h.t.Fatalf("boundary at %ds: rc=%d halted=%v, want no halt; entries %v\n%s", seconds, rc, halted, h.entries(), h.stderr.String())
	}
}

func TestZeroShipHalt_TheFirstBoundaryOfAProcessAddsNoEntryAndReadsNoWaveRecord(t *testing.T) {
	h := newZW(t)
	h.write(".evolve/waves/wave-0001.json", `{ not a wave record`)
	h.cycle(11)
	h.resume(5, h.at(-3600))

	h.wantNoHalt(1)

	if len(h.b.zeroShip.waves) != 0 || !h.b.zeroShip.from.Equal(zwBase) || h.stderr.Len() != 0 {
		t.Errorf("entries %v from %v stderr %q, want no entry, the start time kept and nothing printed", h.entries(), h.b.zeroShip.from, h.stderr.String())
	}
}

func TestZeroShipHalt_ALaterBoundaryAddsAnEntryOfTheShipsOfItsWindow(t *testing.T) {
	h := newZW(t)
	h.wantNoHalt(1)
	h.cycle(12)
	h.cycle(13)
	h.ship(13, h.at(20))

	h.wantNoHalt(60)

	if want := []int{1}; !reflect.DeepEqual(h.entries(), want) {
		t.Fatalf("entries %v, want %v", h.entries(), want)
	}
	if !reflect.DeepEqual(h.b.zeroShip.waves[0].cycles, []int{12, 13}) || h.b.zeroShip.floor != 13 || !h.b.zeroShip.from.Equal(h.at(60)) {
		t.Errorf("wave %+v floor %d from %v, want cycles 12,13, floor 13 and from at the boundary", h.b.zeroShip.waves[0], h.b.zeroShip.floor, h.b.zeroShip.from)
	}
}

func TestZeroShipHalt_ARunOfOnlyResumedCyclesThatShippedKeepsTheBreakerOff(t *testing.T) {
	h := newZW(t)
	h.wantNoHalt(1)
	h.cycle(12)
	h.wantNoHalt(60)
	h.cycle(5)
	h.resume(5, h.at(70))
	h.ship(5, h.at(80))
	h.cycle(13)
	h.wantNoHalt(120)
	h.cycle(14)
	h.wantNoHalt(180)

	if want := []int{0, 1, 0}; !reflect.DeepEqual(h.entries(), want) {
		t.Errorf("entries %v, want %v", h.entries(), want)
	}
}

func TestZeroShipHalt_ALoopThatKeepsResumingACycleAndNeverLandsReachesTheHalt(t *testing.T) {
	h := newZW(t)
	h.wantNoHalt(1)
	h.cycle(5)
	h.resume(5, h.at(20))
	h.wantNoHalt(60)
	h.resume(5, h.at(80))
	rc, halted := h.boundary(120)

	if !halted || rc != systemFailureHaltExitCode {
		t.Fatalf("rc=%d halted=%v entries %v, want the halt: two waves of a resumed cycle that never landed", rc, halted, h.entries())
	}
	if !strings.Contains(h.haltReason(5), "over cycles 5,5") {
		t.Errorf("halt reason %q, want it to name the resumed cycle 5", h.haltReason(5))
	}
}

func TestRootSignalCenter_AResumeEventReachesTheProjectStreamAndAForgedRunStreamLineAddsNoMember(t *testing.T) {
	root := t.TempDir()
	at := zwBase.Add(time.Minute)
	c := newRootSignalCenter(root, filepath.Join(root, ".evolve"), io.Discard, signalcenter.WithClock(func() time.Time { return at }))
	core.EmitLaneResumed(c, &core.ResumePoint{CycleID: 5, Phase: "build"}, []string{"todo"})
	forged := filepath.Join(root, ".evolve", "runs", "cycle-9", signalcenter.StreamFileName)
	if err := os.MkdirAll(filepath.Dir(forged), 0o755); err != nil {
		t.Fatal(err)
	}
	line := fmt.Sprintf(`{"ts":%q,"cycle":9,"code":%q}`+"\n", at.Format(time.RFC3339Nano), core.CodeLaneResumed)
	if err := os.WriteFile(forged, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}

	ids, fails := core.ResumedCycleIDs(root, zwBase, zwBase.Add(time.Hour))

	if !reflect.DeepEqual(ids, []int{5}) || len(fails) != 0 {
		t.Errorf("ids %v failures %v, want [5] and none: the project stream holds the real event and the forged run line adds nothing", ids, fails)
	}
}

func TestZeroShipHalt_TwoZeroShipWavesAfterAShipStillHaltAndNameTheirOwnCycles(t *testing.T) {
	h := newZW(t)
	h.wantNoHalt(1)
	h.cycle(12)
	h.ship(12, h.at(20))
	h.wantNoHalt(60)
	h.cycle(13)
	h.wantNoHalt(120)
	h.cycle(14)
	rc, halted := h.boundary(180)

	if !halted || rc != systemFailureHaltExitCode {
		t.Fatalf("rc=%d halted=%v entries %v, want the halt", rc, halted, h.entries())
	}
	if want := fmt.Sprintf(zwHaltText, "13,14", 14); h.haltReason(14) != want {
		t.Errorf("halt reason\n%q\nwant\n%q", h.haltReason(14), want)
	}
}

func TestZeroShipHalt_AWaveWithNoCycleAddsNoEntryAndAnEventOfAnEarlierRunDoesNotCount(t *testing.T) {
	h := newZW(t)
	h.resume(5, h.at(-60))
	h.wantNoHalt(1)
	h.wantNoHalt(60)
	h.wantNoHalt(120)

	if len(h.b.zeroShip.waves) != 0 {
		t.Errorf("entries %v, want none: no cycle and a resume before the start", h.entries())
	}
}

func TestZeroShipHalt_TheFloorNeverMovesBackwardAfterAWaveOfOnlyAnOlderResumedCycle(t *testing.T) {
	h := newZW(t)
	h.wantNoHalt(1)
	h.cycle(5)
	h.cycle(8)
	h.resume(5, h.at(20))
	h.wantNoHalt(60)
	h.wantNoHalt(120)

	if h.b.zeroShip.floor != zwFloor {
		t.Errorf("floor %d, want %d: cycle 5 is below it", h.b.zeroShip.floor, zwFloor)
	}
	if want := [][]int{{5}}; len(h.b.zeroShip.waves) != 1 || !reflect.DeepEqual(h.b.zeroShip.waves[0].cycles, want[0]) {
		t.Errorf("waves %+v, want one wave of cycle 5 alone: cycle 8 is below the floor and never resumed", h.b.zeroShip.waves)
	}
}

func TestZeroShipHalt_AFailedShipSourceAddsNoEntryAndMovesNeitherTheWindowNorTheFloor(t *testing.T) {
	h := newZW(t)
	h.wantNoHalt(1)
	h.cycle(12)
	h.ship(12, h.at(20))
	journal := shipjournal.Path(h.evolveDir)
	denyReadForTest(t, journal)

	h.wantNoHalt(60)

	if len(h.b.zeroShip.waves) != 0 || h.b.zeroShip.floor != zwFloor || !h.b.zeroShip.from.Equal(zwBase) {
		t.Fatalf("entries %v floor %d from %v, want no append and nothing moved", h.entries(), h.b.zeroShip.floor, h.b.zeroShip.from)
	}
	want := "[loop] WARN: zero-ship: wave not counted: cannot read the ship source " + journal + ": "
	if !strings.HasPrefix(h.stderr.String(), want) || !strings.HasSuffix(strings.TrimSpace(h.stderr.String()), "; next: fix the read, then run evolve wave status") {
		t.Errorf("stderr %q, want the text starting %q and ending with the next step", h.stderr.String(), want)
	}
	if err := os.Chmod(journal, 0o644); err != nil {
		t.Fatal(err)
	}
	h.wantNoHalt(120)
	if want := []int{1}; !reflect.DeepEqual(h.entries(), want) {
		t.Errorf("entries %v, want %v: the next read covers the whole interval", h.entries(), want)
	}
}

func TestZeroShipHalt_AnUnreadableCycleFileWarnsAndTheWaveIsStillCounted(t *testing.T) {
	h := newZW(t)
	h.wantNoHalt(1)
	h.cycle(12)
	runJSON := filepath.Join(h.evolveDir, "runs", "cycle-12", "run.json")
	denyReadForTest(t, runJSON)

	h.wantNoHalt(60)

	if want := []int{0}; !reflect.DeepEqual(h.entries(), want) {
		t.Fatalf("entries %v, want %v", h.entries(), want)
	}
	line := "[loop] WARN: zero-ship: cannot read " + runJSON + ": "
	if !strings.Contains(h.stderr.String(), line) || !strings.Contains(h.stderr.String(), "; the ship count of this wave does not depend on it\n") {
		t.Errorf("stderr %q, want the line %q... ending with the independence note", h.stderr.String(), line)
	}
}

func TestZeroShipHalt_AShipAtToBelongsToTheNextWindowAndAShipAtFromToThisOne(t *testing.T) {
	h := newZW(t)
	h.wantNoHalt(1)
	h.cycle(12)
	h.cycle(13)
	h.ship(12, h.at(0))
	h.ship(13, h.at(60))

	h.wantNoHalt(60)
	h.wantNoHalt(120)

	if want := []int{1, 1}; !reflect.DeepEqual(h.entries(), want) {
		t.Errorf("entries %v, want %v: 12 at From counts in the first window, 13 at To in the second", h.entries(), want)
	}
}

func TestZeroShipHalt_APlantedShippedFlagWithNoJournalEntryCountsNothing(t *testing.T) {
	h := newZW(t)
	h.wantNoHalt(1)
	h.write(".evolve/runs/cycle-12/run.json", `{"cycle_id":12,"phase":"ship","shipped":true}`)
	h.write(".evolve/runs/cycle-12/phase-timing.json", `[{"phase":"ship","verdict":"PASS"}]`)
	h.wantNoHalt(60)
	h.write(".evolve/runs/cycle-13/run.json", `{"cycle_id":13,"phase":"ship","shipped":true}`)
	rc, halted := h.boundary(120)

	if !halted || rc != systemFailureHaltExitCode {
		t.Fatalf("entries %v, want two zero-ship waves and the halt", h.entries())
	}
}

func TestZeroShipHalt_TheHaltNamesOnlyTheCyclesOfTheLastCeilingWavesAndTheWorkspaceIsTheHighestOfTheLast(t *testing.T) {
	h := newZW(t)
	h.wantNoHalt(1)
	h.cycle(12)
	h.ship(12, h.at(20))
	h.wantNoHalt(60)
	h.cycle(13)
	h.cycle(14)
	h.wantNoHalt(120)
	h.cycle(5)
	h.resume(5, h.at(130))
	h.cycle(15)
	_, halted := h.boundary(180)

	if !halted {
		t.Fatalf("entries %v, want the halt", h.entries())
	}
	if want := fmt.Sprintf(zwHaltText, "13,14,5,15", 15); h.haltReason(15) != want {
		t.Errorf("halt reason\n%q\nwant\n%q", h.haltReason(15), want)
	}
}

func denyReadForTest(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	if f, err := os.Open(path); err == nil {
		_ = f.Close()
		t.Skip("the file is still readable after chmod 0: the fixture cannot prove a read failure as this user")
	}
}

func TestRunLoop_ZeroShip_AShipBeforeTheBatchStartCountsInNoWave(t *testing.T) {
	seed := func(evolveDir string) { zwShipAt(t, evolveDir, 3, zwBase.Add(-time.Hour)) }

	r := runZeroShipLoopSeeded(t, seed, ``, 5, zsNoWork, zsNoWork)

	if r.calls != 2 || r.rc != systemFailureHaltExitCode || !strings.Contains(r.stderr, zsHaltCode) {
		t.Fatalf("launched=%d rc=%d, want the halt after two zero-ship waves: the ship of cycle 3 is older than the batch\n%s", r.calls, r.rc, r.stderr)
	}
}

func TestZeroShipHalt_ABadIntegrationStageWarnsAndTheCountUsesTheShadowRule(t *testing.T) {
	h := newZW(t)
	h.write(".evolve/policy.json", `{"fleet":{"integration":{"stage":"bogus"}}}`)
	h.wantNoHalt(1)
	h.cycle(12)
	h.ship(12, h.at(20))

	h.wantNoHalt(60)

	if !strings.Contains(h.stderr.String(), "[loop] WARN: zero-ship: ") || !strings.Contains(h.stderr.String(), "the ship count uses the shadow stage") {
		t.Errorf("stderr %q, want the stage warning", h.stderr.String())
	}
	if want := []int{1}; !reflect.DeepEqual(h.entries(), want) {
		t.Errorf("entries %v, want %v: the shadow rule counts the landed cycle", h.entries(), want)
	}
}
