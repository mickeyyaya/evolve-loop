package main

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cycleblocker"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

const blockerFloor = 1

func blockerRecordBody(cycle int, blocker string, minutes float64) string {
	raw, _ := json.Marshal(cycleblocker.Record{Schema: cycleblocker.SchemaV1, Cycle: cycle, Blocker: blocker, Minutes: minutes})
	return string(raw)
}

func seedBlockerPlane(t *testing.T) (wavePlane, wave.Record) {
	t.Helper()
	p := newWavePlane(t)
	rec := wave.Record{Number: 9, RunID: "r9", PID: 9, CycleFloor: blockerFloor, StartedAt: waveTestClock}
	if err := p.store().Save(rec); err != nil {
		t.Fatal(err)
	}
	return p, rec
}

func seedSealedCycle(p wavePlane, id int, verdict string, wallMinutes int, llmMillis int, extraSignals string) {
	start := waveTestClock.Add(time.Minute)
	rel := ".evolve/runs/cycle-" + strconv.Itoa(id)
	p.write(rel+"/run.json", `{"cycle_id":`+strconv.Itoa(id)+`,"phase":"ship"}`)
	p.write(rel+"/signals.ndjson", waveSignalLine(start, "phase.dispatched", "", "")+extraSignals+
		waveSignalLine(start.Add(time.Duration(wallMinutes)*time.Minute), "cycle.sealed", "", `"final_verdict":"`+verdict+`"`))
	if llmMillis > 0 {
		p.write(rel+"/llm-calls.ndjson", `{"phase":"build","cli":"claude","duration_ms":`+strconv.Itoa(llmMillis)+`}`+"\n")
	}
}

func seedShippedWaiting(p wavePlane, id int) {
	seedSealedCycle(p, id, "PASS", 90, 20*60_000, "")
	rwLand(p.t, p, waveTestClock.Add(2*time.Hour), id, "sha")
}

func statusBlockers(t *testing.T, p wavePlane, rec wave.Record) waveStatusReport {
	t.Helper()
	return buildWaveStatus(newWaveHarness().env(p.root), rwPlaneDirs(p), []wave.Record{rec})
}

func blockerOf(r waveStatusReport, id int) (waveBlocker, bool) {
	for _, b := range r.Blockers {
		if b.Cycle == id {
			return b, true
		}
	}
	return waveBlocker{}, false
}

func TestWaveStatus_AShippedCycleWithWallFarAboveLLMNamesWaiting(t *testing.T) {
	t.Parallel()
	p, rec := seedBlockerPlane(t)
	seedShippedWaiting(p, 2)

	b, ok := blockerOf(statusBlockers(t, p, rec), 2)
	if !ok || b.Blocker != cycleblocker.Waiting || b.Minutes != 70 {
		t.Fatalf("blocker = %+v, %v; want waiting with 70 minutes (wall 90 minus LLM 20)", b, ok)
	}
}

func TestWaveStatus_TheThresholdComesFromPolicy(t *testing.T) {
	t.Parallel()
	p, rec := seedBlockerPlane(t)
	seedShippedWaiting(p, 2)
	p.write(".evolve/policy.json", `{"workflow":{"blocker_attribution":{"wait_share_pct":95}}}`)

	b, _ := blockerOf(statusBlockers(t, p, rec), 2)
	if b.Blocker != cycleblocker.None {
		t.Fatalf("blocker = %+v; want none: the share of 78%% is under the policy threshold of 95%%", b)
	}
}

func TestWaveStatus_AnUnreadablePolicyWarnsAndUsesTheDefaultThreshold(t *testing.T) {
	t.Parallel()
	p, rec := seedBlockerPlane(t)
	seedShippedWaiting(p, 2)
	p.write(".evolve/policy.json", `{"workflow":{"blocker_attribution":{"wait_share_pct":5}}}`)

	report := statusBlockers(t, p, rec)
	b, _ := blockerOf(report, 2)
	if b.Blocker != cycleblocker.Waiting || !strings.Contains(strings.Join(report.Warnings, "\n"), "blocker attribution: policy unreadable") {
		t.Fatalf("blocker %+v, warnings %q; want waiting at the default and a warning naming the key", b, report.Warnings)
	}
}

func TestWaveStatus_QuotaPausesMakeAShippedCycleNone(t *testing.T) {
	t.Parallel()
	p, rec := seedBlockerPlane(t)
	seedSealedCycle(p, 2, "PASS", 90, 20*60_000, waveSignalLine(waveTestClock.Add(5*time.Minute), "quota.paused", "", ""))
	rwLand(p.t, p, waveTestClock.Add(2*time.Hour), 2, "sha")

	b, ok := blockerOf(statusBlockers(t, p, rec), 2)
	if !ok || b.Blocker != cycleblocker.None {
		t.Fatalf("blocker = %+v, %v; want none for a paused cycle", b, ok)
	}
}

func TestWaveStatus_AFailCycleTakesItsSealTimeRecord(t *testing.T) {
	t.Parallel()
	p, rec := seedBlockerPlane(t)
	seedSealedCycle(p, 3, "FAIL", 14, 13*60_000, "")
	p.write(".evolve/failure-digests/cycle-3.blocker.json", blockerRecordBody(3, "check:ste-lint", 13.5))

	b, ok := blockerOf(statusBlockers(t, p, rec), 3)
	if !ok || b.Blocker != "check:ste-lint" || b.Minutes != 13.5 {
		t.Fatalf("blocker = %+v, %v; want the record check:ste-lint with 13.5 minutes", b, ok)
	}
}

func TestWaveStatus_AFailCycleWithNoRecordIsNone(t *testing.T) {
	t.Parallel()
	p, rec := seedBlockerPlane(t)
	seedSealedCycle(p, 3, "FAIL", 14, 13*60_000, "")

	report := statusBlockers(t, p, rec)
	b, ok := blockerOf(report, 3)
	if !ok || b.Blocker != cycleblocker.None || len(report.Warnings) != 0 {
		t.Fatalf("blocker = %+v, %v, warnings %q; want none and no warning for an old cycle", b, ok, report.Warnings)
	}
}

func TestWaveStatus_AFailCycleWithABadRecordWarnsAndIsNone(t *testing.T) {
	t.Parallel()
	p, rec := seedBlockerPlane(t)
	seedSealedCycle(p, 3, "FAIL", 14, 13*60_000, "")
	p.write(".evolve/failure-digests/cycle-3.blocker.json", `{"schema":"evolve.blocker/9","cycle":3}`)

	report := statusBlockers(t, p, rec)
	b, _ := blockerOf(report, 3)
	if b.Blocker != cycleblocker.None || !strings.Contains(strings.Join(report.Warnings, "\n"), "evolve.blocker/9") {
		t.Fatalf("blocker %+v, warnings %q; want none and a warning that names the schema", b, report.Warnings)
	}
}

func TestWaveStatus_ACycleStillRunningHasNoBlocker(t *testing.T) {
	t.Parallel()
	p, rec := seedBlockerPlane(t)
	p.write(".evolve/runs/cycle-4/run.json", `{"cycle_id":4,"phase":"build"}`)

	if b, ok := blockerOf(statusBlockers(t, p, rec), 4); ok {
		t.Fatalf("blocker = %+v; want none listed for an open cycle", b)
	}
}

func seedMixedBlockers(t *testing.T) (wavePlane, wave.Record) {
	t.Helper()
	p, rec := seedBlockerPlane(t)
	seedSealedCycle(p, 3, "FAIL", 14, 13*60_000, "")
	p.write(".evolve/failure-digests/cycle-3.blocker.json", blockerRecordBody(3, "check:ste-lint", 13.5))
	seedSealedCycle(p, 4, "FAIL", 9, 8*60_000, "")
	p.write(".evolve/failure-digests/cycle-4.blocker.json", blockerRecordBody(4, "class:audit|verdict-fail", 8))
	seedSealedCycle(p, 5, "FAIL", 9, 8*60_000, "")
	seedShippedWaiting(p, 2)
	return p, rec
}

func TestWaveStatus_JSONCountsBlockersByKind(t *testing.T) {
	t.Parallel()
	p, _ := seedMixedBlockers(t)
	h := newWaveHarness()
	seedSealedCycle(p, 6, "PASS", 10, 9*60_000, "")
	rwLand(p.t, p, waveTestClock.Add(2*time.Hour), 6, "sha6")
	if rc := h.run("status", "--json", "--project-root", p.root); rc != 0 {
		t.Fatalf("rc = %d: %s", rc, h.stderr.String())
	}
	var out struct {
		BlockerCounts map[string]int `json:"blocker_counts"`
	}
	if err := json.Unmarshal(h.stdout.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"check": 1, "class": 1, "none": 1, "waiting": 1}
	for k, v := range want {
		if out.BlockerCounts[k] != v {
			t.Errorf("blocker_counts = %v; want %v", out.BlockerCounts, want)
			break
		}
	}
}

func TestWaveStatus_TextPrintsTheBlockerOfACycle(t *testing.T) {
	t.Parallel()
	p, _ := seedMixedBlockers(t)
	h := newWaveHarness()
	if rc := h.run("status", "--project-root", p.root); rc != 0 {
		t.Fatalf("rc = %d: %s", rc, h.stderr.String())
	}
	lines := map[int]string{}
	for _, line := range strings.Split(h.stdout.String(), "\n") {
		for _, id := range []int{2, 3, 4, 5} {
			if strings.HasPrefix(strings.TrimSpace(line), "cycle "+strconv.Itoa(id)+":") {
				lines[id] = line
			}
		}
	}
	for id, want := range map[int]string{2: "blocker waiting (70.0 min)", 3: "blocker check:ste-lint (13.5 min)", 4: "blocker class:audit|verdict-fail (8.0 min)"} {
		if !strings.Contains(lines[id], want) {
			t.Errorf("cycle %d line %q; want it to hold %q", id, lines[id], want)
		}
	}
	if strings.Contains(lines[5], "blocker") {
		t.Errorf("cycle 5 line %q; a none blocker prints no blocker text", lines[5])
	}
}
