package phaseoutputs

// io.go — the one sanctioned workspace reader. Survey, CycleChainStatus and
// Signal stay pure (they consume what was read); this file is their shared I/O
// companion so the two adapters — `evolve cycle outputs` and the loop's
// post-cycle signal emitter — cannot drift into reading different files or
// classifying the same read differently.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LoadListing builds the name→size listing Survey consumes: top-level files
// only, sizes kept because Empty detection depends on them.
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

// LoadShadowReading totalizes the three outcomes of reading the shadow record
// file (the caller passes auditchain.ShadowRecordFile — the name is not
// re-declared here): absent → zero reading, unparseable → Corrupt, parsed →
// View. Collapsing corrupt into absent was a review finding — an existing
// truncated record is a recorder defect, not a missing record.
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

// LoadCompletedPhases decodes the one run.json field the survey needs. A
// missing or unparseable run.json is a loud error — an aborted cycle is a
// fact the caller must surface, not an empty survey.
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
