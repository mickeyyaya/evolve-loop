package gc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func channelDir(evolveDir, name string) string {
	return filepath.Join(paths.EventChannelsRoot(evolveDir), name)
}

func segmentPath(evolveDir, name string, base int64) string {
	return filepath.Join(channelDir(evolveDir, name), fmt.Sprintf("seg-%020d.ndjson", base))
}

func channelOptions(dir string, chs ...gcpolicy.ChannelRetention) Options {
	return Options{EvolveDir: dir, Channels: chs, Now: nowT0}
}

func retention(name string, ttl, capMB int) gcpolicy.ChannelRetention {
	return gcpolicy.ChannelRetention{Name: name, Dir: filepath.ToSlash(filepath.Join(paths.EventChannelsDir, name)), TTLDays: ttl, MaxTotalMB: capMB}
}

func planChannels(t *testing.T, opts Options) Manifest {
	t.Helper()
	m, err := Plan(opts)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	return m
}

func TestPlan_NeverDeletesTheTailSegment(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for i, base := range []int64{0, 100, 200} {
		writeAged(t, segmentPath(dir, "loop", base), 2<<20, daysAgo(90-i))
	}

	m := planChannels(t, channelOptions(dir, retention("loop", 1, 1)))

	items := planItems(t, m)
	if _, deleted := items[segmentPath(dir, "loop", 200)]; deleted {
		t.Errorf("the tail segment is planned for %+v, want it kept although it is expired and over the cap", items[segmentPath(dir, "loop", 200)])
	}
	for _, base := range []int64{0, 100} {
		if it := items[segmentPath(dir, "loop", base)]; it.Action != ActionDelete {
			t.Errorf("segment %d = %+v, want a delete", base, it)
		}
	}
	if len(m.Warnings) != 1 || !strings.Contains(m.Warnings[0], "pinned") || !strings.Contains(m.Warnings[0], `events.channels["loop"].max_total_mb`) {
		t.Errorf("Warnings = %q, want one line that names the pinned logs and the events cap key", m.Warnings)
	}
}

func TestPlan_ExpiredSegmentsBelowTheTailAreDeletedAndFreshOnesStay(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeAged(t, segmentPath(dir, "ship", 0), 10, daysAgo(60))
	writeAged(t, segmentPath(dir, "ship", 50), 10, daysAgo(2))
	writeAged(t, segmentPath(dir, "ship", 100), 10, daysAgo(1))

	items := planItems(t, planChannels(t, channelOptions(dir, retention("ship", 30, 128))))

	if len(items) != 1 || items[segmentPath(dir, "ship", 0)].Rule != `events.channels["ship"].ttl_days` {
		t.Errorf("items = %+v, want only the 60-day segment, by the events ttl key", items)
	}
}

func TestPlan_ChannelSegmentsReportTheEventsRuleKeys(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeAged(t, segmentPath(dir, "quality.breach", 0), 10, daysAgo(60))
	writeAged(t, segmentPath(dir, "quality.breach", 50), 2<<20, daysAgo(2))
	writeAged(t, segmentPath(dir, "quality.breach", 100), 10, daysAgo(1))
	writeAged(t, filepath.Join(dir, "dispatch-logs", "old.log"), 10, daysAgo(60))

	items := planItems(t, planChannels(t, channelOptions(dir, retention("quality.breach", 30, 1))))

	want := map[string]string{
		segmentPath(dir, "quality.breach", 0):          `events.channels["quality.breach"].ttl_days`,
		segmentPath(dir, "quality.breach", 50):         `events.channels["quality.breach"].max_total_mb`,
		filepath.Join(dir, "dispatch-logs", "old.log"): "logs.dispatch.ttl_days",
	}
	for path, rule := range want {
		if items[path].Rule != rule {
			t.Errorf("%s rule = %q, want %q", path, items[path].Rule, rule)
		}
	}
}

func TestPlan_AnEmptyOrAbsentChannelDirectoryPlansNothing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(channelDir(dir, "loop"), 0o755); err != nil {
		t.Fatal(err)
	}

	m := planChannels(t, channelOptions(dir, retention("loop", 1, 1), retention("never-written", 1, 1)))

	if len(m.Items) != 0 || len(m.Warnings) != 0 {
		t.Errorf("manifest = %+v, want nothing for channels without segments", m)
	}
}

func TestPlan_AFileThatIsNotASegmentNameIsNeverPinnedAsTheTail(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeAged(t, segmentPath(dir, "loop", 0), 10, daysAgo(60))
	writeAged(t, filepath.Join(channelDir(dir, "loop"), "seg-bad.ndjson"), 10, daysAgo(60))

	items := planItems(t, planChannels(t, channelOptions(dir, retention("loop", 1, 128))))

	if _, ok := items[segmentPath(dir, "loop", 0)]; ok {
		t.Error("the one real segment is the tail and must stay")
	}
	if items[filepath.Join(channelDir(dir, "loop"), "seg-bad.ndjson")].Action != ActionDelete {
		t.Error("a file with a bad segment name is not the tail and expires by ttl")
	}
}

func TestPlan_TheChannelGlobMatchesTheSegmentsThatTheChannelLogWrites(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	log, err := channel.New(paths.EventChannelsRoot(dir), "loop", channel.Config{SegmentBytes: 1, LockDeadline: 1e9})
	if err != nil {
		t.Fatal(err)
	}
	event := signalcenter.Event{Module: signalcenter.ModuleLoop, Origin: "Test.emit", Kind: signalcenter.KindLoopStarted, Severity: signalcenter.SeverityInfo, Reason: "r"}
	for range 3 {
		if _, err := log.Append([]channel.Record{{Source: "loop", Signal: &event}}); err != nil {
			t.Fatal(err)
		}
	}
	segs, err := log.Segments()
	if err != nil || len(segs) < 3 {
		t.Fatalf("segments = %v, %v, want at least 3", segs, err)
	}
	for _, s := range segs {
		if err := os.Chtimes(s.Path, daysAgo(90), daysAgo(90)); err != nil {
			t.Fatal(err)
		}
	}

	items := planItems(t, planChannels(t, channelOptions(dir, retention("loop", 1, 128))))

	if tail := segs[len(segs)-1].Path; len(items) != len(segs)-1 || hasItem(items, tail) {
		t.Errorf("items = %+v, want the first two real segments deleted and the tail kept", items)
	}
}

func TestPlan_APinRuleThatTheTableDoesNotKnowPinsEveryEntry(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	old := filepath.Join(dir, "x", "a.log")
	writeAged(t, old, 10, daysAgo(60))
	cat := gcpolicy.LogCategory{Name: "x", TTLDays: 1, MaxTotalBytes: 1, Pin: gcpolicy.PinRule(99), Homes: []gcpolicy.LogHome{{Dir: "x", Glob: "*.log"}}}
	var planned []string
	lp := logPlanner{now: nowT0(), add: func(p string, _ Action, _ string) { planned = append(planned, p) }, warn: func(string) {}}

	lp.planCatalog(dir, []gcpolicy.LogCategory{cat})

	if len(planned) != 0 {
		t.Errorf("planned %v, want nothing: an unknown pin rule fails safe", planned)
	}
}

func TestPlan_CollectLooksTheStrategyUpAndNamesNoCategoryKind(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	keep, drop := filepath.Join(dir, "x", "keep.log"), filepath.Join(dir, "x", "drop.log")
	writeAged(t, keep, 10, daysAgo(60))
	writeAged(t, drop, 10, daysAgo(60))
	custom := gcpolicy.PinRule(7)
	cat := gcpolicy.LogCategory{Name: "x", TTLDays: 1, MaxTotalBytes: 1 << 20, Pin: custom, Homes: []gcpolicy.LogHome{{Dir: "x", Glob: "*.log"}}}
	pins := map[gcpolicy.PinRule]pinStrategy{custom: func(in pinInput) map[string]bool {
		out := map[string]bool{}
		for _, e := range in.entries {
			out[e.path] = strings.HasSuffix(e.path, "keep.log")
		}
		return out
	}}
	var planned []string
	lp := logPlanner{now: nowT0(), pins: pins, add: func(p string, _ Action, _ string) { planned = append(planned, p) }, warn: func(string) {}}

	lp.planCatalog(dir, []gcpolicy.LogCategory{cat})

	if len(planned) != 1 || planned[0] != drop {
		t.Errorf("planned %v, want only %s: the registered strategy decides the pin", planned, drop)
	}
}

func TestPlan_TheCurrentOrLiveRuleStillPinsThroughTheTable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	current := filepath.Join(dir, "logs", "20260610T000000Z")
	writeAged(t, filepath.Join(current, gcpolicy.LoopLogName), 10, daysAgo(60))
	ageDir(t, current, daysAgo(60))
	symlink(t, filepath.Base(current), filepath.Join(dir, "logs", gcpolicy.LogsCurrentLink))

	items := planLogs(t, dir, Policy{})

	if _, ok := items[current]; ok {
		t.Errorf("the current run dir is planned %+v, want it pinned", items[current])
	}
}

func hasItem(items map[string]Item, path string) bool {
	_, ok := items[path]
	return ok
}
