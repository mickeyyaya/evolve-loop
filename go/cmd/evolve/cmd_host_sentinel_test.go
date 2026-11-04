package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/hostsentinel"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func writeHostPolicy(t *testing.T, evolveDir, body string) {
	t.Helper()
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const hostEnforceOnALowDisk = `{"host_sentinel":{"stage":"enforce","disk_low_gib":1000000000,"floor_gib":0.000001,"tmux_sessions_max":100000,"orphans_max":100000,"load_per_cpu_max":100000},"gc":{"go_cache_ttl_hours":24,"go_cache_max_gb":1000,"temp_ttl_hours":0}}`

func TestHostSentinelJourney_AtEnforceALowDiskMidWaveTrimsTheStaleCacheSignalsAndSparesTheLiveLane(t *testing.T) {
	projectRoot, evolveDir, _ := gcWorktreeEnv(t, "")
	stale, fresh := gcFakeGoCache(t)
	writeHostPolicy(t, evolveDir, hostEnforceOnALowDisk)
	failOnSessionReap(t)
	tree := liveLane(t, projectRoot, 1900)

	var console bytes.Buffer
	signals := newRootSignalCenter(projectRoot, evolveDir, &console)
	host := newHostSentinel(loopConfig{ProjectRoot: projectRoot, EvolveDir: evolveDir}, signals, &console)
	if host == nil {
		t.Fatalf("an enforce policy built no sentinel: %s", console.String())
	}
	st := host.Tick(context.Background())
	signals.Flush()

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("the stale cache entry survived the enforce release: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("the release trimmed a fresh cache entry: %v", err)
	}
	if _, err := os.Stat(tree); err != nil {
		t.Errorf("the release touched the live lane's worktree: %v", err)
	}
	if !strings.Contains(console.String(), string(hostsentinel.CodeDiskLow)) || !strings.Contains(console.String(), "threshold=1073741824000000000") {
		t.Errorf("console does not carry HOST_DISK_LOW with its threshold:\n%s", console.String())
	}
	if len(st.Releases) != 1 || st.Releases[0].Scope != gcScopeCache || !st.Releases[0].Applied {
		t.Errorf("releases = %+v, want one applied cache scope", st.Releases)
	}

	p := wavePlane{t: t, root: projectRoot}
	p.seedMeasuredRuns()
	h := newWaveHarness()
	h.run("status", "--project-root", projectRoot)
	if out := h.stdout.String(); !strings.Contains(out, "host:    sampled ") || !strings.Contains(out, "HOST_DISK_LOW") {
		t.Errorf("wave status does not show the last sample:\n%s", out)
	}
	js := newWaveHarness()
	js.run("status", "--json", "--project-root", projectRoot)
	var got waveStatusReport
	if err := json.Unmarshal(js.stdout.Bytes(), &got); err != nil || got.Host == nil || got.Host.Stage != hostsentinel.StageEnforce {
		t.Errorf("wave status --json host = %+v, %v", got.Host, err)
	}
}

func TestHostSentinelJourney_AtShadowTheSameHostReleasesNothing(t *testing.T) {
	projectRoot, evolveDir, _ := gcWorktreeEnv(t, "")
	stale, _ := gcFakeGoCache(t)
	writeHostPolicy(t, evolveDir, strings.Replace(hostEnforceOnALowDisk, `"enforce"`, `"shadow"`, 1))
	failOnSessionReap(t)

	var console bytes.Buffer
	host := newHostSentinel(loopConfig{ProjectRoot: projectRoot, EvolveDir: evolveDir}, nil, &console)
	st := host.Tick(context.Background())
	if _, err := os.Stat(stale); err != nil {
		t.Fatalf("shadow removed a cache entry: %v", err)
	}
	if len(st.Releases) != 1 || st.Releases[0].Applied || !strings.Contains(strings.Join(st.Releases[0].Report, "\n"), "would be trimmed") {
		t.Fatalf("shadow releases = %+v, want the preview of the cache scope only", st.Releases)
	}
}

func TestHostSentinel_AGCLeaseHeldByTheBoundaryMakesTheReleaseSkipNotWait(t *testing.T) {
	projectRoot, evolveDir, _ := gcWorktreeEnv(t, "")
	stale, _ := gcFakeGoCache(t)
	writeHostPolicy(t, evolveDir, hostEnforceOnALowDisk)
	release, rc := takeGCLease(projectRoot, 0, io.Discard)
	if rc != cliExitOK {
		t.Fatalf("take the boundary lease: rc=%d", rc)
	}
	defer release()

	host := newHostSentinel(loopConfig{ProjectRoot: projectRoot, EvolveDir: evolveDir}, nil, io.Discard)
	start := time.Now()
	st := host.Tick(context.Background())
	if waited := time.Since(start); waited > 30*time.Second {
		t.Fatalf("the sentinel waited %s for the lease", waited)
	}
	if len(st.Releases) != 1 || !st.Releases[0].LeaseHeld || st.Releases[0].Applied {
		t.Fatalf("releases = %+v, want the cache scope skipped on the held lease", st.Releases)
	}
	if _, err := os.Stat(stale); err != nil {
		t.Fatalf("a skipped release removed a cache entry: %v", err)
	}
}

func TestNewHostSentinel_OffAndAnUnknownStageBuildNoSentinel(t *testing.T) {
	t.Parallel()
	for body, warn := range map[string]string{
		`{"host_sentinel":{"stage":"off"}}`:  "",
		`{"host_sentinel":{"stage":"loud"}}`: `"loud"`,
	} {
		evolveDir := filepath.Join(t.TempDir(), ".evolve")
		writeHostPolicy(t, evolveDir, body)
		var stderr bytes.Buffer
		cfg := loopConfig{ProjectRoot: filepath.Dir(evolveDir), EvolveDir: evolveDir}
		if s := newHostSentinel(cfg, nil, &stderr); s != nil {
			t.Errorf("%s built a sentinel", body)
		}
		if warn != "" && !strings.Contains(stderr.String(), warn) {
			t.Errorf("%s: stderr %q, want the bad stage named", body, stderr.String())
		}
		host, stop := startHostSentinel(context.Background(), cfg, nil, io.Discard)
		stop()
		if host != nil {
			t.Errorf("%s started a sentinel", body)
		}
	}
}

func TestStartHostSentinel_TheDefaultPolicyRunsAtShadowAndStopsWithTheLoop(t *testing.T) {
	t.Parallel()
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	writeHostPolicy(t, evolveDir, `{}`)
	ctx, cancel := context.WithCancel(context.Background())
	host, stop := startHostSentinel(ctx, loopConfig{ProjectRoot: filepath.Dir(evolveDir), EvolveDir: evolveDir}, nil, io.Discard)
	if host == nil || host.HoldLanes() {
		t.Fatal("the default policy must start a shadow sentinel that holds no lane")
	}
	cancel()
	done := make(chan struct{})
	go func() { stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the sentinel did not stop with the loop")
	}
}

func TestHostGatedLaunch_BelowTheFloorNoNewLaneStartsAndARunningLaneContinues(t *testing.T) {
	t.Parallel()
	c, err := hostsentinel.Policy{Stage: "enforce", FloorGiB: 5}.Config(1)
	if err != nil {
		t.Fatal(err)
	}
	disk := 100.0 * (1 << 30)
	host := hostsentinel.New(c, hostsentinel.Ports{
		Sample: func(context.Context) hostsentinel.Sample {
			return hostsentinel.Sample{At: time.Now(), Readings: map[hostsentinel.Metric]hostsentinel.Reading{hostsentinel.MetricDiskFree: {Value: disk}}}
		},
		Release: func(context.Context, []hostsentinel.Breach, bool) []hostsentinel.Release { return nil },
		Emit:    func(signalcenter.Event) {},
		Save:    func(hostsentinel.Status) error { return nil },
		Warn:    func(string) {},
	})
	started := make(chan struct{}, 4)
	finish := make(chan struct{})
	launch := hostGatedLaunch(host, func(ctx context.Context, spec fleet.CycleSpec) (int, error) {
		started <- struct{}{}
		<-finish
		return 0, nil
	})
	host.Tick(context.Background())
	running := make(chan int, 1)
	go func() {
		code, _ := launch(context.Background(), fleet.CycleSpec{})
		running <- code
	}()
	<-started
	disk = 1 << 30
	host.Tick(context.Background())
	code, err := launch(context.Background(), fleet.CycleSpec{})
	if code != fleet.ExitDeferred || err != nil {
		t.Fatalf("a launch below the floor = %d, %v; want deferred", code, err)
	}
	if len(started) != 0 {
		t.Fatal("a lane started below the floor")
	}
	close(finish)
	if got := <-running; got != 0 {
		t.Fatalf("the running lane ended with %d, want its own exit 0", got)
	}
}

func TestHostProbes_AFailedProbeIsAFailedReadingNotAZeroValue(t *testing.T) {
	t.Setenv("GOCACHE", "off")
	if _, err := probeGoCache(context.Background()); err == nil {
		t.Error("GOCACHE=off must fail the cache reading")
	}
	fail := func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		return 0, errors.New("no lsof")
	}
	if _, err := probeOrphans(gc.WorktreeOptions{ProjectRoot: t.TempDir(), Exec: sysexec.RunFunc(fail)})(context.Background()); err == nil {
		t.Error("a failed process list must fail the orphans reading")
	}
	if _, err := probeLoad(filepath.Join(t.TempDir(), "loadavg"))(context.Background()); err == nil {
		t.Error("a missing loadavg must fail the load reading")
	}
	path := filepath.Join(t.TempDir(), "loadavg")
	if err := os.WriteFile(path, []byte("0.00 0.00 0.00 1/1 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if v, err := probeLoad(path)(context.Background()); err != nil || v != 0 {
		t.Errorf("probeLoad = %v, %v; want 0, nil", v, err)
	}
	gcDir := t.TempDir()
	t.Setenv("GOCACHE", gcDir)
	if v, err := probeGoCache(context.Background()); err != nil || v != 0 {
		t.Errorf("an empty cache = %v, %v; want 0, nil", v, err)
	}
}

func TestHostScopes_EachReleasableBreachNamesItsScopeOnce(t *testing.T) {
	t.Parallel()
	got := hostScopes([]hostsentinel.Breach{
		{Metric: hostsentinel.MetricDiskFree}, {Metric: hostsentinel.MetricTmux},
		{Metric: hostsentinel.MetricGoCache}, {Metric: hostsentinel.MetricOrphans}, {Metric: hostsentinel.MetricLoad},
	})
	if strings.Join(got, ",") != "cache,orphans" {
		t.Fatalf("hostScopes = %v, want cache,orphans (tmux and load have no mid-wave release)", got)
	}
}

func TestWaveStatus_AnUnreadableHostSampleIsAWarning(t *testing.T) {
	t.Parallel()
	p := newWavePlane(t)
	p.seedMeasuredRuns()
	p.write(".evolve/"+hostsentinel.StatusFile, "{")
	h := newWaveHarness()
	h.run("status", "--project-root", p.root)
	if out := h.stdout.String(); !strings.Contains(out, "WARN:    host sentinel:") || strings.Contains(out, "host:    sampled") {
		t.Errorf("text:\n%s\nwant a host sentinel warning and no sample line", out)
	}
}

func TestNewHostSentinel_AStatusThatCannotBeSavedIsAWarningAndTheTickReturns(t *testing.T) {
	t.Parallel()
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	writeHostPolicy(t, evolveDir, `{"host_sentinel":{"stage":"shadow","disk_low_gib":0.000001,"floor_gib":0.000001,"tmux_sessions_max":100000,"orphans_max":100000,"load_per_cpu_max":100000},"gc":{"go_cache_max_gb":100000}}`)
	if err := os.MkdirAll(filepath.Join(evolveDir, hostsentinel.StatusFile), 0o755); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	host := newHostSentinel(loopConfig{ProjectRoot: filepath.Dir(evolveDir), EvolveDir: evolveDir}, nil, &stderr)
	host.Tick(context.Background())
	if !strings.Contains(stderr.String(), "[host] WARN: host sentinel: the last sample was not saved") {
		t.Fatalf("stderr = %q, want the failed save as a warning", stderr.String())
	}
}
