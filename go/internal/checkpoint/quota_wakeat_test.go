package checkpoint

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotareset"
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

func TestQuotaBoundaryCheckpointer_TheEarliestActiveBenchSetsTheWakeAt(t *testing.T) {
	root, cs := quotaCheckpointFixture(t)
	now := time.Date(2026, 10, 9, 18, 42, 13, 0, time.Local)
	store := clihealth.NewStore(root, func() time.Time { return now })
	for family, reset := range map[string]time.Time{"agy-claude": now.Add(89 * time.Hour), "codex": now.Add(3 * time.Hour)} {
		if _, err := store.BenchWallUntil(family, clihealth.Wall{Pattern: "exhausted", Reset: reset}); err != nil {
			t.Fatalf("bench %s: %v", family, err)
		}
	}
	want := store.Active()["codex"].BenchedUntil
	cs.QuotaWalkCLIs = []string{"agy-claude-tmux", "codex-tmux"}

	if err := core.QuotaBoundaryCheckpointer(cs, root, now); err != nil {
		t.Fatalf("checkpointer: %v", err)
	}

	at, source := readQuotaWakeAt(t, root)
	wake, err := time.Parse("2006-01-02T15:04:05-0700", at)
	if err != nil {
		t.Fatalf("quotaResetAt %q unparseable: %v", at, err)
	}
	if source != "bench" || !wake.Equal(want.Truncate(time.Second)) {
		t.Errorf("wake-at=%s source=%q, want %s from the bench: the first family back sets the resume, not a default", at, source, want)
	}
}

func TestWithQuotaReset_AnEstimatorErrorRecordsUnavailableAndNoWakeAt(t *testing.T) {
	saved := computeQuotaReset
	t.Cleanup(func() { computeQuotaReset = saved })
	computeQuotaReset = func(string, quotareset.Options) (quotareset.Result, error) {
		return quotareset.Result{}, errors.New("estimator down")
	}

	cp := withQuotaReset(Checkpoint{}, core.CycleState{WorkspacePath: t.TempDir()}, t.TempDir(), time.Date(2026, 10, 9, 18, 42, 13, 0, time.Local))

	if cp.QuotaResetSource != "unavailable" || cp.QuotaResetAt != "" {
		t.Errorf("source=%q at=%q, want unavailable with no wake-at: a failed estimate never invents a time", cp.QuotaResetSource, cp.QuotaResetAt)
	}
}

type pauseRecord struct {
	QuotaResetAt          string `json:"quotaResetAt"`
	QuotaResetSource      string `json:"quotaResetSource"`
	AutoResumeMaxAttempts int    `json:"autoResumeMaxAttempts"`
	OperatorAction        string `json:"operatorAction"`
}

func readPause(t *testing.T, root string) pauseRecord {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, ".evolve", "cycle-state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state struct {
		Checkpoint pauseRecord `json:"checkpoint"`
	}
	if err := json.Unmarshal(b, &state); err != nil {
		t.Fatal(err)
	}
	return state.Checkpoint
}

func withUsageReset(t *testing.T, fn func(projectRoot string, families []string, now time.Time) (time.Time, bool)) {
	t.Helper()
	saved := UsageReset
	t.Cleanup(func() { UsageReset = saved })
	UsageReset = fn
}

func TestQuotaBoundaryCheckpointer_AnUnknownResetIsNotAutoResumableAndNamesTheOperatorAction(t *testing.T) {
	withUsageReset(t, nil)
	root, cs := quotaCheckpointFixture(t)
	cs.QuotaWalkCLIs = []string{"claude-tmux"}

	if err := core.QuotaBoundaryCheckpointer(cs, root, time.Date(2026, 10, 9, 18, 42, 13, 0, time.Local)); err != nil {
		t.Fatalf("checkpointer: %v", err)
	}

	got := readPause(t, root)
	if got.QuotaResetSource != "unknown" || got.QuotaResetAt != "" || got.AutoResumeMaxAttempts != 0 || got.OperatorAction == "" {
		t.Errorf("pause=%+v, want source unknown, no wake-at, no auto-resume and an operator action: an unknown reset is never a scheduled wake", got)
	}
}

func TestQuotaBoundaryCheckpointer_ABenchThatIsNoQuotaResetOfTheWalkIsNoEvidence(t *testing.T) {
	now := time.Date(2026, 10, 9, 18, 42, 13, 0, time.Local)
	for _, tc := range []struct {
		name  string
		bench func(root string) error
	}{
		{"a credential bench", func(root string) error {
			_, err := clihealth.NewStore(root, func() time.Time { return now }).BenchWall("claude", clihealth.CredentialPattern, "Please log in")
			return err
		}},
		{"a boot-timeout strike", func(root string) error {
			s := clihealth.NewStore(root, func() time.Time { return now })
			if _, err := s.RecordBootStrike("claude"); err != nil {
				return err
			}
			_, err := s.RecordBootStrike("claude")
			return err
		}},
		{"a quota bench of a family outside the walk", func(root string) error {
			_, err := clihealth.NewStore(root, func() time.Time { return now }).BenchWallUntil("codex", clihealth.Wall{Pattern: "rate_limit", Reset: now.Add(time.Hour)})
			return err
		}},
		{"a quota bench that already lapsed", func(root string) error {
			past := clihealth.NewStore(root, func() time.Time { return now.Add(-10 * time.Hour) })
			_, err := past.BenchWallUntil("claude", clihealth.Wall{Pattern: "rate_limit", Reset: now.Add(-9 * time.Hour)})
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withUsageReset(t, nil)
			root, cs := quotaCheckpointFixture(t)
			cs.QuotaWalkCLIs = []string{"claude-tmux"}
			if err := tc.bench(root); err != nil {
				t.Fatalf("seed bench: %v", err)
			}

			if err := core.QuotaBoundaryCheckpointer(cs, root, now); err != nil {
				t.Fatalf("checkpointer: %v", err)
			}

			if got := readPause(t, root); got.QuotaResetSource != "unknown" {
				t.Errorf("source=%q at=%q, want unknown: %s is not the reset of a walled family", got.QuotaResetSource, got.QuotaResetAt, tc.name)
			}
		})
	}
}

func TestQuotaBoundaryCheckpointer_TheUsageQuerySuppliesTheResetOfTheWalkedFamilies(t *testing.T) {
	now := time.Date(2026, 10, 9, 18, 42, 13, 0, time.Local)
	reset := now.Add(89 * time.Hour)
	var asked []string
	withUsageReset(t, func(_ string, families []string, _ time.Time) (time.Time, bool) {
		asked = families
		return reset, true
	})
	root, cs := quotaCheckpointFixture(t)
	cs.QuotaWalkCLIs = []string{"claude-tmux", "agy-claude-tmux"}

	if err := core.QuotaBoundaryCheckpointer(cs, root, now); err != nil {
		t.Fatalf("checkpointer: %v", err)
	}

	got := readPause(t, root)
	if got.QuotaResetSource != "usage" || got.AutoResumeMaxAttempts != DefaultAutoResumeAttempts || got.OperatorAction != "" {
		t.Errorf("pause=%+v, want an auto-resumable usage reset", got)
	}
	if strings.Join(asked, ",") != "claude,agy-claude" {
		t.Errorf("usage query asked for %v, want the walked families claude,agy-claude", asked)
	}
}
