package core

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/router"
)

// WS3-S5 (ADR-0052): ReplayPlanFromResponse reparses a captured advisor
// response (WS3-S1's advisor-response-<kind>.txt) through the SAME parse +
// integrity-floor clamp the live planning path runs, so the recorded
// phase-plan.json is deterministically reproducible from the raw response —
// a regression lock on the parse + floor (the entry point WS4 builds on).

func planRunsTest(p *router.PhasePlan, phase string) bool {
	for _, e := range p.Entries {
		if e.Phase == phase {
			return e.Run
		}
	}
	return false
}

func TestRoutingReplay_ReparsesCapturedResponseToSamePlan(t *testing.T) {
	t.Parallel()
	// A captured response that reaches ship while SKIPPING audit — the floor
	// must clamp audit ON during replay, exactly as the live path did.
	raw := `[{"phase":"audit","run":false,"justification":"skip"},{"phase":"ship","run":true,"justification":"done"}]`
	in := router.RouteInput{}
	floor := router.DefaultShipFloor()

	clamped, clamps, err := ReplayPlanFromResponse(raw, in, floor)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	// Replay reuses the REAL floor — audit forced ON. This is what locks
	// prompt/model regressions against the floor, not just the parse.
	if !planRunsTest(clamped, "audit") {
		t.Errorf("replay must clamp audit ON (ship-without-audit); entries=%+v", clamped.Entries)
	}
	if len(clamps) == 0 {
		t.Error("expected ≥1 clamp (audit forced) on replay")
	}

	// Equivalence: replay == the live parse+clamp the orchestrator records, so
	// the recorded phase-plan.json is reproducible byte-for-byte.
	parsed, perr := parsePhasePlan(raw)
	if perr != nil {
		t.Fatalf("parse: %v", perr)
	}
	wantClamped, _ := router.ClampPlanToFloorWith(in, parsed, floor, in.IntentRequired)
	if !reflect.DeepEqual(clamped.Entries, wantClamped.Entries) {
		t.Errorf("replay must equal the live parse+clamp\n got %+v\nwant %+v", clamped.Entries, wantClamped.Entries)
	}

	// Determinism: identical input ⇒ identical clamped entries.
	clamped2, _, _ := ReplayPlanFromResponse(raw, in, floor)
	if !reflect.DeepEqual(clamped.Entries, clamped2.Entries) {
		t.Error("replay must be deterministic")
	}

	// A corrupted/unparseable capture is a LOUD error, never a silent empty plan
	// (the whole point of replay is to detect exactly this).
	if _, _, err := ReplayPlanFromResponse("not json at all", in, floor); err == nil {
		t.Error("unparseable response must error")
	}
}
