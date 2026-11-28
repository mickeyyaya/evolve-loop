//go:build acs

package cycle2676

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/hostpressure"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	evolveCmdPkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
	waveOnePID   = 7001
)

var (
	evolveBinary string
	waveStart    = time.Date(2026, 11, 21, 18, 0, 0, 0, time.UTC)
	ruleStart    = time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "cycle2676-evolve-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot make the build dir:", err)
		os.Exit(2)
	}
	evolveBinary = filepath.Join(dir, "evolve")
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func windowRule(bound time.Duration) hostpressure.Config {
	return hostpressure.Config{
		LoadHigh: 1.5, LoadLow: 1.0, MemMinPct: 15, SwapMaxMB: 4096, DecreaseFactor: 0.5, Step: 1,
		SmoothSamples: 1, Floor: 4, Ceiling: 12, Cooldown: 60 * time.Second, IncreaseCooldown: 180 * time.Second,
		LockWaitMax: bound,
	}
}

func calmHost(lockWait time.Duration) hostpressure.Reading {
	return hostpressure.Reading{Load1: 7, CPUs: 14, MemAvailablePct: 80, LockWait: lockWait}
}

func deadBandHost(lockWait time.Duration) hostpressure.Reading {
	return hostpressure.Reading{Load1: 16.8, CPUs: 14, MemAvailablePct: 80, LockWait: lockWait}
}

func heavyHost(lockWait time.Duration) hostpressure.Reading {
	return hostpressure.Reading{Load1: 28, CPUs: 14, MemAvailablePct: 80, LockWait: lockWait}
}

func narrowedAt(cap int) hostpressure.State { return hostpressure.State{Cap: cap} }

func TestC2676_001_LockWaitOverTheBoundGivesNoWidening(t *testing.T) {
	cfg := windowRule(10 * time.Minute)
	state := narrowedAt(cfg.Floor)
	now := ruleStart
	for i := range 6 {
		var d hostpressure.Decision
		state, d = hostpressure.Decide(state, calmHost(701*time.Second), true, cfg, now)
		if d.Old != cfg.Floor || d.New != cfg.Floor || state.Cap != cfg.Floor {
			t.Fatalf("RED: sample %d: cap %d -> %d (state %d), want the cap held at %d: the last window waited 701s at the lock against a bound of 600s", i, d.Old, d.New, state.Cap, cfg.Floor)
		}
		if d.Cause != hostpressure.CauseLockWaitHigh || !d.LockWaitHeld {
			t.Errorf("RED: sample %d: cause %q held=%v, want %q and held=true: the record must say that the lock wait stopped a widening", i, d.Cause, d.LockWaitHeld, hostpressure.CauseLockWaitHigh)
		}
		now = now.Add(cfg.IncreaseCooldown)
	}
	state, d := hostpressure.Decide(state, calmHost(0), true, cfg, now)
	if d.New != cfg.Floor+1 || d.Cause != hostpressure.CausePressureLow || d.LockWaitHeld {
		t.Errorf("RED: after the wait cleared the cap went %d -> %d cause %q held=%v, want the first widening at once: %d, pressure_low, held=false (a held widening must not restart the cooldown)", d.Old, d.New, d.Cause, d.LockWaitHeld, cfg.Floor+1)
	}
}

func TestC2676_002_LockWaitUnderTheBoundGivesTheOldResult(t *testing.T) {
	cfg := windowRule(10 * time.Minute)
	baseline := func(reading hostpressure.Reading, c hostpressure.Config, cap int) hostpressure.Decision {
		_, d := hostpressure.Decide(narrowedAt(cap), reading, true, c, ruleStart)
		return d
	}
	zero := baseline(calmHost(0), cfg, cfg.Floor)
	if zero.New != cfg.Floor+1 || zero.Cause != hostpressure.CausePressureLow {
		t.Fatalf("RED: no lock wait gave cap %d -> %d cause %q, want %d -> %d pressure_low", zero.Old, zero.New, zero.Cause, cfg.Floor, cfg.Floor+1)
	}
	for _, wait := range []time.Duration{time.Second, 599 * time.Second, 600 * time.Second} {
		if got := baseline(calmHost(wait), cfg, cfg.Floor); got != zero {
			t.Errorf("RED: lock wait %s (bound 600s) gave %+v, want the old result %+v: only a wait over the bound may stop a widening", wait, got, zero)
		}
	}
	if got := baseline(calmHost(time.Hour), windowRule(0), cfg.Floor); got != zero {
		t.Errorf("RED: a bound of 0 with a lock wait of 1h gave %+v, want the old result %+v: a bound of 0 switches the rule off", got, zero)
	}
	if d := baseline(heavyHost(time.Hour), cfg, cfg.Ceiling); d.New != cfg.Ceiling/2 || d.Cause != hostpressure.CausePressureHigh || d.LockWaitHeld {
		t.Errorf("RED: heavy load with a long lock wait gave %+v, want the halving of the old rule: the lock wait must never stop a decrease", d)
	}
	if d := baseline(deadBandHost(time.Hour), cfg, cfg.Floor); d.New != cfg.Floor || d.Cause != hostpressure.CauseHold || d.LockWaitHeld {
		t.Errorf("RED: a load inside the dead band with a long lock wait gave %+v, want hold and held=false: no widening was due", d)
	}
	if d := baseline(calmHost(time.Hour), cfg, cfg.Ceiling); d.New != cfg.Ceiling || d.Cause != hostpressure.CauseHold || d.LockWaitHeld {
		t.Errorf("RED: a cap at the ceiling with a long lock wait gave %+v, want hold and held=false: no widening was due", d)
	}
	_, inCooldown := hostpressure.Decide(hostpressure.State{Cap: cfg.Floor, Changed: ruleStart}, calmHost(time.Hour), true, cfg, ruleStart.Add(cfg.IncreaseCooldown-time.Second))
	if inCooldown.Cause != hostpressure.CauseHold || inCooldown.LockWaitHeld {
		t.Errorf("RED: inside the increase cooldown with a long lock wait gave %+v, want hold and held=false: the cooldown stopped the widening, not the wait", inCooldown)
	}
}

func TestC2676_003_TheBoundIsPolicyDataWithADefaultAndRefusesANegativeValue(t *testing.T) {
	adaptive := func(block string) (policy.AdaptiveConfig, error) {
		raw := `{"fleet":{"count":12,"orchestrator":{"stage":"enforce","window":{"start":8,"adaptive":` + block + `}}}}`
		p, err := policy.Parse([]byte(raw))
		if err != nil {
			return policy.AdaptiveConfig{}, err
		}
		return p.FleetConfig().Orchestrator.Adaptive, nil
	}
	set, err := adaptive(`{"lock_wait_max_s":420}`)
	if err != nil || set.LockWaitMaxS != 420 {
		t.Errorf("RED: lock_wait_max_s 420 resolved to %d (err %v), want 420", set.LockWaitMaxS, err)
	}
	omitted, err := adaptive(`{"stage":"shadow"}`)
	if err != nil || omitted.LockWaitMaxS <= 0 {
		t.Errorf("RED: an omitted lock_wait_max_s resolved to %d (err %v), want a default above 0: the bound is in policy data, and the policy file is a protected path that this change cannot edit", omitted.LockWaitMaxS, err)
	}
	if _, err := adaptive(`{"lock_wait_max_s":-1}`); err == nil {
		t.Errorf("RED: a negative lock_wait_max_s was accepted, want the strict policy parse to refuse it")
	}
}

func goTestPasses(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	stdout, stderr, code, err := acsassert.SubprocessOutputIn(goDir, "go", "test", "-count=1", "-v", "-run", pattern, pkg)
	if err != nil || code != 0 {
		t.Fatalf("RED: go test -run %s %s exit %d (%v)\n%s\n%s", pattern, pkg, code, err, acsassert.OutputTail(stdout, 40), acsassert.OutputTail(stderr, 40))
	}
	for _, name := range names {
		if !strings.Contains(stdout, "--- PASS: "+name+" ") {
			t.Errorf("RED: %s did not run and pass in %s:\n%s", name, pkg, acsassert.OutputTail(stdout, 40))
		}
	}
}

func TestC2676_004_TheShadowRecordAndTheProductionCallerCarryTheLockWaitToTheWindowRule(t *testing.T) {
	goTestPasses(t, "./cmd/evolve",
		"TestHostPressureLockWait_ShadowLineNamesAWidenThatTheLockWaitHeld",
		"TestHostPressureLockWait_EnforceHoldsTheCapWhileTheWaitIsOverTheBound",
		"TestHostPressureLockWait_HostConfigCarriesThePolicyBound",
		"TestStartHostPressure_ReadsTheLockWaitFromTheRunSignalLogsOfTheFleet",
		"TestStartHostPressure_AWaitOutsideTheLastWindowOrUnderTheBoundDoesNotHoldTheWidening",
	)
}

func buildEvolve(t *testing.T) string {
	t.Helper()
	if _, err := os.Stat(evolveBinary); err == nil {
		return evolveBinary
	}
	if _, stderr, code, err := acsassert.SubprocessOutput("go", "build", "-o", evolveBinary, evolveCmdPkg); err != nil || code != 0 {
		t.Fatalf("RED: the evolve binary did not build: exit %d (%v)\n%s", code, err, acsassert.OutputTail(stderr, 30))
	}
	return evolveBinary
}

type lockWaitSignal struct {
	pid, seq int
	cycle    int
	minute   int
	waitMS   string
	outcome  string
}

func (s lockWaitSignal) line() string {
	ts := waveStart.Add(time.Duration(s.minute) * time.Minute).Format(time.RFC3339Nano)
	return fmt.Sprintf(`{"schema_version":"signal/1.0","seq":%d,"pid":%d,"ts":%q,"cycle":%d,"module":"host","origin":"verifylock.AcquireWithin","kind":"loop.warning","code":"HOST_VERIFY_LOCK_WAIT","severity":"INFO","reason":"waited at the host verification lock","fields":{"outcome":%q,"source":"build-floor","wait_ms":%q}}`+"\n",
		s.seq, s.pid, ts, s.cycle, s.outcome, s.waitMS)
}

func acquired(pid, seq, cycle, minute int, waitMS string) lockWaitSignal {
	return lockWaitSignal{pid: pid, seq: seq, cycle: cycle, minute: minute, waitMS: waitMS, outcome: "acquired"}
}

type waveFixture struct {
	perRun map[int][]lockWaitSignal
	global []lockWaitSignal
	floor  int
}

func (f waveFixture) write(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	var global strings.Builder
	for _, s := range f.global {
		global.WriteString(s.line())
	}
	files := map[string]string{
		filepath.Join(evolveDir, "state.json"):     `{"lastCycleNumber":9}`,
		filepath.Join(evolveDir, "signals.ndjson"): global.String(),
	}
	for cycle, signals := range f.perRun {
		var body strings.Builder
		for _, s := range signals {
			body.WriteString(s.line())
		}
		files[filepath.Join(evolveDir, "runs", fmt.Sprintf("cycle-%d", cycle), "signals.ndjson")] = body.String()
	}
	for path, body := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	end := waveStart.Add(4 * time.Hour)
	rec := wave.Record{Number: 1, RunID: "r1", CycleFloor: f.floor, PID: waveOnePID, StartedAt: waveStart, EndedAt: &end}
	if err := wave.NewStore(evolveDir).Save(rec); err != nil {
		t.Fatal(err)
	}
	return root
}

func waveStatus(t *testing.T, root string, asJSON bool) string {
	t.Helper()
	args := []string{"wave", "status", "--project-root", root}
	if asJSON {
		args = append(args, "--json")
	}
	stdout, stderr, code, err := acsassert.SubprocessOutput(buildEvolve(t), args...)
	if err != nil || code != 0 {
		t.Fatalf("RED: evolve %s exit %d (%v)\nstdout: %s\nstderr: %s", strings.Join(args, " "), code, err, acsassert.OutputTail(stdout, 20), acsassert.OutputTail(stderr, 20))
	}
	return stdout
}

func statusLockWaitMS(t *testing.T, root string) int64 {
	t.Helper()
	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			LockWaitMS *int64 `json:"lock_wait_ms"`
		} `json:"data"`
	}
	out := waveStatus(t, root, true)
	if err := json.Unmarshal([]byte(out), &envelope); err != nil || !envelope.OK {
		t.Fatalf("RED: wave status --json is not an ok envelope (%v):\n%s", err, acsassert.OutputTail(out, 20))
	}
	if envelope.Data.LockWaitMS == nil {
		t.Fatalf("RED: wave status --json has no data.lock_wait_ms field: the lock wait of the wave is not shown")
	}
	return *envelope.Data.LockWaitMS
}

func TestC2676_005_WaveStatusShowsTheTotalLockWaitOfTheWaveFromTheRunSignalLogs(t *testing.T) {
	root := waveFixture{perRun: map[int][]lockWaitSignal{
		2593: {acquired(53475, 279, 2593, 30, "280000"), {pid: 53475, seq: 280, cycle: 2593, minute: 60, waitMS: "900000", outcome: "unserialized"}},
		2595: {acquired(53479, 249, 2595, 45, "701000")},
	}}.write(t)

	if got, want := statusLockWaitMS(t, root), int64(280000+900000+701000); got != want {
		t.Errorf("RED: wave status lock_wait_ms = %d, want %d: the waits of wave 186 (280 s, 900 s unserialized, 701 s) live only in runs/cycle-N/signals.ndjson", got, want)
	}
	text := strings.ToLower(waveStatus(t, root, false))
	if !strings.Contains(text, "lock wait") || !strings.Contains(text, "31m21s") {
		t.Errorf("RED: wave status text does not show the lock wait of the wave as \"lock wait\" and 31m21s:\n%s", acsassert.OutputTail(text, 30))
	}
}

func TestC2676_006_ABadWaitAnOldCycleAndAWaveWithoutWaitsMoveNothing(t *testing.T) {
	root := waveFixture{floor: 1, perRun: map[int][]lockWaitSignal{
		1: {acquired(53470, 11, 1, 10, "500000")},
		2: {acquired(53475, 12, 2, 20, "120000"), acquired(53475, 13, 2, 21, "not-a-number"), acquired(53475, 14, 2, 22, "-900000"), acquired(53475, 15, 2, 23, "0")},
	}}.write(t)
	if got := statusLockWaitMS(t, root); got != 120000 {
		t.Errorf("RED: wave status lock_wait_ms = %d, want 120000: a cycle at or under the wave floor, a malformed wait, a negative wait and a zero wait must not count", got)
	}

	none := waveFixture{perRun: map[int][]lockWaitSignal{2: {}}}.write(t)
	if got := statusLockWaitMS(t, none); got != 0 {
		t.Errorf("RED: wave status lock_wait_ms = %d for a wave without waits, want 0 shown (the field is present, not absent)", got)
	}
}

func TestC2676_007_OneWaitInBothLogsCountsOnceAndAWaitOnlyInTheGlobalLogCounts(t *testing.T) {
	root := waveFixture{
		perRun: map[int][]lockWaitSignal{2: {acquired(53475, 279, 2, 30, "280000")}},
		global: []lockWaitSignal{acquired(53475, 279, 2, 30, "280000"), acquired(53479, 90, 3, 50, "701000")},
	}.write(t)

	if got, want := statusLockWaitMS(t, root), int64(280000+701000); got != want {
		t.Errorf("RED: wave status lock_wait_ms = %d, want %d: the wait of pid 53475 seq 279 is in both logs and counts once, and the wait of pid 53479 is only in the global log and counts", got, want)
	}
}
