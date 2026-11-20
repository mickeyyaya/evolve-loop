package hostpressure

import (
	"math"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

func testConfig() Config {
	return Config{
		LoadHigh: 1.5, LoadLow: 1.0, MemMinPct: 15, SwapMaxMB: 4096, DecreaseFactor: 0.5, Step: 1,
		SmoothSamples: 1, Floor: 4, Ceiling: 12, Cooldown: 60 * time.Second, IncreaseCooldown: 180 * time.Second,
	}
}

func calm() Reading  { return Reading{Load1: 7, CPUs: 14, MemAvailablePct: 80} }
func heavy() Reading { return Reading{Load1: 28, CPUs: 14, MemAvailablePct: 80} }

func TestDecide_HighLoadHalvesTheCapDownToTheFloor(t *testing.T) {
	cfg := testConfig()
	s := NewState(cfg.Ceiling)
	now := t0
	want := []int{6, 4, 4}
	for i, w := range want {
		var d Decision
		s, d = Decide(s, heavy(), true, cfg, now)
		if s.Cap != w || d.New != w {
			t.Fatalf("step %d: cap = %d (decision %d), want %d", i, s.Cap, d.New, w)
		}
		now = now.Add(cfg.Cooldown)
	}
}

func TestDecide_DecreaseNamesCauseOldAndNew(t *testing.T) {
	cfg := testConfig()
	_, d := Decide(NewState(12), heavy(), true, cfg, t0)
	if d.Old != 12 || d.New != 6 || d.Cause != CausePressureHigh || d.Score != 2.0/1.5*1 {
		t.Fatalf("decision = %+v", d)
	}
}

func TestDecide_LowPressureRaisesOneStepPerIncreaseCooldown(t *testing.T) {
	cfg := testConfig()
	s := NewState(cfg.Ceiling)
	s.Cap = cfg.Floor
	now := t0
	for want := 5; want <= cfg.Ceiling; want++ {
		var d Decision
		s, d = Decide(s, calm(), true, cfg, now)
		if s.Cap != want || d.Cause != CausePressureLow {
			t.Fatalf("cap = %d cause %q, want %d pressure_low", s.Cap, d.Cause, want)
		}
		_, held := Decide(s, calm(), true, cfg, now.Add(cfg.IncreaseCooldown-time.Second))
		if held.New != s.Cap || held.Cause != CauseHold {
			t.Fatalf("inside the increase cooldown: %+v, want hold", held)
		}
		now = now.Add(cfg.IncreaseCooldown)
	}
	if s.Cap != cfg.Ceiling {
		t.Fatalf("cap = %d, want the ceiling %d after 8 increase cooldowns", s.Cap, cfg.Ceiling)
	}
	if _, d := Decide(s, calm(), true, cfg, now); d.New != cfg.Ceiling || d.Cause != CauseHold {
		t.Fatalf("at the ceiling: %+v, want hold", d)
	}
}

func TestDecide_AlternatingSamplesInsideTheCooldownCauseNoChange(t *testing.T) {
	cfg := testConfig()
	s, _ := Decide(NewState(12), heavy(), true, cfg, t0)
	now := t0
	for i := range 6 {
		now = now.Add(5 * time.Second)
		r := calm()
		if i%2 == 0 {
			r = heavy()
		}
		var d Decision
		s, d = Decide(s, r, true, cfg, now)
		if s.Cap != 6 || d.Cause != CauseHold {
			t.Fatalf("sample %d: cap %d cause %q, want hold at 6", i, s.Cap, d.Cause)
		}
	}
}

func TestDecide_DeadBandBetweenLowAndHighHolds(t *testing.T) {
	cfg := testConfig()
	mid := Reading{Load1: 17.5, CPUs: 14, MemAvailablePct: 80}
	s := NewState(8)
	if _, d := Decide(s, mid, true, cfg, t0); d.New != 8 || d.Cause != CauseHold {
		t.Fatalf("decision = %+v, want hold", d)
	}
}

func TestDecide_MemoryBelowMinShrinksEvenWhenLoadIsLow(t *testing.T) {
	cfg := testConfig()
	low := Reading{Load1: 1, CPUs: 14, MemAvailablePct: 7.5}
	_, d := Decide(NewState(12), low, true, cfg, t0)
	if d.New != 6 || d.Cause != CausePressureHigh || d.Score != 2 {
		t.Fatalf("decision = %+v, want a halving on a score of 2", d)
	}
}

func TestDecide_SwapAboveMaxShrinks(t *testing.T) {
	cfg := testConfig()
	swap := Reading{Load1: 1, CPUs: 14, MemAvailablePct: 80, SwapUsedMB: 8192}
	if _, d := Decide(NewState(12), swap, true, cfg, t0); d.New != 6 || d.Score != 2 {
		t.Fatalf("decision = %+v, want a halving on a score of 2", d)
	}
}

func TestDecide_ZeroMemoryAvailableScoresInfinite(t *testing.T) {
	r := Reading{Load1: 1, CPUs: 14, MemAvailablePct: 0}
	if got := Score(r, testConfig()); !math.IsInf(got, 1) {
		t.Fatalf("Score = %v, want +Inf", got)
	}
}

func TestDecide_ZeroCPUsCountsAsOne(t *testing.T) {
	r := Reading{Load1: 3, CPUs: 0, MemAvailablePct: 100}
	if got := Score(r, testConfig()); got != 2 {
		t.Fatalf("Score = %v, want 2", got)
	}
}

func TestDecide_FailedReadHoldsTheCapAndBlindFiresOnceAtTheStreak(t *testing.T) {
	cfg := testConfig()
	cfg.SmoothSamples = 3
	s := NewState(9)
	blind := 0
	for i := range 5 {
		var d Decision
		s, d = Decide(s, Reading{}, false, cfg, t0.Add(time.Duration(i)*time.Hour))
		if s.Cap != 9 || d.New != 9 || d.Cause != CauseBlind {
			t.Fatalf("read %d: cap %d decision %+v, want hold at 9", i, s.Cap, d)
		}
		if d.Blind {
			blind++
			if i != 2 {
				t.Errorf("Blind fired at read %d, want read 2 (the third)", i)
			}
		}
	}
	if blind != 1 {
		t.Fatalf("Blind fired %d times, want once", blind)
	}
}

func TestDecide_AGoodReadResetsTheBlindStreak(t *testing.T) {
	cfg := testConfig()
	s := NewState(9)
	s, _ = Decide(s, Reading{}, false, cfg, t0)
	s, _ = Decide(s, calm(), true, cfg, t0)
	if s.Blind != 0 {
		t.Fatalf("Blind = %d, want 0 after a good read", s.Blind)
	}
}

func TestDecide_SmoothingNeedsSustainedPressureAndKeepsOnlyTheLastSamples(t *testing.T) {
	cfg := testConfig()
	cfg.SmoothSamples = 3
	s := NewState(12)
	now := t0
	for i, r := range []Reading{calm(), calm(), heavy()} {
		s, _ = Decide(s, r, true, cfg, now)
		if s.Cap != 12 {
			t.Fatalf("sample %d: cap = %d, want 12: one heavy sample among calm ones does not shrink", i, s.Cap)
		}
		now = now.Add(cfg.Cooldown)
	}
	for range 3 {
		s, _ = Decide(s, heavy(), true, cfg, now)
		now = now.Add(cfg.Cooldown)
	}
	if s.Cap >= 12 {
		t.Fatalf("cap = %d, want a decrease after sustained heavy samples", s.Cap)
	}
	if len(s.Scores) != 3 {
		t.Fatalf("kept %d scores, want 3", len(s.Scores))
	}
}

func TestDecide_DoesNotMutateItsInputState(t *testing.T) {
	cfg := testConfig()
	s := NewState(12)
	before := s
	Decide(s, heavy(), true, cfg, t0)
	if s.Cap != before.Cap || len(s.Scores) != 0 {
		t.Fatalf("input state changed: %+v", s)
	}
}

func TestDecide_ExactlyAtTheHighThresholdHolds(t *testing.T) {
	cfg := testConfig()
	at := Reading{Load1: cfg.LoadHigh * 14, CPUs: 14, MemAvailablePct: 80}
	if got := Score(at, cfg); got != 1 {
		t.Fatalf("fixture: Score = %v, want exactly 1", got)
	}
	if _, d := Decide(NewState(12), at, true, cfg, t0); d.New != 12 || d.Cause != CauseHold {
		t.Fatalf("decision = %+v, want hold: only a mean above 1 shrinks", d)
	}
}

func TestDecide_ExactlyAtTheLowThresholdHolds(t *testing.T) {
	cfg := testConfig()
	at := Reading{Load1: cfg.LoadLow * 14, CPUs: 14, MemAvailablePct: 80}
	if got := Score(at, cfg); got != cfg.LoadLow/cfg.LoadHigh {
		t.Fatalf("fixture: Score = %v, want exactly %v", got, cfg.LoadLow/cfg.LoadHigh)
	}
	s := NewState(12)
	s.Cap = cfg.Floor
	if _, d := Decide(s, at, true, cfg, t0); d.New != cfg.Floor || d.Cause != CauseHold {
		t.Fatalf("decision = %+v, want hold: only a mean under low over high raises", d)
	}
}

func TestReading_LoadPerCoreCountsZeroCPUsAsOne(t *testing.T) {
	if got := (Reading{Load1: 6, CPUs: 3}).LoadPerCore(); got != 2 {
		t.Errorf("LoadPerCore = %v, want 2", got)
	}
	if got := (Reading{Load1: 6}).LoadPerCore(); got != 6 {
		t.Errorf("LoadPerCore with no CPUs = %v, want 6", got)
	}
}

func TestState_AFreshStateStartsAtTheCeilingAndAChangeStampsTheClock(t *testing.T) {
	cfg := testConfig()
	fresh := NewState(cfg.Ceiling)
	if fresh.Cap != cfg.Ceiling || fresh.Blind != 0 || len(fresh.Scores) != 0 {
		t.Fatalf("NewState = %+v, want cap %d with no blind streak and no scores", fresh, cfg.Ceiling)
	}
	var next State = fresh
	next, d := Decide(next, heavy(), true, cfg, t0)
	if next.Cap != d.New || !next.Changed.Equal(t0) {
		t.Errorf("state after a cut = %+v, want cap %d changed at %v", next, d.New, t0)
	}
}
