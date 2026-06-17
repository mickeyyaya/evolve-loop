package core

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// TestPhaseAdvisor_RePlanThreadsSignalsAndDepth pins the WS1-S3 re-invokable
// seam (ADR-0052): RePlan is a SECOND whole-cycle plan run once scout's handoff
// has populated in.Signals (need MEASURED, not inferred from goal text). It
// shares planWith with the initial Plan (same compose→dispatch→parse path),
// writes a DISTINCT routing-replan.json artifact, threads the populated Signals
// into the prompt, and stamps replan_depth=1 on its decision span. It is NOT yet
// wired into the loop — this slice only provides the entrypoint; WS2 drives it
// behind the EVOLVE_ROUTER_REPLAN dial.
func TestPhaseAdvisor_RePlanThreadsSignalsAndDepth(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	plan := `[{"phase":"scout","run":true,"justification":"x"},{"phase":"build","run":true,"justification":"y"}]`
	fb := &fakeBridge{stdout: plan, durationMS: 9}
	in := baseRouteInput()
	in.Workspace = ws
	in.Signals = router.RoutingSignals{Scout: router.ScoutSignals{Present: true, ItemCount: 5, CycleSizeEstimate: "large"}}

	got, err := NewPhaseAdvisor(fb, WithPersona("PERSONA")).RePlan(in)
	if err != nil {
		t.Fatalf("RePlan: %v", err)
	}
	if len(got.Entries) != 2 {
		t.Fatalf("RePlan parsed %d entries, want 2 (must parse like Plan)", len(got.Entries))
	}
	if !strings.HasSuffix(fb.gotReq.ArtifactPath, "routing-replan.json") {
		t.Errorf("RePlan artifact=%q, want .../routing-replan.json (distinct from the initial plan)", fb.gotReq.ArtifactPath)
	}
	if !strings.Contains(fb.gotReq.Prompt, "item_count=5") {
		t.Errorf("RePlan must thread the populated Signals into the prompt; got:\n%s", fb.gotReq.Prompt)
	}

	span := readReplanSpan(t, ws, "replan")
	if span["replan_depth"] != float64(1) {
		t.Errorf("replan_depth = %v, want 1 (RePlan is one level deep)", span["replan_depth"])
	}

	// The initial Plan stamps replan_depth=0 — the value VARIES with stage (which
	// is why WS3-S3 deferred asserting it until this slice could make it differ).
	fb2 := &fakeBridge{stdout: plan}
	in2 := baseRouteInput()
	in2.Workspace = ws
	if _, err := NewPhaseAdvisor(fb2, WithPersona("PERSONA")).Plan(in2); err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if span := readReplanSpan(t, ws, "plan"); span["replan_depth"] != float64(0) {
		t.Errorf("initial Plan replan_depth = %v, want 0", span["replan_depth"])
	}
}

func readReplanSpan(t *testing.T, ws, kind string) map[string]any {
	t.Helper()
	raw := readAdvisorArtifact(t, filepath.Join(ws, "advisor-span-"+kind+".json"))
	var span map[string]any
	if err := json.Unmarshal([]byte(raw), &span); err != nil {
		t.Fatalf("span json (%s): %v\n%s", kind, err, raw)
	}
	return span
}
