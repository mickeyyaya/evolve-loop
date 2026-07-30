package checkpoint

// quota_wakeat_test.go — RED contract for the inbox defect
// `quota-pause-wakeat-unpopulated`.
//
// The all-families-exhausted checkpointer (core.QuotaBoundaryCheckpointer, the
// cycle-656 seam) wrote ReasonQuotaLikely with QuotaResetAt/QuotaResetSource left
// EMPTY. cmd_loop then printed `QUOTA-PAUSE: … wake-at= source=unknown`, and
// skills/loop/SKILL.md instructs the operator model to parse wake-at=ISO8601 and
// compute its ScheduleWakeup delay from it — arithmetic that could never run on
// the Go path. Auto-resume was structurally dead for every quota wall.
//
// FIX CONTRACT: the checkpointer populates both fields at write time from
// quotareset.Compute — the package that already owns the source-priority chain
// (operator override > workspace hint file > now + default hours) — so the
// wake-at is NEVER empty and the source always says where it came from.
//
// ADVERSARIAL DIVERSITY: one test per source arm (override / parsed hint /
// estimate fallback) plus a negative that the sibling phase-complete
// checkpointer does NOT grow a fabricated wake-at — only the quota wall has a
// reset instant, and stamping one on every phase boundary would invent data.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// quotaCheckpointFixture seeds a project root with a cycle-state.json and a
// cycle workspace, and returns the CycleState the dispatch seam would pass.
func quotaCheckpointFixture(t *testing.T) (root string, cs core.CycleState) {
	t.Helper()
	root = t.TempDir()
	seedCycleState(t, root, "") // no prior checkpoint block
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-656")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, core.CycleState{
		CycleID:       656,
		Phase:         "build",
		WorkspacePath: workspace,
	}
}

// readQuotaWakeAt returns the persisted (quotaResetAt, quotaResetSource) pair.
func readQuotaWakeAt(t *testing.T, root string) (at, source string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, ".evolve", "cycle-state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state struct {
		Checkpoint struct {
			Reason           string `json:"reason"`
			QuotaResetAt     string `json:"quotaResetAt"`
			QuotaResetSource string `json:"quotaResetSource"`
		} `json:"checkpoint"`
	}
	if err := json.Unmarshal(b, &state); err != nil {
		t.Fatal(err)
	}
	if state.Checkpoint.Reason != string(ReasonQuotaLikely) {
		t.Fatalf("checkpoint reason = %q, want %q", state.Checkpoint.Reason, ReasonQuotaLikely)
	}
	return state.Checkpoint.QuotaResetAt, state.Checkpoint.QuotaResetSource
}

// TestQuotaBoundaryCheckpointer_PopulatesWakeAtFromEstimate is the core RED: with
// no operator override and no hint file, the checkpoint must still carry a
// non-empty wake-at and say it is an estimate. Empty was the defect.
func TestQuotaBoundaryCheckpointer_PopulatesWakeAtFromEstimate(t *testing.T) {
	if core.QuotaBoundaryCheckpointer == nil {
		t.Fatal("core.QuotaBoundaryCheckpointer not registered (init() wiring missing)")
	}
	root, cs := quotaCheckpointFixture(t)
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.Local)
	if err := core.QuotaBoundaryCheckpointer(cs, root, now); err != nil {
		t.Fatalf("checkpointer: %v", err)
	}
	at, source := readQuotaWakeAt(t, root)
	if at == "" {
		t.Fatal("quotaResetAt is EMPTY — the auto-resume delay computation has nothing to parse (the defect)")
	}
	if source == "" || source == "unknown" {
		t.Errorf("quotaResetSource = %q, want an explicit source (fail-open estimate, never blank)", source)
	}
	// The fallback must land in the FUTURE relative to the injected now, else a
	// consumer's max(60, wake-now) clamp degrades to a busy 60s poll.
	wake, err := time.Parse("2006-01-02T15:04:05-0700", at)
	if err != nil {
		t.Fatalf("quotaResetAt %q is not the ISO 8601 shape SKILL.md parses: %v", at, err)
	}
	if !wake.After(now) {
		t.Errorf("wake-at %s is not after now %s — an already-passed wake time cannot schedule a resume", at, now)
	}
}

// TestQuotaBoundaryCheckpointer_UsesWorkspaceHint — when the CLI adapter scraped
// Anthropic's "resets HH:MMam" message into the cycle workspace, THAT is the
// authoritative wake time and the source must say so (not the blind estimate).
// This is also the proof that the checkpointer passes cs.WorkspacePath through:
// a hard-coded empty workspace would silently fall back to the estimate.
func TestQuotaBoundaryCheckpointer_UsesWorkspaceHint(t *testing.T) {
	root, cs := quotaCheckpointFixture(t)
	if err := os.WriteFile(filepath.Join(cs.WorkspacePath, "quota-reset-hint.txt"), []byte("resets 4:10pm\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.Local)
	if err := core.QuotaBoundaryCheckpointer(cs, root, now); err != nil {
		t.Fatalf("checkpointer: %v", err)
	}
	at, source := readQuotaWakeAt(t, root)
	if source != "parsed" {
		t.Errorf("quotaResetSource = %q, want \"parsed\" — the scraped hint must beat the estimate", source)
	}
	if wake, err := time.Parse("2006-01-02T15:04:05-0700", at); err != nil {
		t.Fatalf("quotaResetAt %q unparseable: %v", at, err)
	} else if wake.Hour() != 16 || wake.Minute() != 10 {
		t.Errorf("wake-at = %s, want the hint's 16:10", at)
	}
}

// TestQuotaBoundaryCheckpointer_HonoursOperatorOverride — policy.json
// quota_reset.reset_at is the top of the source chain, so an operator who knows
// the real reset instant wins over both the hint and the estimate. Also pins that
// the checkpointer reads its config from policy, never a Go literal
// (feedback_phase_settings_from_config_not_code).
func TestQuotaBoundaryCheckpointer_HonoursOperatorOverride(t *testing.T) {
	root, cs := quotaCheckpointFixture(t)
	// A hint file is present too: the override must still win.
	if err := os.WriteFile(filepath.Join(cs.WorkspacePath, "quota-reset-hint.txt"), []byte("resets 4:10pm\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	const override = "2026-07-30T23:45:00Z"
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"),
		[]byte(`{"quota_reset":{"reset_at":"`+override+`"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.QuotaBoundaryCheckpointer(cs, root, time.Now()); err != nil {
		t.Fatalf("checkpointer: %v", err)
	}
	at, source := readQuotaWakeAt(t, root)
	if at != override || source != "operator-override" {
		t.Errorf("(at, source) = (%q, %q), want (%q, \"operator-override\")", at, source, override)
	}
}

// TestPhaseBoundaryCheckpointer_DoesNotFabricateWakeAt is the paired negative:
// only the quota wall has a reset instant. A phase-complete breadcrumb must NOT
// grow one, or every routine checkpoint would advertise a fictional wake time
// that a resume consumer could act on.
func TestPhaseBoundaryCheckpointer_DoesNotFabricateWakeAt(t *testing.T) {
	root := t.TempDir()
	seedCycleState(t, root, "")
	cs := core.CycleState{CycleID: 657, Phase: "audit", WorkspacePath: filepath.Join(root, ".evolve", "runs", "cycle-657")}
	if err := core.PhaseBoundaryCheckpointer(cs, root, time.Now()); err != nil {
		t.Fatalf("phase-boundary checkpointer: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, ".evolve", "cycle-state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state struct {
		Checkpoint struct {
			Reason           string `json:"reason"`
			QuotaResetAt     string `json:"quotaResetAt"`
			QuotaResetSource string `json:"quotaResetSource"`
		} `json:"checkpoint"`
	}
	if err := json.Unmarshal(b, &state); err != nil {
		t.Fatal(err)
	}
	if state.Checkpoint.Reason != string(ReasonPhaseComplete) {
		t.Fatalf("reason = %q, want phase-complete", state.Checkpoint.Reason)
	}
	if state.Checkpoint.QuotaResetAt != "" || state.Checkpoint.QuotaResetSource != "" {
		t.Errorf("phase-complete checkpoint fabricated a wake-at: at=%q source=%q",
			state.Checkpoint.QuotaResetAt, state.Checkpoint.QuotaResetSource)
	}
}
