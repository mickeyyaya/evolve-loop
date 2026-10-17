package gcpolicy_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

func TestWithDefaults(t *testing.T) {
	got := gcpolicy.Policy{}.WithDefaults()
	want := gcpolicy.Policy{
		Runs:           gcpolicy.RunsPolicy{KeepFull: 10},
		SalvageTTLDays: 30,
		LogsTTLDays:    30,
		TrackerTTLDays: 7,
	}
	if got != want {
		t.Errorf("zero Policy.WithDefaults() = %+v, want %+v", got, want)
	}
	if got.Runs.ArchiveAfterDays != 0 || got.Runs.DeleteAfterDays != 0 {
		t.Errorf("WithDefaults invented a retention horizon: archive=%d delete=%d, want 0/0",
			got.Runs.ArchiveAfterDays, got.Runs.DeleteAfterDays)
	}
}

func TestWithDefaultsPreservesExplicitValues(t *testing.T) {
	in := gcpolicy.Policy{
		Mode:           "enforce",
		Runs:           gcpolicy.RunsPolicy{KeepFull: 3, ArchiveAfterDays: 14, DeleteAfterDays: 60},
		SalvageTTLDays: 1,
		LogsTTLDays:    2,
		TrackerTTLDays: 3,
		Worktrees:      gcpolicy.WorktreesPolicy{KeepRecent: 5, MinAgeMinutes: 90},
	}
	if got := in.WithDefaults(); got != in {
		t.Errorf("WithDefaults() overwrote explicit config: got %+v, want %+v", got, in)
	}
}

func TestPolicyJSONRoundTrip(t *testing.T) {
	raw := `{"mode":"shadow","runs":{"keep_full":4,"archive_after_days":7,"delete_after_days":30},
	         "salvage_ttl_days":11,"logs_ttl_days":12,"tracker_ttl_days":13,
	         "worktrees":{"keep_recent":2,"min_age_minutes":45}}`
	var p gcpolicy.Policy
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := gcpolicy.Policy{
		Mode:           "shadow",
		Runs:           gcpolicy.RunsPolicy{KeepFull: 4, ArchiveAfterDays: 7, DeleteAfterDays: 30},
		SalvageTTLDays: 11,
		LogsTTLDays:    12,
		TrackerTTLDays: 13,
		Worktrees:      gcpolicy.WorktreesPolicy{KeepRecent: 2, MinAgeMinutes: 45},
	}
	if p != want {
		t.Fatalf("decoded %+v, want %+v", p, want)
	}
	out, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back gcpolicy.Policy
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if back != want {
		t.Errorf("round trip lost data: %+v, want %+v", back, want)
	}
}

func TestGoCacheMaxBytes_AnUnsetOrNonPositiveCapTakesTheTwentyGBDefault(t *testing.T) {
	cases := []struct {
		name string
		gb   int
		want int64
	}{
		{"unset", 0, 20e9},
		{"negative", -5, 20e9},
		{"explicit", 35, 35e9},
		{"explicit below the default", 1, 1e9},
	}
	for _, c := range cases {
		if got := (gcpolicy.Policy{GoCacheMaxGB: c.gb}).GoCacheMaxBytes(); got != c.want {
			t.Errorf("%s: GoCacheMaxBytes() with go_cache_max_gb=%d = %d, want %d", c.name, c.gb, got, c.want)
		}
	}
}

func TestGoCacheMaxGB_DecodesFromItsPolicyKey(t *testing.T) {
	var p gcpolicy.Policy
	if err := json.Unmarshal([]byte(`{"go_cache_max_gb":12,"go_cache_ttl_hours":24}`), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.GoCacheMaxGB != 12 || p.GoCacheMaxBytes() != 12e9 {
		t.Errorf("decoded go_cache_max_gb=%d (%d bytes), want 12 (12e9 bytes)", p.GoCacheMaxGB, p.GoCacheMaxBytes())
	}
}

func TestDevQuietPeriod_AppliesADefaultAndAFloorSoAZeroNeverMeansNoGrace(t *testing.T) {
	cases := []struct {
		name    string
		minutes int
		want    time.Duration
	}{
		{"unset", 0, 2 * time.Hour},
		{"negative", -10, 2 * time.Hour},
		{"below the floor", 5, 30 * time.Minute},
		{"at the floor", 30, 30 * time.Minute},
		{"explicit", 600, 10 * time.Hour},
	}
	for _, c := range cases {
		if got := (gcpolicy.WorktreesPolicy{DevQuietMinutes: c.minutes}).DevQuietPeriod(); got != c.want {
			t.Errorf("%s: dev_quiet_minutes=%d DevQuietPeriod() = %s, want %s", c.name, c.minutes, got, c.want)
		}
	}
}

func TestDevQuietMinutes_DecodesBesideMinAgeMinutes(t *testing.T) {
	var p gcpolicy.Policy
	if err := json.Unmarshal([]byte(`{"worktrees":{"min_age_minutes":45,"dev_quiet_minutes":240}}`), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Worktrees.DevQuietMinutes != 240 || p.Worktrees.DevQuietPeriod() != 4*time.Hour {
		t.Errorf("decoded dev_quiet_minutes=%d (%s), want 240 (4h)", p.Worktrees.DevQuietMinutes, p.Worktrees.DevQuietPeriod())
	}
}

func TestDevArchiveAfter_AnUnsetOrNonPositiveHorizonTakesTheFourteenDayDefault(t *testing.T) {
	const day = 24 * time.Hour
	cases := []struct {
		name string
		days int
		want time.Duration
	}{
		{"unset", 0, 14 * day},
		{"negative", -3, 14 * day},
		{"explicit", 30, 30 * day},
		{"one", 1, day},
	}
	for _, c := range cases {
		if got := (gcpolicy.DevWorktreesPolicy{ArchiveAfterDays: c.days}).ArchiveAfter(); got != c.want {
			t.Errorf("%s: archive_after_days=%d ArchiveAfter() = %s, want %s", c.name, c.days, got, c.want)
		}
	}
}

func TestDevWorktreesArchiveAfterDays_DecodesFromItsPolicyKeyBesideWorktrees(t *testing.T) {
	var p gcpolicy.Policy
	raw := `{"worktrees":{"min_age_minutes":45},"dev_worktrees":{"archive_after_days":21}}`
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.DevWorktrees.ArchiveAfterDays != 21 || p.Worktrees.MinAgeMinutes != 45 {
		t.Errorf("decoded %+v, want dev_worktrees.archive_after_days=21 and worktrees.min_age_minutes=45", p)
	}
	out, err := json.Marshal(p)
	if err != nil || !strings.Contains(string(out), `"dev_worktrees":{"archive_after_days":21}`) {
		t.Errorf("re-marshal = %s (err %v), want the dev_worktrees key kept", out, err)
	}
}

func TestDevSweepBudget_AnUnsetOrNonPositiveValueTakesTheFiveMinuteDefault(t *testing.T) {
	cases := []struct {
		name string
		secs int
		want time.Duration
	}{
		{"unset", 0, 5 * time.Minute},
		{"negative", -9, 5 * time.Minute},
		{"explicit", 90, 90 * time.Second},
	}
	for _, c := range cases {
		if got := (gcpolicy.DevWorktreesPolicy{SweepBudgetSeconds: c.secs}).SweepBudget(); got != c.want {
			t.Errorf("%s: sweep_budget_seconds=%d SweepBudget() = %s, want %s", c.name, c.secs, got, c.want)
		}
	}
}

func TestDevSweepBudgetSeconds_DecodesFromItsPolicyKey(t *testing.T) {
	var p gcpolicy.Policy
	if err := json.Unmarshal([]byte(`{"dev_worktrees":{"sweep_budget_seconds":120}}`), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := p.DevWorktrees.SweepBudget(); got != 2*time.Minute {
		t.Errorf("SweepBudget() = %s, want 2m0s", got)
	}
}

func TestTestCacheBounds_UnsetOrNonPositiveTakeTheCompiledDefaults(t *testing.T) {
	cases := []struct {
		name    string
		days    int
		mb      int
		wantTTL time.Duration
		wantMax int64
	}{
		{"unset", 0, 0, 14 * 24 * time.Hour, 256 * 1024 * 1024},
		{"negative", -1, -1, 14 * 24 * time.Hour, 256 * 1024 * 1024},
		{"explicit", 3, 8, 3 * 24 * time.Hour, 8 * 1024 * 1024},
	}
	for _, c := range cases {
		p := gcpolicy.Policy{TestCacheTTLDays: c.days, TestCacheMaxMB: c.mb}
		if got := p.TestCacheTTL(); got != c.wantTTL {
			t.Errorf("%s: TestCacheTTL() = %v, want %v", c.name, got, c.wantTTL)
		}
		if got := p.TestCacheMaxBytes(); got != c.wantMax {
			t.Errorf("%s: TestCacheMaxBytes() = %d, want %d", c.name, got, c.wantMax)
		}
	}
}

func TestTestCacheBounds_DecodeFromTheirPolicyKeys(t *testing.T) {
	var p gcpolicy.Policy
	if err := json.Unmarshal([]byte(`{"testcache_ttl_days":5,"testcache_max_mb":64}`), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.TestCacheTTLDays != 5 || p.TestCacheMaxMB != 64 {
		t.Errorf("decoded %+v", p)
	}
}

func TestCacheTrimFreeBytes_AnUnsetOrNonPositiveFloorIsOff(t *testing.T) {
	cases := []struct {
		name string
		gb   int
		want uint64
	}{
		{"unset", 0, 0},
		{"negative", -3, 0},
		{"explicit", 50, 50e9},
	}
	for _, c := range cases {
		if got := (gcpolicy.Policy{CacheTrimFreeGB: c.gb}).CacheTrimFreeBytes(); got != c.want {
			t.Errorf("%s: CacheTrimFreeBytes() with cache_trim_free_gb=%d = %d, want %d", c.name, c.gb, got, c.want)
		}
	}
}
