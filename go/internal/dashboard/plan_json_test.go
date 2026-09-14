package dashboard

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// The wire shape the page reads: PhasePlan and PlanStep marshal with the
// snake_case keys app.js dereferences, every status word the page branches
// on is a pinned literal, and the optional marks are omitted when false.
func TestPhasePlan_PlanStep_WireShape(t *testing.T) {
	t.Parallel()
	for _, status := range []string{StatePass, StateWarn, StateFail, StateIncomplete, stepOngoing, stepPending, stepUnreached, stepSkipped} {
		if _, ok := map[string]bool{"pass": true, "warn": true, "fail": true, "incomplete": true, "ongoing": true, "pending": true, "unreached": true, "skipped": true}[status]; !ok {
			t.Errorf("status %q is not one the page renders", status)
		}
	}
	plan := PhasePlan{
		Mandatory:      []string{"scout", "build"},
		Steps:          []PlanStep{{Phase: "scout", Status: StatePass, GateVerified: true, DurationMS: 1000}, {Phase: "tdd", Status: StatePass, Conditional: true}, {Phase: "build", Status: stepOngoing, Rounds: 2}},
		Required:       3,
		PassedRequired: 2,
		Total:          3,
		Passed:         2,
		Ongoing:        "build",
		OngoingSince:   time.Date(2026, 9, 14, 8, 35, 0, 0, time.UTC),
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, key := range []string{`"mandatory":["scout","build"]`, `"steps":[`, `"phase":"scout"`, `"status":"pass"`, `"gate_verified":true`, `"duration_ms":1000`, `"conditional":true`, `"status":"ongoing"`, `"rounds":2`, `"required":3`, `"passed_required":2`, `"total":3`, `"passed":2`, `"ongoing":"build"`, `"ongoing_since":"2026-09-14T08:35:00Z"`} {
		if !strings.Contains(s, key) {
			t.Errorf("wire shape lacks %s: %s", key, s)
		}
	}
	if strings.Contains(s, `"optional"`) || strings.Contains(s, `"remaining"`) || strings.Contains(s, `"advisor_proposed"`) {
		t.Errorf("false/empty marks must be omitted: %s", s)
	}
	var back PhasePlan
	if err := json.Unmarshal(raw, &back); err != nil || len(back.Steps) != 3 || back.Steps[2].Rounds != 2 {
		t.Errorf("round trip: %+v err=%v", back, err)
	}
}
