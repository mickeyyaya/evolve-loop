package campaign

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
)

type CampaignProgress struct {
	PlanSHA           string   `json:"plan_sha"`
	CompletedWaves    []int    `json:"completed_waves"`
	CompletedCycleIDs []string `json:"completed_cycle_ids"`
	FailedCycleIDs    []string `json:"failed_cycle_ids"`
}

func ProgressPath(evolveDir, goalHash string) string {
	return filepath.Join(evolveDir, "campaign-progress-"+goalHash+".json")
}

func HashPlan(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

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

func (p *CampaignProgress) IsWaveComplete(w int) bool {
	for _, c := range p.CompletedWaves {
		if c == w {
			return true
		}
	}
	return false
}

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
