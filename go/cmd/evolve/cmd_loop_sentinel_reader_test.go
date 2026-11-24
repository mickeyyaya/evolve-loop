package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
	"github.com/mickeyyaya/evolve-loop/go/internal/hostsentinel"
)

func newTestReader(t *testing.T) (*sentinelReader, *[]error) {
	t.Helper()
	var warned []error
	return &sentinelReader{
		root: "/root", tmp: func() string { return "/tmp" },
		freeBytes: func(p string) (uint64, error) {
			return map[string]uint64{"/root": 50e9, "/tmp": 30e9, "/cache": 80e9}[p], nil
		},
		cacheDir: func(context.Context) (string, error) { return "/cache", nil },
		measure:  func(context.Context, string) (int64, error) { return 41e9, nil },
		servers:  func(context.Context) (int, error) { return 7, nil },
		orphans:  func(context.Context) (int, error) { return 9, nil },
		warn:     func(err error) { warned = append(warned, err) },
	}, &warned
}

func TestSentinelReader_FreeDiskIsTheMinimumOverTheThreeVolumes(t *testing.T) {
	t.Parallel()
	r, _ := newTestReader(t)
	s, err := r.read(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if s.FreeBytes != 30e9 || s.Path != "/tmp" {
		t.Fatalf("free %d at %q, want the 30 GB of /tmp", s.FreeBytes, s.Path)
	}
}

func TestSentinelReader_ALightTickReadsOnlyFreeDisk(t *testing.T) {
	t.Parallel()
	r, _ := newTestReader(t)
	r.measure = func(context.Context, string) (int64, error) {
		t.Fatal("a light tick measured the cache")
		return 0, nil
	}
	r.servers = func(context.Context) (int, error) { t.Fatal("a light tick listed servers"); return 0, nil }
	r.orphans = func(context.Context) (int, error) { t.Fatal("a light tick listed processes"); return 0, nil }
	s, err := r.read(context.Background(), false)
	if err != nil || s.CacheRead || s.CountsRead {
		t.Fatalf("sample %+v err %v, want only the free disk", s, err)
	}
}

func TestSentinelReader_AFullTickReadsTheCacheAndBothCounts(t *testing.T) {
	t.Parallel()
	r, _ := newTestReader(t)
	s, err := r.read(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !s.CacheRead || s.CacheBytes != 41e9 || !s.CountsRead || s.Servers != 7 || s.Orphans != 9 {
		t.Fatalf("sample = %+v", s)
	}
}

func TestSentinelReader_TheCacheDirJoinsTheVolumesOnlyOnceKnown(t *testing.T) {
	t.Parallel()
	r, _ := newTestReader(t)
	r.freeBytes = func(p string) (uint64, error) {
		if p == "/cache" {
			return 5e9, nil
		}
		return 50e9, nil
	}
	light, _ := r.read(context.Background(), false)
	if light.FreeBytes != 50e9 {
		t.Fatalf("a volume that is not known yet counted: %+v", light)
	}
	full, _ := r.read(context.Background(), true)
	if full.FreeBytes != 5e9 || full.Path != "/cache" {
		t.Fatalf("full tick = %+v, want the cache volume", full)
	}
	again, _ := r.read(context.Background(), true)
	if again.FreeBytes != 5e9 || again.Path != "/cache" {
		t.Fatalf("the cache dir was forgotten: %+v", again)
	}
}

func TestSentinelReader_AFailedOptionalReadingIsWarnedAndLeftUnread(t *testing.T) {
	t.Parallel()
	r, warned := newTestReader(t)
	r.measure = func(context.Context, string) (int64, error) { return 0, errors.New("walk failed") }
	r.servers = func(context.Context) (int, error) { return 0, errors.New("no tmux") }
	s, err := r.read(context.Background(), true)
	if err != nil || s.CacheRead || s.CountsRead {
		t.Fatalf("sample %+v err %v, want the failed parts unread and no error", s, err)
	}
	if len(*warned) != 2 {
		t.Fatalf("warnings = %v, want the cache and the count failures", *warned)
	}
	r.cacheDir = func(context.Context) (string, error) { return "", errors.New("no go toolchain") }
	r.dir = ""
	if _, err := r.read(context.Background(), true); err != nil || len(*warned) != 3 {
		t.Fatalf("a missing cache dir: err %v warnings %v", err, *warned)
	}
}

func TestSentinelReader_AFailedOrphanListIsWarnedAndTheCountsStayUnread(t *testing.T) {
	t.Parallel()
	r, warned := newTestReader(t)
	r.orphans = func(context.Context) (int, error) { return 0, errors.New("lsof failed") }
	s, _ := r.read(context.Background(), true)
	if s.CountsRead || len(*warned) != 1 {
		t.Fatalf("sample %+v warnings %v", s, *warned)
	}
}

func TestSentinelReader_ANoVolumeReadableIsAnErrorAndAPartialFailureWarnsOnce(t *testing.T) {
	t.Parallel()
	r, warned := newTestReader(t)
	r.freeBytes = func(p string) (uint64, error) {
		if p == "/root" {
			return 50e9, nil
		}
		return 0, errors.New("statfs failed")
	}
	for i := 0; i < 3; i++ {
		if s, err := r.read(context.Background(), false); err != nil || s.FreeBytes != 50e9 {
			t.Fatalf("partial read: %+v %v", s, err)
		}
	}
	if len(*warned) != 1 {
		t.Fatalf("warnings = %v, want one for the failing volume", *warned)
	}
	r.freeBytes = func(string) (uint64, error) { return 0, errors.New("statfs failed") }
	if _, err := r.read(context.Background(), false); err == nil {
		t.Fatal("no readable volume gave no error")
	}
}

func TestMeasureTree_SumsEveryFileAndStopsOnACancel(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for name, n := range map[string]int{"a/one": 100, "a/two": 200, "b/three": 300} {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, make([]byte, n), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := measureTree(context.Background(), dir)
	if err != nil || got != 600 {
		t.Fatalf("size %d err %v, want 600", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := measureTree(ctx, dir); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled walk gave %v", err)
	}
	if _, err := measureTree(context.Background(), filepath.Join(dir, "absent")); err == nil {
		t.Fatal("a missing dir gave no error")
	}
}

func TestProductionSentinelReader_CountsOwnerDeadServersAndFinishedCycleOrphansWithoutKilling(t *testing.T) {
	root, _, _ := gcWorktreeEnv(t, "")
	sockets := bridge.TmuxSocketDir()
	if err := os.MkdirAll(sockets, 0o755); err != nil {
		t.Fatal(err)
	}
	dead := filepath.Join(sockets, "evolve-bridge-p2147483646")
	if err := os.WriteFile(dead, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	listing := "p4242\nR1\nn" + filepath.Join(root, ".evolve", "worktrees", "cycle-99") + "\n"
	listProcesses := func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, out, _ io.Writer) (int, error) {
		_, _ = io.WriteString(out, listing)
		return 0, nil
	}
	r := productionSentinelReader(root, filepath.Join(root, ".evolve"), func(error) {}, listProcesses)
	if n, err := r.servers(context.Background()); err != nil || n != 1 {
		t.Fatalf("servers = %d err %v, want the one owner-dead socket counted", n, err)
	}
	if n, err := r.orphans(context.Background()); err != nil || n != 1 {
		t.Fatalf("orphans = %d err %v, want the one finished-cycle orphan counted", n, err)
	}
	if gone(dead) {
		t.Fatal("counting the owner-dead server removed its socket")
	}
	if r.tmp() != os.TempDir() {
		t.Fatalf("tmp = %q", r.tmp())
	}
	failing := func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		return 1, errors.New("lsof missing")
	}
	broken := productionSentinelReader(root, filepath.Join(root, ".evolve"), func(error) {}, failing)
	if n, err := broken.orphans(context.Background()); err == nil || n != 0 {
		t.Fatalf("orphans = %d err = %v, want a listing failure and no count", n, err)
	}
}

func TestSentinelPortsFor_FillsEachMissingPortFromProductionAndKeepsTheInjectedOnes(t *testing.T) {
	t.Parallel()
	b, _, _ := supplyCoordinator(t, t.TempDir())
	empty := b.sentinelPortsFor(gcpolicy.Policy{})
	if empty.read == nil || empty.release == nil || empty.ticker == nil || empty.record == nil {
		t.Fatalf("a missing port stayed nil: %+v", empty)
	}
	b.sentinel.read = func(context.Context, bool) (hostsentinel.Sample, error) {
		return hostsentinel.Sample{FreeBytes: 7}, nil
	}
	kept := b.sentinelPortsFor(gcpolicy.Policy{})
	if s, err := kept.read(context.Background(), false); err != nil || s.FreeBytes != 7 {
		t.Fatalf("the injected read was replaced: %+v %v", s, err)
	}
}
