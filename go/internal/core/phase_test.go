package core

import (
	"encoding/json"
	"strings"
	"testing"
)

// PhaseRequest/PhaseResponse must round-trip through encoding/json
// exactly — they're the wire format for the subprocess phase override
// (pkg/phaseproto) and the in-memory contract between orchestrator
// and phases.
func TestPhaseRequest_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	in := PhaseRequest{
		Cycle:       104,
		ProjectRoot: "/tmp/x",
		Workspace:   "/tmp/x/.evolve/runs/cycle-104",
		Worktree:    "/tmp/x/.evolve/worktrees/cycle-104",
		GoalHash:    "abc123",
		Context:     map[string]string{"intent": "rewrite as Go"},
		Budget:      BudgetEnvelope{MaxUSD: 5.0, BatchCapUSD: 20.0},
		Env:         map[string]string{"EVOLVE_PROJECT_ROOT": "/tmp/x"},
	}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out PhaseRequest
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Cycle != in.Cycle || out.GoalHash != in.GoalHash || out.Workspace != in.Workspace {
		t.Errorf("round-trip mismatch:\n in=%+v\nout=%+v", in, out)
	}
	if out.Context["intent"] != "rewrite as Go" {
		t.Errorf("Context lost: %+v", out.Context)
	}
	if out.Budget.MaxUSD != 5.0 || out.Budget.BatchCapUSD != 20.0 {
		t.Errorf("Budget lost: %+v", out.Budget)
	}
}

func TestPhaseResponse_JSONShape(t *testing.T) {
	t.Parallel()
	r := PhaseResponse{
		Phase:        "scout",
		Verdict:      "PASS",
		ArtifactsDir: "/tmp/x/.evolve/runs/cycle-104",
		CostUSD:      0.42,
		Tokens:       TokenUsage{Input: 1000, Output: 500, CacheRead: 200},
		DurationMS:   12345,
		Diagnostics: []Diagnostic{
			{Severity: "WARN", Message: "research quota near"},
		},
	}
	raw, _ := json.Marshal(r)
	// Spot-check key field names match what bash/audit artifacts use.
	for _, want := range []string{
		`"phase":"scout"`,
		`"verdict":"PASS"`,
		`"artifacts_dir":"/tmp/x/.evolve/runs/cycle-104"`,
		`"cost_usd":0.42`,
		`"duration_ms":12345`,
		`"diagnostics"`,
	} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("missing %q in: %s", want, raw)
		}
	}
}

// VerdictPASS/FAIL/WARN/SKIPPED are the only allowed verdict strings
// (matches EGPS gate semantics — see CLAUDE.md env-var table).
func TestVerdict_Recognised(t *testing.T) {
	t.Parallel()
	for _, v := range []string{VerdictPASS, VerdictFAIL, VerdictWARN, VerdictSKIPPED} {
		if !IsVerdict(v) {
			t.Errorf("IsVerdict(%q) = false; want true", v)
		}
	}
	for _, v := range []string{"", "ok", "FAIL ", "pass"} {
		if IsVerdict(v) {
			t.Errorf("IsVerdict(%q) = true; want false", v)
		}
	}
}
