package campaign

// progress.go — durable campaign-run progress (Memento + Repository). A long
// multi-wave campaign is interrupted by crashes, Ctrl-C, quota walls, and poison
// tasks; without a checkpoint the runner re-burns every completed wave on
// restart. CampaignProgress records which waves shipped so `--resume` skips them.
// It is keyed by goal hash and bound to the plan via PlanSHA: a changed plan
// invalidates stale progress (the caller ignores a mismatch and starts fresh).

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolveloop/go/internal/adapters/flock"
)

// CampaignProgress is the persisted record of how far a campaign run got.
type CampaignProgress struct {
	// PlanSHA binds this progress to the exact plan it was made for; on resume a
	// mismatch means the plan changed, so the progress is discarded (fresh run).
	PlanSHA string `json:"plan_sha"`
	// CompletedWaves holds the wave indices fully shipped (every cycle ok).
	CompletedWaves []int `json:"completed_waves"`
	// CompletedCycleIDs holds the todo IDs that shipped (cross-wave union).
	CompletedCycleIDs []string `json:"completed_cycle_ids"`
	// FailedCycleIDs holds quarantined/skipped IDs (e.g. an Optional poison task).
	FailedCycleIDs []string `json:"failed_cycle_ids"`
}

// ProgressPath is the canonical progress-file path for a goal hash under
// evolveDir (the run's .evolve directory).
func ProgressPath(evolveDir, goalHash string) string {
	return filepath.Join(evolveDir, "campaign-progress-"+goalHash+".json")
}

// HashPlan binds progress to the exact plan bytes — a changed plan yields a
// different SHA, so stale progress for an older plan is recognized and ignored.
func HashPlan(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// LoadProgress reads the progress file. An ABSENT file is not an error: it
// returns a zero-value progress, so "never ran" and "ran nothing yet" are
// handled identically by the caller.
func LoadProgress(path string) (*CampaignProgress, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &CampaignProgress{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("campaign: read progress %s: %w", path, err)
	}
	var p CampaignProgress
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("campaign: parse progress %s: %w", path, err)
	}
	return &p, nil
}

// Save persists the progress atomically (temp + rename) while holding the
// "<path>.lock" sidecar flock, so concurrent writers serialize and a crash
// mid-write leaves a stale .tmp rather than a corrupt progress file.
func (p *CampaignProgress) Save(path string) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("campaign: marshal progress: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("campaign: create progress dir: %w", err)
	}
	return flock.WithPathLock(path, func() error {
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, data, 0o644); err != nil {
			return fmt.Errorf("campaign: write progress tmp: %w", err)
		}
		if err := os.Rename(tmp, path); err != nil {
			return fmt.Errorf("campaign: rename progress: %w", err)
		}
		return nil
	})
}

// IsWaveComplete reports whether wave index w was already shipped.
func (p *CampaignProgress) IsWaveComplete(w int) bool {
	for _, c := range p.CompletedWaves {
		if c == w {
			return true
		}
	}
	return false
}

// MarkWaveComplete records wave w as shipped plus its cycle IDs. Idempotent:
// re-marking the same wave or IDs never duplicates entries (safe to call on a
// resumed run that re-touches a boundary).
func (p *CampaignProgress) MarkWaveComplete(w int, cycleIDs []string) {
	if !p.IsWaveComplete(w) {
		p.CompletedWaves = append(p.CompletedWaves, w)
	}
	for _, id := range cycleIDs {
		if !containsStr(p.CompletedCycleIDs, id) {
			p.CompletedCycleIDs = append(p.CompletedCycleIDs, id)
		}
	}
}

func containsStr(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
