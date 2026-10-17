package looppreflight

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type releaseProbe struct {
	calls  int
	latest string
	err    error
}

func (p *releaseProbe) read(bin string) (string, error) {
	p.calls++
	return p.latest, p.err
}

func releaseLagOptions(t *testing.T, installed string, probe *releaseProbe) Options {
	t.Helper()
	opts := goodPipelineOptions(t)
	opts.VersionInventory = func() map[string]string {
		if installed == "" {
			return map[string]string{}
		}
		return map[string]string{"claude": installed}
	}
	opts.LatestRelease = probe.read
	opts.ReleaseLag = ReleaseLagConfig{Threshold: 2, CacheTTL: 24 * time.Hour}
	return opts
}

func runReleaseLag(t *testing.T, opts Options) CheckResult {
	t.Helper()
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.Halted() {
		t.Fatalf("release lag must never halt; OverallLevel=%s", r.OverallLevel)
	}
	return findCheck(t, r, "claude-release-lag")
}

func TestReleaseLag_Verdicts(t *testing.T) {
	cases := []struct {
		name      string
		installed string
		latest    string
		readErr   error
		wantLevel CheckLevel
		wantIn    []string
	}{
		{"patches behind past the threshold warns", "2.1.283", "2.1.286", nil, LevelWarn,
			[]string{"2.1.283", "2.1.286", "claude update", "evolve doctor live claude-tmux"}},
		{"at the threshold is silent", "2.1.283", "2.1.285", nil, LevelPass, nil},
		{"current is silent", "2.1.286", "2.1.286", nil, LevelPass, nil},
		{"ahead of the registry is silent", "2.1.290", "2.1.286", nil, LevelPass, nil},
		{"a newer minor warns whatever the patch", "2.1.283", "2.2.0", nil, LevelWarn, []string{"2.2.0"}},
		{"a lookup error is a quiet note", "2.1.283", "", errors.New("dial tcp: timeout"), LevelPass,
			[]string{"release lookup failed", "dial tcp: timeout"}},
		{"an unreadable installed version is a quiet note", "", "2.1.286", nil, LevelPass,
			[]string{"installed version unknown"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			probe := &releaseProbe{latest: tc.latest, err: tc.readErr}
			c := runReleaseLag(t, releaseLagOptions(t, tc.installed, probe))
			if c.Level != tc.wantLevel {
				t.Fatalf("level = %s (%s | %s), want %s", c.Level, c.Message, c.Detail, tc.wantLevel)
			}
			text := c.Message + "\n" + c.Detail
			for _, want := range tc.wantIn {
				if !strings.Contains(text, want) {
					t.Errorf("check text %q missing %q", text, want)
				}
			}
		})
	}
}

func TestReleaseLag_FreshCacheMakesNoLookupAndStaysSilent(t *testing.T) {
	probe := &releaseProbe{latest: "2.1.290"}
	opts := releaseLagOptions(t, "2.1.283", probe)

	first := runReleaseLag(t, opts)
	second := runReleaseLag(t, opts)

	if first.Level != LevelWarn {
		t.Fatalf("first run = %s, want warn", first.Level)
	}
	if second.Level != LevelPass {
		t.Fatalf("second run inside the window = %s, want silent pass (one report per window)", second.Level)
	}
	if probe.calls != 1 {
		t.Fatalf("reader calls = %d, want 1: a fresh cache must not hit the network", probe.calls)
	}
}

func TestReleaseLag_StaleCacheLooksUpAgain(t *testing.T) {
	probe := &releaseProbe{latest: "2.1.290"}
	opts := releaseLagOptions(t, "2.1.283", probe)
	now := fixedNow()
	opts.Now = func() time.Time { return now }

	runReleaseLag(t, opts)
	now = now.Add(25 * time.Hour)
	c := runReleaseLag(t, opts)

	if probe.calls != 2 {
		t.Fatalf("reader calls = %d, want 2 after the TTL elapsed", probe.calls)
	}
	if c.Level != LevelWarn {
		t.Fatalf("level after the window = %s, want warn again", c.Level)
	}
}

func TestReleaseLag_AFailedLookupIsCachedSoItCannotStormOrDelay(t *testing.T) {
	probe := &releaseProbe{err: errors.New("offline")}
	opts := releaseLagOptions(t, "2.1.283", probe)

	runReleaseLag(t, opts)
	c := runReleaseLag(t, opts)

	if probe.calls != 1 {
		t.Fatalf("reader calls = %d, want 1: a failure must not retry inside the window", probe.calls)
	}
	if c.Level != LevelPass {
		t.Fatalf("level = %s, want pass", c.Level)
	}
}

func TestReleaseLag_CacheFileLivesUnderTheEvolveDir(t *testing.T) {
	probe := &releaseProbe{latest: "2.1.290"}
	opts := releaseLagOptions(t, "2.1.283", probe)

	runReleaseLag(t, opts)

	if _, err := os.Stat(filepath.Join(opts.EvolveDir, "release-lag-cache.json")); err != nil {
		t.Fatalf("cache file missing: %v", err)
	}
}

func TestReleaseLag_NoReaderWiredIsASilentPass(t *testing.T) {
	probe := &releaseProbe{latest: "2.1.290"}
	opts := releaseLagOptions(t, "2.1.283", probe)
	opts.LatestRelease = nil

	c := runReleaseLag(t, opts)

	if c.Level != LevelPass || probe.calls != 0 {
		t.Fatalf("level=%s calls=%d, want pass and no lookup", c.Level, probe.calls)
	}
}

func TestReleaseLag_DefaultsApplyWhenConfigIsZero(t *testing.T) {
	probe := &releaseProbe{latest: "2.1.286"}
	opts := releaseLagOptions(t, "2.1.283", probe)
	opts.ReleaseLag = ReleaseLagConfig{}

	c := runReleaseLag(t, opts)

	if c.Level != LevelWarn {
		t.Fatalf("level = %s, want warn under the policy default threshold", c.Level)
	}
}

func TestIsLaggingBy(t *testing.T) {
	t.Parallel()
	cases := []struct {
		installed, latest string
		threshold         int
		want              bool
	}{
		{"2.1.283", "2.1.286", 2, true},
		{"2.1.283", "2.1.285", 2, false},
		{"2.1.283", "2.2.0", 2, true},
		{"1.9.9", "2.0.0", 2, true},
		{"2.1.283", "2.1.283", 2, false},
		{"2.1.290", "2.1.283", 2, false},
		{"2.1", "2.1.4", 2, true},
	}
	for _, c := range cases {
		if got := IsLaggingBy(c.installed, c.latest, c.threshold); got != c.want {
			t.Errorf("IsLaggingBy(%q,%q,%d) = %v, want %v", c.installed, c.latest, c.threshold, got, c.want)
		}
	}
}

func TestReleaseLag_CorruptCacheIsIgnoredAndRewritten(t *testing.T) {
	probe := &releaseProbe{latest: "2.1.290"}
	opts := releaseLagOptions(t, "2.1.283", probe)
	path := filepath.Join(opts.EvolveDir, "release-lag-cache.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := runReleaseLag(t, opts)

	if c.Level != LevelWarn || probe.calls != 1 {
		t.Fatalf("level=%s calls=%d, want warn after one lookup", c.Level, probe.calls)
	}
	if data, _ := os.ReadFile(path); !strings.Contains(string(data), "checked_at") {
		t.Fatalf("cache not rewritten: %q", data)
	}
}

func TestReleaseLag_AnUnwritableCacheIsANoteNeverAFailure(t *testing.T) {
	probe := &releaseProbe{latest: "2.1.290"}
	opts := releaseLagOptions(t, "2.1.283", probe)
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	opts.EvolveDir = filepath.Join(blocker, "evolve")

	c := runReleaseLag(t, opts)

	if c.Level != LevelWarn || !strings.Contains(c.Detail, "release cache not saved") {
		t.Fatalf("level=%s detail=%q, want warn noting the unsaved cache", c.Level, c.Detail)
	}
}

func TestIsLagging_EdgeCases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		installed, latest string
		want              bool
	}{
		{"2.1.3", "unavailable", false},
		{"2.1.3", "", false},
		{"2.2.0", "2.1.9", false},
		{"v2.1.3", "2.1.4-beta.1", true},
		{"2.1.3", "2.1.3.1", true},
	}
	for _, c := range cases {
		if got := IsLagging(c.installed, c.latest); got != c.want {
			t.Errorf("IsLagging(%q,%q) = %v, want %v", c.installed, c.latest, got, c.want)
		}
	}
}
