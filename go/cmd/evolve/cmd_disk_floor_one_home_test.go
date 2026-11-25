package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

const (
	floorFreeBytes  = 40e9
	floorTotalBytes = 100e9
)

type floorReading struct {
	breached bool
	warnings string
}

type floorReader struct {
	name string
	read func(t *testing.T, policyBody string) floorReading
}

func writeFloorPolicyBody(t *testing.T, body string) (root, evolveDir string) {
	t.Helper()
	root = t.TempDir()
	evolveDir = filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, evolveDir
}

func warnLineNaming(output string, keys ...string) bool {
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, "WARN") {
			continue
		}
		all := true
		for _, k := range keys {
			all = all && strings.Contains(line, k)
		}
		if all {
			return true
		}
	}
	return false
}

func readFloorThroughGC(t *testing.T, policyBody string) (code int, out string) {
	t.Helper()
	root, _ := writeFloorPolicyBody(t, policyBody)
	withDiskFree(t, floorFreeBytes, nil)
	var stdout, stderr bytes.Buffer
	code = runGC([]string{"--dry-run", "--only", "cache", "--project-root", root}, nil, &stdout, &stderr)
	return code, stdout.String() + stderr.String()
}

func readFloorThroughLoopPreflight(t *testing.T, policyBody string) (disk looppreflight.CheckResult, stderr string) {
	t.Helper()
	root, evolveDir := writeFloorPolicyBody(t, policyBody)
	var warn bytes.Buffer
	opts := loopPreflightOptions(loopConfig{ProjectRoot: root, EvolveDir: evolveDir, SkipPreflightBoot: true}, &warn)
	opts.DiskFreeBytes = func(string) (uint64, error) { return floorFreeBytes, nil }
	res, err := looppreflight.Run(opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range res.Checks {
		if c.Name == "disk-space" {
			return c, warn.String()
		}
	}
	t.Fatal("the boot preflight ran no disk-space check")
	return looppreflight.CheckResult{}, ""
}

func readFloorThroughCycleStart(t *testing.T, policyBody string) (paused bool, warnings string, err error) {
	t.Helper()
	root, evolveDir := writeFloorPolicyBody(t, policyBody)
	var warn bytes.Buffer
	usage := func(string) (looppreflight.DiskUsage, error) {
		return looppreflight.DiskUsage{FreeBytes: floorFreeBytes, TotalBytes: floorTotalBytes}, nil
	}
	opts, err := buildCycleStartOptions(root, evolveDir, &warn, cycleStartHooks{usage: usage})
	if err != nil {
		return false, warn.String(), err
	}
	return opts.Guard.Measure().Paused, warn.String(), nil
}

func floorReaders() []floorReader {
	return []floorReader{
		{"evolve gc", func(t *testing.T, body string) floorReading {
			code, out := readFloorThroughGC(t, body)
			if code != 0 {
				t.Fatalf("evolve gc exit %d:\n%s", code, out)
			}
			return floorReading{breached: strings.Contains(out, "is under"), warnings: out}
		}},
		{"evolve loop preflight", func(t *testing.T, body string) floorReading {
			disk, warn := readFloorThroughLoopPreflight(t, body)
			return floorReading{breached: disk.Level == looppreflight.LevelHalt, warnings: warn + disk.Message}
		}},
		{"evolve loop cycle start", func(t *testing.T, body string) floorReading {
			paused, warn, err := readFloorThroughCycleStart(t, body)
			if err != nil {
				t.Fatalf("the cycle start refused the policy: %v", err)
			}
			return floorReading{breached: paused, warnings: warn}
		}},
	}
}

func TestDiskFloorOneHome_NewKeyAloneIsOneFloorForEveryReader(t *testing.T) {
	cases := []struct {
		name     string
		policy   string
		breached bool
	}{
		{"free disk under the floor", `{"disk":{"floor_gb":50,"warn_gb":50}}`, true},
		{"free disk above the floor", `{"disk":{"floor_gb":30,"warn_gb":30}}`, false},
	}
	for _, c := range cases {
		for _, r := range floorReaders() {
			t.Run(c.name+"/"+r.name, func(t *testing.T) {
				if got := r.read(t, c.policy); got.breached != c.breached {
					t.Errorf("breached = %v, want %v for %s with 40 GB free\n%s", got.breached, c.breached, c.policy, got.warnings)
				}
			})
		}
	}
}

func TestDiskFloorOneHome_GCNamesTheNewKeyWhenTheFloorIsBreached(t *testing.T) {
	_, out := readFloorThroughGC(t, `{"disk":{"floor_gb":50,"warn_gb":50}}`)
	if !warnLineNaming(out, "is under", "disk.floor_gb") {
		t.Errorf("no WARN line names disk.floor_gb as the breached floor:\n%s", out)
	}
}

func TestDiskFloorOneHome_AnOldKeyStillSetsTheFloorAndWarnsNamingTheNewKey(t *testing.T) {
	cases := []struct {
		name   string
		policy string
		oldKey string
		newKey string
	}{
		{"gc floor", `{"gc":{"disk_floor_gb":50,"disk_warn_gb":50}}`, "gc.disk_floor_gb", "disk.floor_gb"},
		{"gc warn", `{"gc":{"disk_floor_gb":50,"disk_warn_gb":60}}`, "gc.disk_warn_gb", "disk.warn_gb"},
		{"preflight floor", `{"preflight":{"min_free_gib":50}}`, "preflight.min_free_gib", "disk.floor_gb"},
	}
	for _, c := range cases {
		for _, r := range floorReaders() {
			t.Run(c.name+"/"+r.name, func(t *testing.T) {
				got := r.read(t, c.policy)
				if !got.breached {
					t.Errorf("the old key %s no longer sets the floor: 40 GB free is not under it\n%s", c.oldKey, got.warnings)
				}
				if !warnLineNaming(got.warnings, c.oldKey, c.newKey) {
					t.Errorf("no WARN line names the old key %s and the new key %s:\n%s", c.oldKey, c.newKey, got.warnings)
				}
			})
		}
	}
}

func TestDiskFloorOneHome_NoWarnWhenOnlyTheNewKeyIsSet(t *testing.T) {
	for _, r := range floorReaders() {
		t.Run(r.name, func(t *testing.T) {
			got := r.read(t, `{"disk":{"floor_gb":30,"warn_gb":30}}`)
			for _, old := range []string{"gc.disk_floor_gb", "gc.disk_warn_gb", "preflight.min_free_gib"} {
				if strings.Contains(got.warnings, old) {
					t.Errorf("the output names the old key %s though only the new keys are set:\n%s", old, got.warnings)
				}
			}
		})
	}
}

var conflictCases = []struct {
	name   string
	policy string
	oldKey string
	newKey string
}{
	{"gc floor", `{"gc":{"disk_floor_gb":5},"disk":{"floor_gb":8}}`, "gc.disk_floor_gb", "disk.floor_gb"},
	{"preflight floor", `{"preflight":{"min_free_gib":2},"disk":{"floor_gb":8}}`, "preflight.min_free_gib", "disk.floor_gb"},
	{"gc warn", `{"gc":{"disk_warn_gb":70},"disk":{"warn_gb":80}}`, "gc.disk_warn_gb", "disk.warn_gb"},
}

func TestDiskFloorOneHome_ConflictingOldAndNewValuesAreRefusedByGC(t *testing.T) {
	for _, c := range conflictCases {
		t.Run(c.name, func(t *testing.T) {
			code, out := readFloorThroughGC(t, c.policy)
			if code == 0 {
				t.Errorf("evolve gc ran with conflicting keys:\n%s", out)
			}
			if !strings.Contains(out, c.oldKey) || !strings.Contains(out, c.newKey) {
				t.Errorf("the refusal does not name both %s and %s:\n%s", c.oldKey, c.newKey, out)
			}
		})
	}
}

func TestDiskFloorOneHome_ConflictingOldAndNewValuesHaltTheLoopPreflight(t *testing.T) {
	t.Parallel()
	for _, c := range conflictCases {
		t.Run(c.name, func(t *testing.T) {
			disk, stderr := readFloorThroughLoopPreflight(t, c.policy)
			if disk.Level != looppreflight.LevelHalt {
				t.Errorf("the disk-space check is %s, want halt, for conflicting keys", disk.Level)
			}
			out := stderr + disk.Message + disk.Detail
			if !strings.Contains(out, c.oldKey) || !strings.Contains(out, c.newKey) {
				t.Errorf("the refusal does not name both %s and %s:\n%s", c.oldKey, c.newKey, out)
			}
		})
	}
}

func TestDiskFloorOneHome_ConflictingOldAndNewValuesRefuseTheCycleStart(t *testing.T) {
	t.Parallel()
	if _, _, err := readFloorThroughCycleStart(t, `{"disk":{"floor_gb":8}}`); err != nil {
		t.Fatalf("the cycle start refused the new key alone: %v", err)
	}
	for _, c := range conflictCases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := readFloorThroughCycleStart(t, c.policy)
			if !errors.Is(err, errCycleStartPolicyRefused) {
				t.Errorf("buildCycleStartOptions error = %v, want errCycleStartPolicyRefused", err)
			}
		})
	}
}

func TestDiskFloorOneHome_EqualOldAndNewValuesAreNotAConflict(t *testing.T) {
	const body = `{"gc":{"disk_floor_gb":30},"disk":{"floor_gb":30}}`
	code, out := readFloorThroughGC(t, body)
	if code != 0 {
		t.Errorf("evolve gc refused equal old and new values (exit %d):\n%s", code, out)
	}
	if _, _, err := readFloorThroughCycleStart(t, body); err != nil {
		t.Errorf("the cycle start refused equal old and new values: %v", err)
	}
}

func lowFloorUsage(string) (looppreflight.DiskUsage, error) {
	return looppreflight.DiskUsage{FreeBytes: floorFreeBytes, TotalBytes: floorTotalBytes}, nil
}

func TestDiskFloorOneHome_TheSequentialLoopStopsBeforeACycleUnderTheNewFloor(t *testing.T) {
	t.Parallel()
	root, evolveDir := writeFloorPolicyBody(t, `{"disk":{"floor_gb":50,"warn_gb":50},"preflight":{"disk_guard":{"max_pause_s":1}}}`)
	var stderr bytes.Buffer
	b := &loopBatchCoordinator{
		ctx:    context.Background(),
		cfg:    loopConfig{ProjectRoot: root, EvolveDir: evolveDir},
		orch:   cycleStartSeqRunner{events: new([]string)},
		deps:   orchDeps{Storage: &fixtures.FakeStorage{}, Signals: signalcenter.New()},
		result: &loopResult{},
		stdout: &bytes.Buffer{},
		stderr: &stderr,
		startHooks: cycleStartHooks{
			usage: lowFloorUsage,
			wait:  func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() },
		},
	}
	_, decision := b.dispatchSequentialCycle()
	if decision.flow != batchStopIterations || b.result.StopReason != hostDiskLowStopReason {
		t.Errorf("decision = %+v, stop reason = %q; want the batch stopped with %q at 40%% free under a 50 GB floor", decision, b.result.StopReason, hostDiskLowStopReason)
	}
}

func TestDiskFloorOneHome_AFleetLaneStopsBeforeACycleUnderTheNewFloor(t *testing.T) {
	t.Parallel()
	root, _ := writeFloorPolicyBody(t, `{"disk":{"floor_gb":50,"warn_gb":50},"preflight":{"disk_guard":{"max_pause_s":1}}}`)
	launch := cycleLaunch{
		Bin: "unused", Root: root, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
		Signals:    signalcenter.New(),
		startHooks: cycleStartHooks{usage: lowFloorUsage, wait: func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }},
	}
	code, err := launch.LaunchFn()(context.Background(), fleet.CycleSpec{GoalHash: "deadbeef"})
	if code != -1 || !errors.Is(err, looppreflight.ErrDiskPauseExceeded) {
		t.Errorf("LaunchFn = (%d, %v), want (-1, ErrDiskPauseExceeded) at 40%% free under a 50 GB floor", code, err)
	}
}

func TestDiskFloorOneHome_TheWaveHaltReadsTheNewKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		halt bool
	}{
		{"floor above the real free disk", `{"disk":{"floor_gb":100000000}}`, true},
		{"floor far below the real free disk", `{"disk":{"floor_gb":0.001}}`, false},
		{"conflicting old and new floor keys", `{"gc":{"disk_floor_gb":5},"disk":{"floor_gb":8}}`, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b, evolveDir := starvationCoordinator(t)
			if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(c.body), 0o644); err != nil {
				t.Fatal(err)
			}
			b.deps.Signals = signalcenter.New()
			if _, halt := b.diskSpaceHalt(0); halt != c.halt {
				t.Errorf("diskSpaceHalt = %v, want %v for %s", halt, c.halt, c.body)
			}
		})
	}
}

func TestDiskFloorOneHome_WaveNextRefusesUnderTheNewKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want []string
	}{
		{"new key below free disk", `{"disk":{"floor_gb":2}}`, []string{"disk.floor_gb"}},
		{"conflicting old and new floor keys", `{"gc":{"disk_floor_gb":5},"disk":{"floor_gb":8}}`, []string{"gc.disk_floor_gb", "disk.floor_gb"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := newWavePlane(t)
			p.write(".evolve/policy.json", c.body)
			h := newWaveHarness()
			h.diskFree = func(string) (uint64, error) { return 273 << 20, nil }
			rc := h.run("next", "--json", "--project-root", p.root)
			e := decodeWaveEnvelope(t, h.stdout.Bytes())
			if rc != cliExitRefused || e.Refused != waveRefusedDiskLow || e.Launched {
				t.Fatalf("rc = %d envelope = %+v; want a %s refusal and no launch", rc, e, waveRefusedDiskLow)
			}
			for _, want := range c.want {
				if !strings.Contains(e.Error, want) {
					t.Errorf("the refusal %q does not name %q", e.Error, want)
				}
			}
		})
	}
}
