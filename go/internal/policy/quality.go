package policy

import (
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
)

type QualityPanelPolicy struct {
	Stages map[string]qualitypanel.Stage
}

type qualityPanelIndex struct {
	Stage string `json:"stage"`
}

type qualityPanelJSON struct {
	Indexes map[string]qualityPanelIndex `json:"indexes"`
}

func (q *QualityPanelPolicy) UnmarshalJSON(raw []byte) error {
	var decoded qualityPanelJSON
	if err := decodeStrict(raw, &decoded); err != nil {
		return fmt.Errorf("quality: %w", err)
	}
	stages := make(map[string]qualitypanel.Stage, len(decoded.Indexes))
	for id, ix := range decoded.Indexes {
		st, err := qualityStage(id, ix.Stage)
		if err != nil {
			return err
		}
		stages[id] = st
	}
	*q = QualityPanelPolicy{Stages: stages}
	return nil
}

func qualityStage(id, stage string) (qualitypanel.Stage, error) {
	if _, ok := qualitypanel.Lookup(id); !ok {
		return "", fmt.Errorf("quality: index %q is not one of the 22 ids of docs/architecture/llm-quality-indexes.md", id)
	}
	st, err := qualitypanel.ParseStage(stage)
	if err != nil {
		return "", fmt.Errorf("quality: index %s: %w", id, err)
	}
	return st, nil
}

func (p Policy) QualityStages() map[string]qualitypanel.Stage {
	out := map[string]qualitypanel.Stage{}
	if p.Quality != nil {
		for id, st := range p.Quality.Stages {
			out[id] = st
		}
	}
	return out
}

func QualityStagesFor(projectRoot string) (map[string]qualitypanel.Stage, error) {
	pol, err := Load(filepath.Join(paths.EvolveDirOf(projectRoot), paths.PolicyFile))
	if err != nil {
		return map[string]qualitypanel.Stage{}, err
	}
	return pol.QualityStages(), nil
}
