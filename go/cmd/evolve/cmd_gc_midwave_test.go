package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
	"github.com/mickeyyaya/evolve-loop/go/internal/hostsentinel"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/verifylock"
)

type midWaveFixture struct {
	root       string
	tmp        string
	heldTemp   string
	staleTemp  string
	liveSocket string
	freshEntry string
	mappedDir  string
	staleEntry string
	cacheDir   string
}

func newMidWaveFixture(t *testing.T) midWaveFixture {
	t.Helper()
	root, _, _ := gcWorktreeEnv(t, "")
	stale, fresh := gcFakeGoCache(t)
	cacheDir := filepath.Dir(filepath.Dir(stale))
	mapped := filepath.Join(cacheDir, "0c", "0c1b2c-d")
	if err := os.MkdirAll(mapped, 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(mapped, old, old); err != nil {
		t.Fatal(err)
	}
	gcWritePolicy(t, root, `{"go_cache_ttl_hours":24,"temp_ttl_hours":1}`)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	f := midWaveFixture{root: root, tmp: tmp, staleEntry: stale, freshEntry: fresh, mappedDir: mapped, cacheDir: cacheDir,
		heldTemp: filepath.Join(tmp, "go-build111111"), staleTemp: filepath.Join(tmp, "go-build222222"), liveSocket: filepath.Join(tmp, "tmux-live-sockets")}
	for _, dir := range []string{f.heldTemp, f.staleTemp, f.liveSocket} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(dir, old, old); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f midWaveFixture) run(t *testing.T, mutate func(*gcRun)) midWaveRun {
	t.Helper()
	return midWaveRun{
		root: f.root, budget: time.Minute,
		lease: takeGCLease,
		newRun: func(ctx context.Context, stdout, stderr io.Writer) gcRun {
			r := newGCRun(ctx, false, stdout, stderr)
			r.cancellable = true
			r.exec = func(_ context.Context, name, _ string, _, _ []string, _ io.Reader, out, _ io.Writer) (int, error) {
				if name == "lsof" {
					_, _ = io.WriteString(out, "p4242\nn"+f.heldTemp+"\nn"+f.mappedDir+"\n")
				}
				return 0, nil
			}
			if mutate != nil {
				mutate(&r)
			}
			return r
		},
		free:     func() (uint64, string, error) { return 50e9, "/root", nil },
		cacheDir: func(context.Context) (string, error) { return f.cacheDir, nil },
		measure:  func(context.Context, string) (int64, error) { return 1234, nil },
	}
}

func gone(path string) bool {
	_, err := os.Stat(path)
	return errors.Is(err, fs.ErrNotExist)
}

func TestMidWave_LeavesLiveResourcesInPlace(t *testing.T) {
	f := newMidWaveFixture(t)
	out := f.run(t, nil).run(context.Background())
	if out.Skipped != "" || out.Err != "" {
		t.Fatalf("the run did not complete: %+v", out)
	}
	for name, path := range map[string]string{"held temp dir": f.heldTemp, "fresh cache entry": f.freshEntry, "mapped -d dir": f.mappedDir, "live socket dir": f.liveSocket} {
		if gone(path) {
			t.Errorf("the mid-wave run removed the %s %s", name, path)
		}
	}
	if !gone(f.staleEntry) || !gone(f.staleTemp) {
		t.Errorf("the run kept a stale resource: cache entry %v temp %v", !gone(f.staleEntry), !gone(f.staleTemp))
	}
}

func TestMidWave_RunsOnlyCacheScope(t *testing.T) {
	f := newMidWaveFixture(t)
	var signalled int
	m := f.run(t, func(r *gcRun) { r.kill = func(int) error { signalled++; return nil } })
	var steps []string
	base := m.newRun
	m.newRun = func(ctx context.Context, stdout, stderr io.Writer) gcRun {
		r := base(ctx, stdout, stderr)
		r.remove = func(p string) error { steps = append(steps, "remove "+filepath.Base(p)); return os.Remove(p) }
		r.removeAll = func(p string) error { steps = append(steps, "removeAll "+filepath.Base(p)); return os.RemoveAll(p) }
		return r
	}
	m.run(context.Background())
	if signalled != 0 {
		t.Fatalf("the mid-wave run signalled %d processes", signalled)
	}
	want := []string{"remove 0a1b2c-d", "removeAll go-build222222"}
	if strings.Join(steps, ",") != strings.Join(want, ",") {
		t.Fatalf("removals = %v, want exactly the cache step and the temp step %v", steps, want)
	}
}

func TestMidWave_AHeldGCLeaseSkipsWithoutWaitingOrRemoving(t *testing.T) {
	f := newMidWaveFixture(t)
	release, err := verifylock.AcquireAt(context.Background(), filepath.Join(f.root, ".evolve", gcLeaseFile), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	start := time.Now()
	out := f.run(t, nil).run(context.Background())
	if out.Skipped != "gc_lease_held" {
		t.Fatalf("outcome = %+v, want a skip for the held lease", out)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("the run waited %v for the lease", time.Since(start))
	}
	if gone(f.staleEntry) || gone(f.staleTemp) {
		t.Fatal("a skipped run removed a resource")
	}
}

func TestMidWave_AStopDuringARunReturnsInTheBudgetAndCancelsTheChild(t *testing.T) {
	f := newMidWaveFixture(t)
	m := f.run(t, nil)
	started := make(chan struct{})
	childCancelled := make(chan struct{})
	base := m.newRun
	m.newRun = func(ctx context.Context, stdout, stderr io.Writer) gcRun {
		r := base(ctx, stdout, stderr)
		r.exec = func(c context.Context, name, _ string, _, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
			close(started)
			<-c.Done()
			close(childCancelled)
			return 1, c.Err()
		}
		return r
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan midWaveOutcome, 1)
	go func() { done <- m.run(ctx) }()
	<-started
	cancel()
	select {
	case <-childCancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("the child never saw its context cancel")
	}
	select {
	case out := <-done:
		if out.Err == "" {
			t.Fatalf("a stopped run reported no error: %+v", out)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the run did not return after the stop")
	}
}

func TestMidWave_AStopBetweenTheStepsSkipsTheTempStep(t *testing.T) {
	f := newMidWaveFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := f.run(t, func(r *gcRun) {
		r.remove = func(p string) error {
			cancel()
			return os.Remove(p)
		}
	})
	out := m.run(ctx)
	if !gone(f.staleEntry) {
		t.Fatal("setup: the cache step did not run")
	}
	if gone(f.staleTemp) {
		t.Fatal("the temp step ran after the stop")
	}
	if out.Err == "" {
		t.Fatalf("a run stopped between its steps reported no error: %+v", out)
	}
}

func TestMidWave_RemeasuresTheCacheAfterTheTrim(t *testing.T) {
	f := newMidWaveFixture(t)
	m := f.run(t, nil)
	m.measure = func(context.Context, string) (int64, error) { return 53_000_000_000, nil }
	out := m.run(context.Background())
	if out.After.CacheBytes != 53_000_000_000 || !out.After.CacheRead {
		t.Fatalf("after-sample cache = %d read %v, want the re-measured 53 GB", out.After.CacheBytes, out.After.CacheRead)
	}
}

func TestMidWave_FreedBytesIsTheFreeDiskDelta(t *testing.T) {
	f := newMidWaveFixture(t)
	m := f.run(t, nil)
	reads := []uint64{30e9, 35e9}
	m.free = func() (uint64, string, error) {
		v := reads[0]
		reads = reads[1:]
		return v, "/root", nil
	}
	out := m.run(context.Background())
	if out.Freed != 5_000_000_000 || out.After.FreeBytes != 35e9 {
		t.Fatalf("freed %d after-free %d, want 5 GB and 35 GB", out.Freed, out.After.FreeBytes)
	}
}

func TestMidWave_AnUnreadableDiskOrCacheNamesTheFailure(t *testing.T) {
	f := newMidWaveFixture(t)
	m := f.run(t, nil)
	m.free = func() (uint64, string, error) { return 0, "", errors.New("statfs failed") }
	if out := m.run(context.Background()); !strings.Contains(out.Err, "statfs failed") {
		t.Fatalf("outcome = %+v, want the disk error named", out)
	}
	m = f.run(t, nil)
	m.measure = func(context.Context, string) (int64, error) { return 0, errors.New("walk failed") }
	if out := m.run(context.Background()); !strings.Contains(out.Err, "walk failed") {
		t.Fatalf("outcome = %+v, want the cache error named", out)
	}
	m = f.run(t, nil)
	m.cacheDir = func(context.Context) (string, error) { return "", errors.New("no go") }
	if out := m.run(context.Background()); !strings.Contains(out.Err, "no go") {
		t.Fatalf("outcome = %+v, want the cache dir error named", out)
	}
}

func TestMidWave_AnUnreadableDiskAfterTheRunNamesTheFailure(t *testing.T) {
	f := newMidWaveFixture(t)
	m := f.run(t, nil)
	calls := 0
	m.free = func() (uint64, string, error) {
		calls++
		if calls == 2 {
			return 0, "", errors.New("statfs gone")
		}
		return 50e9, "/root", nil
	}
	if out := m.run(context.Background()); !strings.Contains(out.Err, "after the run") || !strings.Contains(out.Err, "statfs gone") {
		t.Fatalf("outcome = %+v, want the disk error after the run named", out)
	}
}

func TestMidWave_AFailedCacheScopeReportsTheGCOutput(t *testing.T) {
	f := newMidWaveFixture(t)
	m := f.run(t, func(r *gcRun) { r.remove = func(string) error { return errors.New("read-only cache") } })
	out := m.run(context.Background())
	if !strings.Contains(out.Err, "read-only cache") {
		t.Fatalf("outcome = %+v, want the gc stderr in the error", out)
	}
}

func TestProductionSentinelPorts_TheReleaseRunsTheCacheScopeUnderTheGCLease(t *testing.T) {
	f := newMidWaveFixture(t)
	b, _, _ := supplyCoordinator(t, f.root)
	ports := b.productionSentinelPorts(policy.GCFor(f.root))
	out := ports.release(context.Background())
	if out.Skipped != "" || out.Err != "" {
		t.Fatalf("outcome = %+v, want a completed run", out)
	}
	if !gone(f.staleEntry) || gone(f.freshEntry) {
		t.Fatalf("stale entry gone %v fresh entry gone %v, want only the stale entry removed", gone(f.staleEntry), gone(f.freshEntry))
	}
	if out.After.CacheBytes == 0 || !out.After.CacheRead {
		t.Fatalf("after-sample = %+v, want the cache measured again", out.After)
	}
}

func TestCacheScope_RunsExactlyTheStepsThatTheSentinelAnnounces(t *testing.T) {
	withDiskFree(t, 1e9, nil)
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	gcFakeGoCache(t)
	gcWritePolicy(t, projectRoot, `{"go_cache_ttl_hours":24,"temp_ttl_hours":24}`)
	var stdout, stderr bytes.Buffer

	newGCRun(context.Background(), true, &stdout, &stderr).cacheScope(projectRoot)

	var ran []string
	for _, line := range strings.Split(stdout.String(), "\n") {
		switch {
		case strings.Contains(line, "go build cache"):
			ran = appendOnce(ran, "go_cache")
		case strings.Contains(line, "pipeline temp"):
			ran = appendOnce(ran, "pipeline_temp")
		case strings.HasPrefix(line, "evolve gc: ") && !strings.Contains(line, "disk.floor_gb"):
			t.Errorf("cache scope printed %q, a step that the sentinel does not announce in hostsentinel.Scopes", line)
		}
	}
	if !slices.Equal(ran, hostsentinel.Scopes) {
		t.Errorf("cache scope ran %v, want the steps %v that HOST_WOULD_RELEASE announces", ran, hostsentinel.Scopes)
	}
}

func appendOnce(list []string, name string) []string {
	if slices.Contains(list, name) {
		return list
	}
	return append(list, name)
}

func TestFailureText_KeepsTheTailAndNamesAStoppedRun(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", midWaveErrorTail) + "TAIL-MARKER"
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	cases := []struct {
		name    string
		ctx     context.Context
		stderr  string
		want    string
		notWant string
	}{
		{"short text passes through trimmed", context.Background(), "  read-only cache\n", "the cache scope failed: read-only cache", "stopped"},
		{"long text keeps only the tail", context.Background(), long, "TAIL-MARKER", strings.Repeat("a", midWaveErrorTail)},
		{"a stopped run names the context error", cancelled, "partial", "the run stopped (context canceled): partial", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := failureText(c.ctx, bytes.NewBufferString(c.stderr))
			if !strings.Contains(got, c.want) {
				t.Fatalf("failureText = %q, want it to contain %q", got, c.want)
			}
			if c.notWant != "" && strings.Contains(got, c.notWant) {
				t.Fatalf("failureText = %q, must not contain %q", got, c.notWant)
			}
		})
	}
}

func TestMidWave_FreedAndTheBackoffFollowTheMinimumVolumeNotTheRoot(t *testing.T) {
	f := newMidWaveFixture(t)
	tmpFree, rootFree := []uint64{15e9, 15e9}, []uint64{50e9, 60e9}
	reader := &sentinelReader{
		root: "/root", tmp: func() string { return "/tmp" },
		freeBytes: func(p string) (uint64, error) {
			if p == "/tmp" {
				v := tmpFree[0]
				tmpFree = tmpFree[1:]
				return v, nil
			}
			v := rootFree[0]
			rootFree = rootFree[1:]
			return v, nil
		},
		warn: func(error) {},
	}
	m := f.run(t, nil)
	m.free = midWaveRunOver(reader, "/root", gcpolicy.Policy{}).free
	out := m.run(context.Background())
	if out.After.FreeBytes != 15e9 || out.After.Path != "/tmp" || out.Freed != 0 {
		t.Fatalf("outcome = %+v, want the /tmp volume at 15 GB and no bytes freed", out)
	}
	cfg := hostsentinel.Config{WarnBytes: 40e9, Repeat: time.Minute, NoProgressBytes: 1e9}
	st := hostsentinel.State{}.AfterRelease(cfg, out.Freed, out.After)
	d, _ := hostsentinel.Decide(hostsentinel.Sample{FreeBytes: out.After.FreeBytes, Path: "/tmp"}, cfg, st, time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC))
	if d.Release {
		t.Fatal("a release that freed nothing on the minimum volume started another run at the same reading")
	}
}

func TestMidWaveRunOver_ReadsFreeDiskThroughTheSentinelReader(t *testing.T) {
	t.Parallel()
	reader := &sentinelReader{
		root: "/root", tmp: func() string { return "/tmp" },
		freeBytes: func(p string) (uint64, error) { return map[string]uint64{"/root": 50e9, "/tmp": 15e9}[p], nil },
		warn:      func(error) {},
	}
	free, path, err := midWaveRunOver(reader, "/root", gcpolicy.Policy{}).free()
	if err != nil || free != 15e9 || path != "/tmp" {
		t.Fatalf("free %d at %q err %v, want the 15 GB of /tmp", free, path, err)
	}
}
