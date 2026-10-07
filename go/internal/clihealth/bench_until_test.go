package clihealth

import (
	"testing"
	"time"
)

func TestBenchWallUntil_BenchesUntilTheStatedResetAndAccumulatesStrikes(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	s := NewStore(t.TempDir(), func() time.Time { return now })
	evidence := "agy: CLAUDE AND GPT MODELS 5h 100% used (resets in 2h 15m)"
	reset := now.Add(2*time.Hour + 15*time.Minute)

	first, err := s.BenchWallUntil("agy-claude", Wall{Pattern: "usage_probe", Evidence: evidence, Reset: reset})
	if err != nil {
		t.Fatal(err)
	}
	if want := reset.Add(resetMargin); !first.BenchedUntil.Equal(want) {
		t.Errorf("BenchedUntil = %v, want the reset plus the reset margin, %v", first.BenchedUntil, want)
	}
	if first.Family != "agy-claude" || first.Reason != "usage_probe" || first.Evidence != evidence || first.Strikes != 1 {
		t.Errorf("entry = %+v", first)
	}
	if _, active := s.Active()["agy-claude"]; !active {
		t.Errorf("the bench is not active for routing")
	}
	second, _ := s.BenchWallUntil("agy-claude", Wall{Pattern: "usage_probe", Evidence: evidence, Reset: now.Add(time.Hour)})
	if second.Strikes != 2 {
		t.Errorf("strikes = %d after a re-bench, want 2", second.Strikes)
	}
}

func TestBenchWallUntil_CapsAFarResetAndFallsBackToTheCooldownWithoutOne(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	s := NewStore(t.TempDir(), func() time.Time { return now })

	far, _ := s.BenchWallUntil("claude", Wall{Pattern: "usage_probe", Evidence: "claude: all models week 100% used", Reset: now.Add(140 * time.Hour)})
	if want := now.Add(resetHintCap); !far.BenchedUntil.Equal(want) {
		t.Errorf("a reset 140h out benched until %v, want the %v cap, %v", far.BenchedUntil, resetHintCap, want)
	}
	for name, until := range map[string]time.Time{"no reset": {}, "a past reset": now.Add(-time.Hour)} {
		e, _ := s.BenchWallUntil("agy-"+name, Wall{Pattern: "usage_probe", Evidence: "drained", Reset: until})
		if want := now.Add(CooldownForStrikes(1)); !e.BenchedUntil.Equal(want) {
			t.Errorf("%s benched until %v, want the first-strike cooldown, %v", name, e.BenchedUntil, want)
		}
	}
}

func TestBenchWallUntil_TheNewestReadingsResetReplacesTheBenchEvenWhenItIsEarlier(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	s := NewStore(t.TempDir(), func() time.Time { return now })
	if _, err := s.BenchWallUntil("agy-claude", Wall{Pattern: "usage_probe", Evidence: "5h window 100% used", Reset: now.Add(3*time.Hour + 7*time.Minute)}); err != nil {
		t.Fatal(err)
	}
	earlier := now.Add(47 * time.Minute)

	e, err := s.BenchWallUntil("agy-claude", Wall{Pattern: "usage_probe", Evidence: "5h window 100% used", Reset: earlier})

	if err != nil || !e.BenchedUntil.Equal(earlier.Add(resetMargin)) {
		t.Fatalf("benched until %v (err %v); the newest usage screen states when the window reopens, so its earlier reset replaces the older bench, as NewBenchEntry's newest wall does", e.BenchedUntil, err)
	}
}
