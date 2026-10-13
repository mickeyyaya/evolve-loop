package router

import (
	"encoding/json"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func TestPromoteUnifiedCommitmentTier_RaisesOnlyPlanningPhasesBelowDeep(t *testing.T) {
	entries := func() *PhasePlan {
		return &PhasePlan{Entries: []PhasePlanEntry{
			{Phase: "plan-review", Run: true, Tier: "balanced"},
			{Phase: "build-planner", Run: true, Tier: "fast"},
			{Phase: "build-planner", Run: true, Tier: "deep"},
			{Phase: "build", Run: true, Tier: "fast"},
			{Phase: "plan-review", Run: false, Tier: ""},
		}}
	}
	signals := RoutingSignals{Triage: TriageSignals{Present: true, UnifiedSize: "small"}}

	plan := entries()
	clamps := promoteUnifiedCommitmentTier(RouteInput{Signals: signals}, plan)

	wantTiers := []string{"deep", "deep", "deep", "fast", "deep"}
	for i, want := range wantTiers {
		if plan.Entries[i].Tier != want {
			t.Errorf("entry %d (%s) tier = %q, want %q", i, plan.Entries[i].Phase, plan.Entries[i].Tier, want)
		}
	}
	want := []Clamp{
		{Phase: "plan-review", Rule: "unified-commitment-planning-tier", Proposed: "plan-review=tier:balanced", Forced: "plan-review=tier:deep"},
		{Phase: "build-planner", Rule: "unified-commitment-planning-tier", Proposed: "build-planner=tier:fast", Forced: "build-planner=tier:deep"},
		{Phase: "plan-review", Rule: "unified-commitment-planning-tier", Proposed: "plan-review=tier:", Forced: "plan-review=tier:deep"},
	}
	if len(clamps) != len(want) {
		t.Fatalf("clamps = %+v, want %+v", clamps, want)
	}
	for i := range want {
		if clamps[i] != want[i] {
			t.Errorf("clamp %d = %+v, want %+v", i, clamps[i], want[i])
		}
	}

	untouched := entries()
	if got := promoteUnifiedCommitmentTier(RouteInput{}, untouched); got != nil {
		t.Errorf("without a unified commitment nothing is promoted: %+v", got)
	}
	if untouched.Entries[0].Tier != "balanced" || untouched.Entries[1].Tier != "fast" {
		t.Errorf("tiers changed without a unified commitment: %+v", untouched.Entries)
	}
}

func TestExtractScoutAndTriage_NonJSONYieldsAbsentSignals(t *testing.T) {
	for _, raw := range []string{"", "not json", "[1,2", `"a string"`} {
		if got := extractScout([]byte(raw)); got.Present {
			t.Errorf("extractScout(%q) = %+v, want absent", raw, got)
		}
		if got := extractTriage([]byte(raw)); got.Present {
			t.Errorf("extractTriage(%q) = %+v, want absent", raw, got)
		}
	}
}

func TestExtractScout_ReadsEstimateCountsAndItemKeys(t *testing.T) {
	raw := `{"cycle_size_estimate":"small","goal_type":"bugfix","deliverable_kind":"document","carryover_count":2,"backlog_size":9,"run_dir.artifact_bytes":123,"item1_scope":"a","item22":"b","items":"x","item":"y"}`

	got := extractScout([]byte(raw))

	want := ScoutSignals{Present: true, CycleSizeEstimate: "small", GoalType: "bugfix", DeliverableKind: DeliverableKindDocument, CarryoverCount: 2, BacklogSize: 9, ArtifactBytes: 123, ItemCount: 2}
	if got != want {
		t.Fatalf("extractScout = %+v, want %+v", got, want)
	}
}

func TestExtractTriage_PrefersCycleSizeOverScoutEstimate(t *testing.T) {
	cases := []struct {
		raw  string
		want TriageSignals
	}{
		{`{"cycle_size":"large","cycle_size_estimate":"small","phase_skip":["tdd"],"deliverable_kind":"document"}`, TriageSignals{Present: true, CycleSize: "large", PhaseSkip: []string{"tdd"}, DeliverableKind: DeliverableKindDocument}},
		{`{"cycle_size_estimate":"small"}`, TriageSignals{Present: true, CycleSize: "small", DeliverableKind: NormalizeDeliverableKind("")}},
	}
	for _, tc := range cases {
		got := extractTriage([]byte(tc.raw))
		if got.Present != tc.want.Present || got.CycleSize != tc.want.CycleSize || got.DeliverableKind != tc.want.DeliverableKind || len(got.PhaseSkip) != len(tc.want.PhaseSkip) {
			t.Errorf("extractTriage(%s) = %+v, want %+v", tc.raw, got, tc.want)
		}
	}
}

func FuzzExtractSignals(f *testing.F) {
	for _, seed := range []string{"", "{}", `{"cycle_size":"small"}`, `{"item1":1}`, "null", "[]", `{"carryover_count":"x"}`, "\x00"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		scout := extractScout(raw)
		triage := extractTriage(raw)
		var probe map[string]json.RawMessage
		isObject := json.Unmarshal(raw, &probe) == nil
		if scout.Present != isObject {
			t.Fatalf("scout.Present=%v but object-ness=%v for %q", scout.Present, isObject, raw)
		}
		if !scout.Present && scout != (ScoutSignals{}) {
			t.Fatalf("an absent scout must be the zero value: %+v", scout)
		}
		if !triage.Present && (triage.CycleSize != "" || triage.PhaseSkip != nil) {
			t.Fatalf("an absent triage must be the zero value: %+v", triage)
		}
	})
}

func TestMandatoryPhaseRun_ConditionalPinOverridesEnableOffOnlyWhileTheRuleHolds(t *testing.T) {
	cfg := config.RoutingConfig{Conditional: map[string]config.CondRule{"tdd": config.DefaultTddRule()}}
	pinned := RouteInput{Cfg: cfg}
	released := RouteInput{Cfg: cfg, Signals: RoutingSignals{Triage: TriageSignals{Present: true, CycleSize: "trivial"}}}

	run, clamp, decided := mandatoryPhaseRun(pinned, "tdd", config.EnableOff)
	if !run || !decided || clamp == nil || clamp.Rule != "conditional-mandatory-pin" || clamp.Proposed != "tdd=off" || clamp.Forced != "tdd=on" {
		t.Fatalf("off + pinned: run=%v decided=%v clamp=%+v", run, decided, clamp)
	}
	run, clamp, decided = mandatoryPhaseRun(pinned, "tdd", config.EnableContent)
	if !run || !decided || clamp != nil {
		t.Fatalf("content + pinned: run=%v decided=%v clamp=%+v", run, decided, clamp)
	}
	run, clamp, decided = mandatoryPhaseRun(released, "tdd", config.EnableOff)
	if run || decided || clamp != nil {
		t.Fatalf("a released rule leaves the decision to the caller: run=%v decided=%v clamp=%+v", run, decided, clamp)
	}
}
