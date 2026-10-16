package phaseoutputs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func LoadListing(workspace string) (map[string]int64, error) {
	entries, err := os.ReadDir(workspace)
	if err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if info, err := e.Info(); err == nil {
			out[e.Name()] = info.Size()
		}
	}
	return out, nil
}

func LoadShadowReading(workspace, filename string) RecordReading {
	raw, err := os.ReadFile(filepath.Join(workspace, filename))
	if err != nil {
		return RecordReading{}
	}
	var v ShadowView
	if json.Unmarshal(raw, &v) != nil {
		return RecordReading{Corrupt: true}
	}
	return RecordReading{View: &v}
}

func LoadCompletedPhases(workspace string) ([]string, error) {
	raw, err := os.ReadFile(filepath.Join(workspace, "run.json"))
	if err != nil {
		return nil, fmt.Errorf("read run.json: %w", err)
	}
	var doc struct {
		CompletedPhases []string `json:"completed_phases"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse run.json: %w", err)
	}
	return doc.CompletedPhases, nil
}
