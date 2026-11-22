//go:build acs

package cycle2598

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	evolveCmdPkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
	waveHours    = 4
	waveOnePID   = 7001
	minute       = int64(time.Minute)
)

var waveStart = time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)

type phaseRun struct {
	name      string
	from, to  int
	modelFrom int
	modelTo   int
}

type queueWait struct {
	minute int
	waitMS string
}

func buildOnly(modelUntil int) phaseRun {
	return phaseRun{name: "build", from: 0, to: 100, modelFrom: 0, modelTo: modelUntil}
}

func at(m int) string { return waveStart.Add(time.Duration(m) * time.Minute).Format(time.RFC3339Nano) }

func replayWave(t *testing.T, phases []phaseRun, waits []queueWait) string {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	runDir := filepath.Join(evolveDir, "runs", "cycle-1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"state.json": `{"lastCycleNumber":1}`, "signals.ndjson": ""} {
		if err := os.WriteFile(filepath.Join(evolveDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	end := waveStart.Add(waveHours * time.Hour)
	rec := wave.Record{Number: 1, RunID: "r1", CycleFloor: 0, PID: waveOnePID, StartedAt: waveStart, EndedAt: &end}
	if err := wave.NewStore(evolveDir).Save(rec); err != nil {
		t.Fatal(err)
	}
	var signals, calls strings.Builder
	for _, p := range phases {
		fmt.Fprintf(&signals, `{"ts":%q,"kind":"phase.dispatched","phase":%q}`+"\n", at(p.from), p.name)
		fmt.Fprintf(&signals, `{"ts":%q,"kind":"phase.outcome","phase":%q,"fields":{"verdict":"PASS"}}`+"\n", at(p.to), p.name)
		fmt.Fprintf(&calls, `{"phase":%q,"cli":"claude-tmux","started_at":%q,"ended_at":%q}`+"\n", p.name, at(p.modelFrom), at(p.modelTo))
	}
	for _, w := range waits {
		fmt.Fprintf(&signals, `{"ts":%q,"kind":"loop.warning","code":"HOST_VERIFY_LOCK_WAIT","fields":{"outcome":"acquired","source":"build-floor","wait_ms":%q}}`+"\n", at(w.minute), w.waitMS)
	}
	for name, body := range map[string]string{"signals.ndjson": signals.String(), "llm-calls.ndjson": calls.String()} {
		if err := os.WriteFile(filepath.Join(runDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func runEvolve(t *testing.T, root string, args ...string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "evolve")
	if _, stderr, code, err := acsassert.SubprocessOutput("go", "build", "-o", bin, evolveCmdPkg); err != nil || code != 0 {
		t.Fatalf("RED: the evolve binary did not build: exit %d (%v)\n%s", code, err, acsassert.OutputTail(stderr, 30))
	}
	full := append(append([]string{}, args...), "--json", "--project-root", root)
	stdout, stderr, code, err := acsassert.SubprocessOutput(bin, full...)
	if err != nil || code != 0 {
		t.Fatalf("RED: evolve %s exit %d (%v)\nstdout: %s\nstderr: %s",
			strings.Join(args, " "), code, err, acsassert.OutputTail(stdout, 20), acsassert.OutputTail(stderr, 20))
	}
	return stdout
}

type phaseRow struct {
	Phase    string `json:"phase"`
	Time     int64  `json:"time"`
	Host     int64  `json:"host"`
	LockWait *int64 `json:"lock_wait"`
}

func latencyPhases(t *testing.T, root string) map[string]phaseRow {
	t.Helper()
	var report struct {
		Phases struct {
			Phases []phaseRow `json:"phases"`
		} `json:"phases"`
	}
	out := runEvolve(t, root, "wave", "latency", "--wave", "1")
	if err := json.Unmarshal([]byte(out), &report); err != nil || len(report.Phases.Phases) == 0 {
		t.Fatalf("RED: wave latency --json printed no phase rows (%v):\n%s", err, acsassert.OutputTail(out, 20))
	}
	rows := map[string]phaseRow{}
	for _, r := range report.Phases.Phases {
		rows[r.Phase] = r
	}
	return rows
}

func (r phaseRow) lockWait(t *testing.T) int64 {
	t.Helper()
	if r.LockWait == nil {
		t.Fatalf("RED: the %s row of wave latency --json has no lock_wait field: host time that queued at the verification lock is not told apart from gate work", r.Phase)
	}
	return *r.LockWait
}

type waveVerdict struct {
	Wave          int      `json:"wave"`
	Status        string   `json:"status"`
	Reasons       []string `json:"reasons"`
	GateShare     float64  `json:"gate_share"`
	LockWaitShare *float64 `json:"lock_wait_share"`
}

func checkedWaveOne(t *testing.T, root string) waveVerdict {
	t.Helper()
	var report struct {
		Waves []waveVerdict `json:"waves"`
	}
	out := runEvolve(t, root, "wave", "latency", "--check")
	if err := json.Unmarshal([]byte(out), &report); err != nil || len(report.Waves) != 1 {
		t.Fatalf("RED: wave latency --check --json did not print one wave row (%v):\n%s", err, acsassert.OutputTail(out, 20))
	}
	return report.Waves[0]
}

func (v waveVerdict) lockShare(t *testing.T) float64 {
	t.Helper()
	if v.LockWaitShare == nil {
		t.Fatalf("RED: the verdict of wave %d has no lock_wait_share field: the check cannot say how much of the host share is queueing", v.Wave)
	}
	return *v.LockWaitShare
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestC2598_001_TheLatencyReportAttributesEachLockWaitToThePhaseThatContainsIt(t *testing.T) {
	root := replayWave(t,
		[]phaseRun{buildOnly(40), {name: "audit", from: 100, to: 140, modelFrom: 100, modelTo: 110}},
		[]queueWait{{minute: 90, waitMS: "2700000"}, {minute: 130, waitMS: "600000"}})

	rows := latencyPhases(t, root)

	build, audit := rows["build"], rows["audit"]
	if build.Host != 60*minute || audit.Host != 30*minute {
		t.Errorf("RED: host time moved: build %d ns, audit %d ns, want 60m and 30m (lock wait is a part of host time, not a replacement for it)", build.Host, audit.Host)
	}
	if got := build.lockWait(t); got != 45*minute {
		t.Errorf("RED: build lock_wait = %d ns, want 45m (the wait at minute 90 lies in the build span)", got)
	}
	if got := audit.lockWait(t); got != 10*minute {
		t.Errorf("RED: audit lock_wait = %d ns, want 10m (the wait at minute 130 lies in the audit span)", got)
	}
}

func TestC2598_002_AWaveWhoseHostTimeIsQueueingAtTheLockIsNotAGateBreach(t *testing.T) {
	root := replayWave(t, []phaseRun{buildOnly(40)}, []queueWait{{minute: 90, waitMS: "2700000"}})

	v := checkedWaveOne(t, root)

	if v.Status != "ok" {
		t.Errorf("RED: wave status = %q (%v), want ok: 60%% of the span is host time, but 45 of those 60 minutes queued at the host verification lock, so gate work is 15%%", v.Status, v.Reasons)
	}
	if !near(v.GateShare, 0.15) {
		t.Errorf("RED: gate_share = %.4f, want 0.15 (host 60m less lock wait 45m, over a 100m span)", v.GateShare)
	}
	if got := v.lockShare(t); !near(got, 0.45) {
		t.Errorf("RED: lock_wait_share = %.4f, want 0.45", got)
	}
}

func TestC2598_003_GateWorkAboveTheCeilingStillBreachesAndTheReasonNamesTheQueueing(t *testing.T) {
	root := replayWave(t, []phaseRun{buildOnly(30)}, []queueWait{{minute: 90, waitMS: "1200000"}})

	v := checkedWaveOne(t, root)

	if v.Status != "breach" {
		t.Errorf("RED: wave status = %q, want breach: 70m of host time less 20m of queueing leaves 50%% gate work against a 25%% ceiling", v.Status)
	}
	if !near(v.GateShare, 0.50) {
		t.Errorf("RED: gate_share = %.4f, want 0.50 (host 70m less lock wait 20m, over a 100m span)", v.GateShare)
	}
	if !strings.Contains(strings.ToLower(strings.Join(v.Reasons, " ")), "lock") {
		t.Errorf("RED: reasons %v do not name the share that queued at the verification lock", v.Reasons)
	}
	if got := v.lockShare(t); !near(got, 0.20) {
		t.Errorf("RED: lock_wait_share = %.4f, want 0.20", got)
	}
}

func TestC2598_004_AWaitOutsideEverySpanAndAMalformedWaitMoveNothing(t *testing.T) {
	root := replayWave(t, []phaseRun{buildOnly(40)},
		[]queueWait{{minute: -30, waitMS: "2700000"}, {minute: 50, waitMS: "not-a-number"}, {minute: 60, waitMS: "-900000"}})

	build := latencyPhases(t, root)["build"]

	if got := build.lockWait(t); got != 0 {
		t.Errorf("RED: build lock_wait = %d ns, want 0: a wait outside the span, a malformed wait_ms and a negative wait_ms must not count", got)
	}
	if build.Host != 60*minute {
		t.Errorf("RED: build host = %d ns, want 60m: a bad wait signal must not change the host time", build.Host)
	}
}

func TestC2598_005_LockWaitNeverExceedsTheHostTimeOfItsPhase(t *testing.T) {
	root := replayWave(t, []phaseRun{buildOnly(40)}, []queueWait{{minute: 90, waitMS: "10800000"}})

	build := latencyPhases(t, root)["build"]
	v := checkedWaveOne(t, root)

	if got := build.lockWait(t); got != build.Host {
		t.Errorf("RED: build lock_wait = %d ns, want the whole host time %d ns: a wait of 3h cannot lie in a span with 60m of host time", got, build.Host)
	}
	if v.GateShare < 0 || !near(v.GateShare, 0) {
		t.Errorf("RED: gate_share = %.4f, want 0 and never negative", v.GateShare)
	}
	if got := v.lockShare(t); !near(got, 0.60) {
		t.Errorf("RED: lock_wait_share = %.4f, want 0.60 (the clamped wait over the 100m span)", got)
	}
}
