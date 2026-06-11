// soakreport_test.go — R8.3: the read-only soak evidence reporter, the
// instrument the EVOLVE_PHASE_RECOVERY enforce flip (R8.5) is gated on.
// It aggregates the DURABLE shadow records each component already writes —
// interaction ledgers (C2 fatal_pane_shadow, I2 salvage, I3 kernel_answer,
// I4 rule_shadow_fire) and observer INCIDENT envelopes (C4/C3 actions) —
// plus the R6 outcome classification per cycle, and renders them against
// the plan §6 evidence bars.
package soakreport

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func plantWS(t *testing.T, root string, cycle int, files map[string]string) {
	t.Helper()
	ws := filepath.Join(root, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCollect_AggregatesAllComponentEvidence(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	plantWS(t, root, 1, map[string]string{
		"phase-timing.json": `[{"phase":"build","verdict":"PASS"},{"phase":"ship","verdict":"PASS"}]`,
		"build-interactions.ndjson": strings.Join([]string{
			`{"kind":"fatal_pane_shadow","phase":"build","result":"would_fast_fail","trigger":"dead_shell"}`,
			`{"kind":"salvage","phase":"build","result":"would_act"}`,
			`{"kind":"kernel_answer","phase":"build","result":"prompt_cleared"}`,
			`{"kind":"rule_shadow_fire","rule_id":"rule-x","result":"would_fire"}`,
		}, "\n") + "\n",
		"builder-observer-events.ndjson": strings.Join([]string{
			`{"type":"stuck_no_output","severity":"INCIDENT","data":{"action":"extend","action_reason":"within budget"}}`,
			`{"type":"heartbeat","severity":"INFO","data":{}}`,
		}, "\n") + "\n",
	})
	plantWS(t, root, 2, map[string]string{
		"phase-timing.json":              `[{"phase":"build","verdict":"FAIL","abort_reason":"audit said no"}]`,
		"build-interactions.ndjson":      `{"kind":"fatal_pane_shadow","phase":"build","result":"would_fast_fail","trigger":"model_invalid"}` + "\n",
		"builder-observer-events.ndjson": `{"type":"process_dead","severity":"INCIDENT","data":{"action":"kill_retry","action_reason":"group gone"}}` + "\n",
	})

	r := Collect(root, []int{1, 2})

	if len(r.Cycles) != 2 {
		t.Fatalf("cycles = %d, want 2", len(r.Cycles))
	}
	if r.Cycles[0].Outcome != "SHIPPED" || r.Cycles[1].Outcome != "FAILED_EXPLAINED" {
		t.Errorf("outcomes wrong: %+v", r.Cycles)
	}

	get := func(comp string) ComponentEvidence {
		t.Helper()
		for _, c := range r.Components {
			if c.Component == comp {
				return c
			}
		}
		t.Fatalf("component %s missing from report", comp)
		return ComponentEvidence{}
	}
	if n := get("C2").Counts["would_fast_fail"]; n != 2 {
		t.Errorf("C2 would_fast_fail = %d, want 2 (across cycles)", n)
	}
	if n := get("C4/C3").Counts["incident:stuck_no_output"]; n != 1 {
		t.Errorf("C4/C3 stuck_no_output incidents = %d, want 1 (INFO lines excluded)", n)
	}
	if n := get("C4/C3").Counts["incident:process_dead"]; n != 1 {
		t.Errorf("C4/C3 process_dead incidents = %d, want 1", n)
	}
	if n := get("I2").Counts["would_act"]; n != 1 {
		t.Errorf("I2 would_act = %d, want 1", n)
	}
	if n := get("I3").Counts["kernel_answer"]; n != 1 {
		t.Errorf("I3 kernel answers = %d, want 1", n)
	}
	if n := get("I4").Counts["would_fire:rule-x"]; n != 1 {
		t.Errorf("I4 per-rule fires = %d, want 1", n)
	}
}

func TestCollect_EmptyWorkspacesDegradeToNotes(t *testing.T) {
	t.Parallel()
	r := Collect(t.TempDir(), []int{7})
	if len(r.Cycles) != 1 || r.Cycles[0].Outcome != "FAILED_UNEXPLAINED" {
		t.Fatalf("missing workspace must still classify (unexplained): %+v", r.Cycles)
	}
	// Every component still appears (with zero counts) — the report's shape
	// is stable so soak comparisons never diff against absent sections.
	if len(r.Components) != 5 {
		t.Errorf("components = %d, want the stable 5 (C2, C4/C3, I2, I3, I4)", len(r.Components))
	}
}

func TestRender_TableCarriesBarsAndCounts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	plantWS(t, root, 3, map[string]string{
		"phase-timing.json":         `[{"phase":"ship","verdict":"PASS"}]`,
		"build-interactions.ndjson": `{"kind":"fatal_pane_shadow","result":"would_fast_fail","trigger":"dead_shell"}` + "\n",
	})
	out := Collect(root, []int{3}).Render()
	for _, want := range []string{
		"cycle-3", "SHIPPED", // per-cycle outcome row
		"C2", "≥10 observations", // component + its plan §6 bar
		"would_fast_fail", "1", // the evidence count
		"I4", "I3", "I2", "C4/C3", // stable sections
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered report missing %q:\n%s", want, out)
		}
	}
}
