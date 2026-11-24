package hostsentinel_test

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/hostsentinel"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const gb = uint64(1e9)

var t0 = time.Date(2026, 11, 21, 8, 0, 0, 0, time.UTC)

func testConfig() hostsentinel.Config {
	return hostsentinel.Config{
		FloorBytes: 10 * gb, WarnBytes: 20 * gb, CacheLimitBytes: int64(20 * gb),
		ServersMax: 20, OrphansMax: 20,
		Repeat: 900 * time.Second, HoldMax: 7200 * time.Second, NoProgressBytes: int64(gb),
	}
}

func healthy() hostsentinel.Sample {
	return hostsentinel.Sample{FreeBytes: 100 * gb, Path: "/p", CacheBytes: int64(5 * gb), CacheRead: true, CountsRead: true}
}

func codes(d hostsentinel.Decision) []signalcenter.Code {
	var out []signalcenter.Code
	for _, f := range d.Findings {
		out = append(out, f.Code)
	}
	return out
}

func TestDecide_EmitsOneFindingForEachCrossedThreshold(t *testing.T) {
	t.Parallel()
	s := hostsentinel.Sample{FreeBytes: 8 * gb, Path: "/p", CacheBytes: int64(41 * gb), CacheRead: true, Servers: 30, Orphans: 25, CountsRead: true}
	d, _ := hostsentinel.Decide(s, testConfig(), hostsentinel.State{}, t0)
	want := []signalcenter.Code{hostsentinel.CodeDiskFloor, hostsentinel.CodeGoCacheOver, hostsentinel.CodeServersHigh, hostsentinel.CodeOrphansHigh}
	got := codes(d)
	if len(got) != len(want) {
		t.Fatalf("findings = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("finding %d = %s, want %s", i, got[i], want[i])
		}
	}
	checks := []struct {
		idx          int
		field, value string
	}{
		{0, "free_bytes", "8000000000"}, {0, "floor_bytes", "10000000000"}, {0, "warn_bytes", "20000000000"}, {0, "path", "/p"},
		{1, "cache_bytes", "41000000000"}, {1, "limit_bytes", "20000000000"},
		{2, "count", "30"}, {2, "limit", "20"}, {3, "count", "25"}, {3, "limit", "20"},
	}
	for _, c := range checks {
		if v := d.Findings[c.idx].Fields[c.field]; v != c.value {
			t.Errorf("finding %s field %s = %q, want %q", got[c.idx], c.field, v, c.value)
		}
	}
}

func TestDecide_AHealthyHostGivesNothing(t *testing.T) {
	t.Parallel()
	d, st := hostsentinel.Decide(healthy(), testConfig(), hostsentinel.State{}, t0)
	if len(d.Findings) != 0 || d.Hold || d.HoldChanged || d.Release || d.Stuck {
		t.Fatalf("decision = %+v, want empty", d)
	}
	if st.Held {
		t.Fatal("a healthy host holds")
	}
}

func TestDecide_UnreadValuesCrossNoThreshold(t *testing.T) {
	t.Parallel()
	s := hostsentinel.Sample{FreeBytes: 100 * gb, CacheBytes: int64(99 * gb), Servers: 99, Orphans: 99}
	d, _ := hostsentinel.Decide(s, testConfig(), hostsentinel.State{}, t0)
	if len(d.Findings) != 0 || d.Release {
		t.Fatalf("a value that was not read acted: %+v", d)
	}
}

func TestDecide_ADiskBetweenFloorAndWarnGivesTheWarnSignal(t *testing.T) {
	t.Parallel()
	s := healthy()
	s.FreeBytes = 15 * gb
	d, _ := hostsentinel.Decide(s, testConfig(), hostsentinel.State{}, t0)
	if got := codes(d); len(got) != 1 || got[0] != hostsentinel.CodeDiskWarn {
		t.Fatalf("findings = %v, want the warn code", got)
	}
	if d.Findings[0].Fields["warn_bytes"] != "20000000000" || d.Hold {
		t.Fatalf("warn finding = %+v hold %v", d.Findings[0], d.Hold)
	}
}

func TestDecide_ARepeatInsideRepeatSecondsEmitsNothing(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	s := healthy()
	s.Orphans = 25
	s.CountsRead = true
	_, st := hostsentinel.Decide(s, cfg, hostsentinel.State{}, t0)
	inside, st := hostsentinel.Decide(s, cfg, st, t0.Add(cfg.Repeat-time.Second))
	if len(inside.Findings) != 0 {
		t.Fatalf("a repeat inside the interval emitted %v", codes(inside))
	}
	at, _ := hostsentinel.Decide(s, cfg, st, t0.Add(cfg.Repeat))
	if got := codes(at); len(got) != 1 || got[0] != hostsentinel.CodeOrphansHigh {
		t.Fatalf("a repeat at the interval emitted %v", got)
	}
}

func TestDecide_ASecondBandRepeatsAtOnce(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	s := healthy()
	s.Orphans = 25
	_, st := hostsentinel.Decide(s, cfg, hostsentinel.State{}, t0)
	s.Orphans = 40
	d, st := hostsentinel.Decide(s, cfg, st, t0.Add(time.Second))
	if got := codes(d); len(got) != 1 || got[0] != hostsentinel.CodeOrphansHigh {
		t.Fatalf("twice the limit did not repeat at once: %v", got)
	}
	again, _ := hostsentinel.Decide(s, cfg, st, t0.Add(2*time.Second))
	if len(again.Findings) != 0 {
		t.Fatalf("the second band repeated again inside the interval: %v", codes(again))
	}

	low := healthy()
	low.FreeBytes = 9 * gb
	_, st = hostsentinel.Decide(low, cfg, hostsentinel.State{}, t0)
	low.FreeBytes = 5 * gb
	d, _ = hostsentinel.Decide(low, cfg, st, t0.Add(time.Second))
	if got := codes(d); len(got) != 1 || got[0] != hostsentinel.CodeDiskFloor {
		t.Fatalf("half the floor did not repeat at once: %v", got)
	}
}

func TestDecide_APersistentOrphanFindingStartsNoRelease(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	s := healthy()
	s.Orphans, s.Servers = 50, 50
	st := hostsentinel.State{}
	for i := 0; i < 10; i++ {
		var d hostsentinel.Decision
		d, st = hostsentinel.Decide(s, cfg, st, t0.Add(time.Duration(i)*time.Hour))
		if d.Release {
			t.Fatalf("tick %d started a release for an orphan finding", i)
		}
	}
}

func TestDecide_ADiskUnderWarnStartsOneReleaseThenWaitsForRepeat(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	s := healthy()
	s.FreeBytes = 15 * gb
	d, st := hostsentinel.Decide(s, cfg, hostsentinel.State{}, t0)
	if !d.Release {
		t.Fatal("no release for a disk under warn")
	}
	d, st = hostsentinel.Decide(s, cfg, st, t0.Add(cfg.Repeat-time.Second))
	if d.Release {
		t.Fatal("a second release inside the cooldown")
	}
	d, _ = hostsentinel.Decide(s, cfg, st, t0.Add(cfg.Repeat))
	if !d.Release {
		t.Fatal("no release after the cooldown")
	}
}

func TestDecide_AnOverCapCacheStartsARelease(t *testing.T) {
	t.Parallel()
	s := healthy()
	s.CacheBytes = int64(30 * gb)
	d, _ := hostsentinel.Decide(s, testConfig(), hostsentinel.State{}, t0)
	if !d.Release {
		t.Fatal("no release for a cache over its cap")
	}
}

func TestDecide_ANoProgressReleaseBlocksTheNextRunUntilTheReadingGrows(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	s := healthy()
	s.CacheBytes = int64(30 * gb)
	d, st := hostsentinel.Decide(s, cfg, hostsentinel.State{}, t0)
	if !d.Release {
		t.Fatal("setup: no first release")
	}
	st = st.AfterRelease(cfg, 100, s)
	d, st = hostsentinel.Decide(s, cfg, st, t0.Add(cfg.Repeat))
	if d.Release {
		t.Fatal("a run started for a reading that did not grow")
	}
	s.CacheBytes = int64(31 * gb)
	d, _ = hostsentinel.Decide(s, cfg, st, t0.Add(2*cfg.Repeat))
	if !d.Release {
		t.Fatal("a grown reading did not start a run")
	}
}

func TestDecide_ANoProgressReleaseBlocksTheDiskFindingUntilFreeFalls(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	s := healthy()
	s.FreeBytes = 15 * gb
	_, st := hostsentinel.Decide(s, cfg, hostsentinel.State{}, t0)
	st = st.AfterRelease(cfg, 0, s)
	d, st := hostsentinel.Decide(s, cfg, st, t0.Add(cfg.Repeat))
	if d.Release {
		t.Fatal("a run started with free disk unchanged")
	}
	s.FreeBytes = 14 * gb
	d, _ = hostsentinel.Decide(s, cfg, st, t0.Add(2*cfg.Repeat))
	if !d.Release {
		t.Fatal("falling free disk did not start a run")
	}
}

func TestDecide_ADiskThatRecoveredPastWarnStartsANewEpisodeAtAHigherFree(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	s := healthy()
	s.FreeBytes = 15 * gb
	_, st := hostsentinel.Decide(s, cfg, hostsentinel.State{}, t0)
	st = st.AfterRelease(cfg, 0, s)
	_, st = hostsentinel.Decide(healthy(), cfg, st, t0.Add(cfg.Repeat))
	s.FreeBytes = 18 * gb
	d, _ := hostsentinel.Decide(s, cfg, st, t0.Add(2*cfg.Repeat))
	if !d.Release {
		t.Fatal("a new low-disk episode after a recovery started no release")
	}
}

func TestDecide_ACacheThatRecoveredUnderItsCapStartsANewEpisodeAtASmallerSize(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	s := healthy()
	s.CacheBytes = int64(30 * gb)
	_, st := hostsentinel.Decide(s, cfg, hostsentinel.State{}, t0)
	st = st.AfterRelease(cfg, 0, s)
	_, st = hostsentinel.Decide(healthy(), cfg, st, t0.Add(cfg.Repeat))
	s.CacheBytes = int64(25 * gb)
	d, _ := hostsentinel.Decide(s, cfg, st, t0.Add(2*cfg.Repeat))
	if !d.Release {
		t.Fatal("a new over-cap episode after a recovery started no release")
	}
}

func TestDecide_ARecoveryToExactlyTheWarnLineOrTheCapEndsTheBackoff(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	disk := healthy()
	disk.FreeBytes = 15 * gb
	cache := healthy()
	cache.CacheBytes = int64(30 * gb)
	for _, c := range []struct {
		name      string
		stalled   hostsentinel.Sample
		atTheLine hostsentinel.Sample
		again     hostsentinel.Sample
	}{
		{"disk", disk, withFree(healthy(), cfg.WarnBytes), withFree(healthy(), 18*gb)},
		{"cache", cache, withCache(healthy(), cfg.CacheLimitBytes), withCache(healthy(), int64(25*gb))},
	} {
		_, st := hostsentinel.Decide(c.stalled, cfg, hostsentinel.State{}, t0)
		st = st.AfterRelease(cfg, 0, c.stalled)
		_, st = hostsentinel.Decide(c.atTheLine, cfg, st, t0.Add(cfg.Repeat))
		d, _ := hostsentinel.Decide(c.again, cfg, st, t0.Add(2*cfg.Repeat))
		if !d.Release {
			t.Errorf("%s: a reading exactly at the line did not end the backoff", c.name)
		}
	}
}

func TestDecide_AReleaseThatFreesExactlyNoProgressBytesIsProgress(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	s := healthy()
	s.CacheBytes = int64(30 * gb)
	_, st := hostsentinel.Decide(s, cfg, hostsentinel.State{}, t0)
	st = st.AfterRelease(cfg, cfg.NoProgressBytes, s)
	d, _ := hostsentinel.Decide(s, cfg, st, t0.Add(cfg.Repeat))
	if !d.Release {
		t.Fatal("a run that freed exactly no_progress_bytes still blocks the next run")
	}
}

func withFree(s hostsentinel.Sample, free uint64) hostsentinel.Sample {
	s.FreeBytes = free
	return s
}

func withCache(s hostsentinel.Sample, bytes int64) hostsentinel.Sample {
	s.CacheBytes = bytes
	return s
}

func TestDecide_AnUnreadCacheKeepsTheCacheBackoff(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	s := healthy()
	s.CacheBytes = int64(30 * gb)
	_, st := hostsentinel.Decide(s, cfg, hostsentinel.State{}, t0)
	st = st.AfterRelease(cfg, 0, s)
	unread := healthy()
	unread.CacheRead = false
	_, st = hostsentinel.Decide(unread, cfg, st, t0.Add(cfg.Repeat))
	d, _ := hostsentinel.Decide(s, cfg, st, t0.Add(2*cfg.Repeat))
	if d.Release {
		t.Fatal("an unread cache cleared the backoff of a cache that did not recover")
	}
}

func TestDecide_AReleaseThatFreesEnoughClearsTheBackoff(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	s := healthy()
	s.CacheBytes = int64(30 * gb)
	_, st := hostsentinel.Decide(s, cfg, hostsentinel.State{}, t0)
	st = st.AfterRelease(cfg, 100, s)
	st = st.AfterRelease(cfg, int64(2*gb), s)
	d, _ := hostsentinel.Decide(s, cfg, st, t0.Add(cfg.Repeat))
	if !d.Release {
		t.Fatal("a run that freed enough still blocks the next run")
	}
}

func TestDecide_TheHoldSetsBelowTheFloorKeepsBetweenAndClearsAtWarn(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	s := healthy()
	s.FreeBytes = 10*gb - 1
	d, st := hostsentinel.Decide(s, cfg, hostsentinel.State{}, t0)
	if !d.Hold || !d.HoldChanged || !st.Held {
		t.Fatalf("floor-1 did not set the hold: %+v", d)
	}
	s.FreeBytes = 15 * gb
	d, st = hostsentinel.Decide(s, cfg, st, t0.Add(time.Minute))
	if !d.Hold || d.HoldChanged {
		t.Fatalf("a disk between floor and warn changed the hold: %+v", d)
	}
	s.FreeBytes = 20 * gb
	d, st = hostsentinel.Decide(s, cfg, st, t0.Add(2*time.Minute))
	if d.Hold || !d.HoldChanged || st.Held {
		t.Fatalf("a disk at warn did not clear the hold: %+v", d)
	}
}

func TestDecide_AHoldPastHoldMaxEmitsOneStuckAndKeepsTheHold(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	s := healthy()
	s.FreeBytes = 5 * gb
	_, st := hostsentinel.Decide(s, cfg, hostsentinel.State{}, t0)
	d, st := hostsentinel.Decide(s, cfg, st, t0.Add(cfg.HoldMax-time.Second))
	if d.Stuck {
		t.Fatal("stuck before hold_max")
	}
	d, st = hostsentinel.Decide(s, cfg, st, t0.Add(cfg.HoldMax))
	if !d.Stuck || !d.Hold {
		t.Fatalf("hold_max did not report stuck and keep the hold: %+v", d)
	}
	if d.HeldFor != cfg.HoldMax {
		t.Fatalf("held for %v, want %v", d.HeldFor, cfg.HoldMax)
	}
	d, _ = hostsentinel.Decide(s, cfg, st, t0.Add(cfg.HoldMax+time.Hour))
	if d.Stuck || !d.Hold {
		t.Fatalf("stuck repeated or the hold dropped: %+v", d)
	}
}

func TestDecide_AClearedHoldCanGoStuckAgainOnTheNextHold(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	s := healthy()
	s.FreeBytes = 5 * gb
	_, st := hostsentinel.Decide(s, cfg, hostsentinel.State{}, t0)
	_, st = hostsentinel.Decide(s, cfg, st, t0.Add(cfg.HoldMax))
	s.FreeBytes = 50 * gb
	_, st = hostsentinel.Decide(s, cfg, st, t0.Add(cfg.HoldMax+time.Minute))
	s.FreeBytes = 5 * gb
	_, st = hostsentinel.Decide(s, cfg, st, t0.Add(cfg.HoldMax+2*time.Minute))
	d, _ := hostsentinel.Decide(s, cfg, st, t0.Add(2*cfg.HoldMax+2*time.Minute))
	if !d.Stuck {
		t.Fatal("the second hold never reported stuck")
	}
}

func TestDecide_DoesNotMutateTheStateItReceives(t *testing.T) {
	t.Parallel()
	s := healthy()
	s.Orphans = 25
	_, st := hostsentinel.Decide(s, testConfig(), hostsentinel.State{}, t0)
	before, _ := hostsentinel.Decide(s, testConfig(), st, t0.Add(time.Hour))
	again, _ := hostsentinel.Decide(s, testConfig(), st, t0.Add(time.Hour))
	if len(before.Findings) != len(again.Findings) || len(before.Findings) != 1 {
		t.Fatalf("the same state gave %d then %d findings", len(before.Findings), len(again.Findings))
	}
}

func TestDecide_ARepeatedCacheFindingInsideRepeatSecondsEmitsNothing(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	s := healthy()
	s.CacheBytes = int64(30 * gb)
	_, st := hostsentinel.Decide(s, cfg, hostsentinel.State{}, t0)
	d, _ := hostsentinel.Decide(s, cfg, st, t0.Add(time.Second))
	if len(d.Findings) != 0 {
		t.Fatalf("a repeat emitted %v", codes(d))
	}
}

func TestState_DeferReleaseLetsTheNextDecisionRelease(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	s := healthy()
	s.FreeBytes = 15 * gb
	d, st := hostsentinel.Decide(s, cfg, hostsentinel.State{}, t0)
	if !d.Release {
		t.Fatal("setup: no release")
	}
	d, _ = hostsentinel.Decide(s, cfg, st.DeferRelease(), t0.Add(time.Second))
	if !d.Release {
		t.Fatal("a deferred release was not offered again at once")
	}
}
