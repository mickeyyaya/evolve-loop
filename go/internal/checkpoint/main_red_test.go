package checkpoint

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestMainRedCheckpointer_WritesAnEscalationCheckpointThatAPhaseBoundaryKeeps(t *testing.T) {
	root, cs := quotaCheckpointFixture(t)
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	if err := core.MainRedBoundaryCheckpointer(cs, root, now); err != nil {
		t.Fatal(err)
	}
	if err := core.PhaseBoundaryCheckpointer(cs, root, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got := mainRedCheckpointReason(t, root); got != core.MainRedReason || string(ReasonMainRed) != core.MainRedReason {
		t.Errorf("checkpoint reason = %q, want %q after a phase boundary", got, core.MainRedReason)
	}
}

func TestReasonMainRed_IsValidAndIsNamedInTheRefusal(t *testing.T) {
	t.Parallel()
	if !ReasonMainRed.IsValid() {
		t.Error("ReasonMainRed is not valid")
	}
	if _, err := ComposeChecked(core.CycleState{}, ReasonMainRed, 0, "", time.Time{}); err != nil {
		t.Errorf("ComposeChecked refused main-red: %v", err)
	}
	if _, err := ComposeChecked(core.CycleState{}, "bogus", 0, "", time.Time{}); err == nil || !strings.Contains(err.Error(), "main-red") {
		t.Errorf("the refusal = %v, want it to list main-red", err)
	}
}

func TestComposeChecked_TheRefusalNamesEachValidReason(t *testing.T) {
	t.Parallel()
	_, err := ComposeChecked(core.CycleState{}, "bogus", 0, "", time.Time{})
	if err == nil {
		t.Fatal("ComposeChecked took a reason that is not in the list")
	}
	for _, r := range []Reason{ReasonQuotaLikely, ReasonBatchCapNear, ReasonOperatorRequest, ReasonStallInactivity, ReasonPhaseComplete, ReasonMainRed} {
		if !r.IsValid() || !strings.Contains(err.Error(), string(r)) {
			t.Errorf("reason %q: valid = %v, refusal = %v, want a valid reason that the refusal names", r, r.IsValid(), err)
		}
	}
	if !strings.Contains(err.Error(), "quota-likely | batch-cap-near") {
		t.Errorf("the refusal = %v, want the reasons with a bar between them", err)
	}
}

func mainRedCheckpointReason(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, ".evolve", "cycle-state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state struct {
		Checkpoint struct {
			Enabled bool   `json:"enabled"`
			Reason  string `json:"reason"`
		} `json:"checkpoint"`
	}
	if err := json.Unmarshal(b, &state); err != nil || !state.Checkpoint.Enabled {
		t.Fatalf("checkpoint = %+v, %v, want an enabled checkpoint", state.Checkpoint, err)
	}
	return state.Checkpoint.Reason
}
