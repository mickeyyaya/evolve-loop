package policy

import (
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/qualityindex"
)

type QualityIndexPolicy struct {
	Thresholds map[string]int `json:"thresholds,omitempty"`
}

type QualityIndexConfig struct {
	Thresholds qualityindex.Thresholds
	Warnings   []string
}

func (p Policy) qualityIndexConfig() QualityIndexConfig {
	var raw map[string]int
	if p.Workflow != nil && p.Workflow.QualityIndex != nil {
		raw = p.Workflow.QualityIndex.Thresholds
	}
	t, warnings := qualityindex.ResolveThresholds(raw)
	return QualityIndexConfig{Thresholds: t, Warnings: warnings}
}

func QualityIndexThresholdsFor(projectRoot string) (qualityindex.Thresholds, error) {
	pol, err := Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		return nil, fmt.Errorf("quality-index thresholds: %w", err)
	}
	return pol.qualityIndexConfig().Thresholds, nil
}
