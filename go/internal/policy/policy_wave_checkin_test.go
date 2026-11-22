package policy

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func defaultCheckin() WaveCheckinConfig {
	return WaveCheckinConfig{
		StallMinAgeMinutes: 120, StallMinLiveLanes: 4, MinShipsPerHour: 0.25,
		AbnormalMinCycles: 3, AbnormalMinEvents: 15, StallLines: 3, LoadSustainedMinutes: 15,
		StallWindowMinutes: 60, LoadPerCoreLimit: 3.5, DiskFloorBytes: 10_000_000_000,
	}
}

func TestWaveCheckinSettings_DefaultsAndEachKeyOverrides(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, json string
		edit       func(*WaveCheckinConfig)
	}{
		{"absent block is the compiled default", `{}`, func(*WaveCheckinConfig) {}},
		{"empty block is the compiled default", `{"wave":{"checkin":{}}}`, func(*WaveCheckinConfig) {}},
		{"non-positive values fall back", `{"wave":{"checkin":{"stall_lines":0,"load_sustained_minutes":-1,"min_ships_per_hour":-2}}}`, func(*WaveCheckinConfig) {}},
		{"stall_min_age_minutes", `{"wave":{"checkin":{"stall_min_age_minutes":30}}}`, func(c *WaveCheckinConfig) { c.StallMinAgeMinutes = 30 }},
		{"stall_min_live_lanes", `{"wave":{"checkin":{"stall_min_live_lanes":6}}}`, func(c *WaveCheckinConfig) { c.StallMinLiveLanes = 6 }},
		{"min_ships_per_hour", `{"wave":{"checkin":{"min_ships_per_hour":0.5}}}`, func(c *WaveCheckinConfig) { c.MinShipsPerHour = 0.5 }},
		{"abnormal_min_cycles", `{"wave":{"checkin":{"abnormal_min_cycles":5}}}`, func(c *WaveCheckinConfig) { c.AbnormalMinCycles = 5 }},
		{"abnormal_min_events", `{"wave":{"checkin":{"abnormal_min_events":40}}}`, func(c *WaveCheckinConfig) { c.AbnormalMinEvents = 40 }},
		{"stall_lines", `{"wave":{"checkin":{"stall_lines":9}}}`, func(c *WaveCheckinConfig) { c.StallLines = 9 }},
		{"load_sustained_minutes", `{"wave":{"checkin":{"load_sustained_minutes":30}}}`, func(c *WaveCheckinConfig) { c.LoadSustainedMinutes = 30 }},
		{"stall_window_minutes", `{"wave":{"checkin":{"stall_window_minutes":90}}}`, func(c *WaveCheckinConfig) { c.StallWindowMinutes = 90 }},
		{"load_per_core_limit", `{"wave":{"checkin":{"load_per_core_limit":5.5}}}`, func(c *WaveCheckinConfig) { c.LoadPerCoreLimit = 5.5 }},
		{"the disk floor is read from gc", `{"gc":{"disk_floor_gb":25}}`, func(c *WaveCheckinConfig) { c.DiskFloorBytes = 25_000_000_000 }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			want := defaultCheckin()
			c.edit(&want)
			if got := loadCIWatchPolicy(t, c.json).WaveCheckinSettings(); got != want {
				t.Errorf("WaveCheckinSettings() = %+v; want %+v", got, want)
			}
		})
	}
}

func TestWaveCheckinSettings_ALiteralBlockResolves(t *testing.T) {
	t.Parallel()
	got := (Policy{Wave: &WavePolicy{Checkin: &WaveCheckinPolicy{StallLines: 7}}}).WaveCheckinSettings()
	want := defaultCheckin()
	want.StallLines = 7
	if got != want {
		t.Errorf("WaveCheckinSettings() = %+v; want %+v", got, want)
	}
}

func TestWaveCheckinPolicy_AnUnknownKeyIsRefused(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"wave":{"checkin":{"stall_line":3}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("a misspelled wave.checkin key must fail with ErrUnknownKey, got %v", err)
	}
}
