package gcpolicy_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

func TestSentinel_DefaultsWhenTheBlockIsAbsent(t *testing.T) {
	t.Parallel()
	s := gcpolicy.Policy{}.Sentinel
	checks := []struct {
		name      string
		got, want any
	}{
		{"interval", s.Interval(), 60 * time.Second},
		{"repeat", s.Repeat(), 900 * time.Second},
		{"orphan servers", s.OrphanServersLimit(), 20},
		{"orphan processes", s.OrphanProcessesLimit(), 20},
		{"hold max", s.HoldMaxDuration(), 7200 * time.Second},
		{"release budget", s.ReleaseBudget(), 300 * time.Second},
		{"no progress", s.NoProgress(), int64(1 << 30)},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestSentinel_ExplicitValuesWin(t *testing.T) {
	t.Parallel()
	s := gcpolicy.SentinelPolicy{IntervalS: 5, RepeatS: 6, OrphanServersMax: 7, OrphanProcessesMax: 8, HoldMaxS: 9, ReleaseBudgetS: 10, NoProgressBytes: 11}
	if s.Interval() != 5*time.Second || s.Repeat() != 6*time.Second || s.OrphanServersLimit() != 7 || s.OrphanProcessesLimit() != 8 ||
		s.HoldMaxDuration() != 9*time.Second || s.ReleaseBudget() != 10*time.Second || s.NoProgress() != 11 {
		t.Fatalf("explicit values lost: %+v", s)
	}
}

func TestSentinel_BothStagesDefaultToShadowAndOnlyEnforceEnforces(t *testing.T) {
	t.Parallel()
	var p gcpolicy.Policy
	if p.Sentinel.HoldEnforced() || p.Sentinel.ReleaseEnforced() {
		t.Fatal("an absent block enforces")
	}
	p.Sentinel.Hold.Stage = "bogus"
	p.Sentinel.Release.Stage = "bogus"
	if p.Sentinel.HoldEnforced() || p.Sentinel.ReleaseEnforced() {
		t.Fatal("an unknown stage enforces")
	}
	p.Sentinel.Hold.Stage = gcpolicy.SentinelStageEnforce
	if !p.Sentinel.HoldEnforced() || p.Sentinel.ReleaseEnforced() {
		t.Fatal("the hold stage did not act alone")
	}
	p.Sentinel.Release.Stage = gcpolicy.SentinelStageEnforce
	if !p.Sentinel.ReleaseEnforced() {
		t.Fatal("the release stage did not enforce")
	}
}

func TestSentinel_DecodesFromItsPolicyKeys(t *testing.T) {
	t.Parallel()
	raw := `{"sentinel":{"interval_s":30,"repeat_s":60,"orphan_servers_max":3,"orphan_processes_max":4,"hold_max_s":5,"release_budget_s":6,"no_progress_bytes":7,"hold":{"stage":"enforce"},"release":{"stage":"shadow"}}}`
	var p gcpolicy.Policy
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatal(err)
	}
	want := gcpolicy.SentinelPolicy{IntervalS: 30, RepeatS: 60, OrphanServersMax: 3, OrphanProcessesMax: 4, HoldMaxS: 5, ReleaseBudgetS: 6, NoProgressBytes: 7,
		Hold: gcpolicy.SentinelStageBlock{Stage: gcpolicy.SentinelStageEnforce}, Release: gcpolicy.SentinelStageBlock{Stage: gcpolicy.SentinelStageShadow}}
	if p.Sentinel != want {
		t.Fatalf("decoded %+v, want %+v", p.Sentinel, want)
	}
}

func TestSentinelStage_OnlyTheEnforceValueEnforces(t *testing.T) {
	t.Parallel()
	cases := map[gcpolicy.SentinelStage]bool{
		gcpolicy.SentinelStageShadow:  false,
		gcpolicy.SentinelStageEnforce: true,
		"":                            false,
		"enforced":                    false,
	}
	for stage, want := range cases {
		s := gcpolicy.SentinelPolicy{Hold: gcpolicy.SentinelStageBlock{Stage: stage}, Release: gcpolicy.SentinelStageBlock{Stage: stage}}
		if s.HoldEnforced() != want || s.ReleaseEnforced() != want {
			t.Errorf("stage %q: hold %v release %v, want %v", stage, s.HoldEnforced(), s.ReleaseEnforced(), want)
		}
	}
}
