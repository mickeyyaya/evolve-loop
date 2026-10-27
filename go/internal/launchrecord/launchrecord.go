package launchrecord

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

const Subdir = "agent-runs"

type Record struct {
	RunID          string `json:"run_id"`
	Profile        string `json:"profile"`
	CLI            string `json:"cli"`
	Family         string `json:"family"`
	Model          string `json:"model"`
	Verdict        string `json:"verdict"`
	WritesSource   bool   `json:"writes_source,omitempty"`
	TreeBefore     string `json:"tree_state_sha_before"`
	TreeAfter      string `json:"tree_state_sha_after"`
	ArtifactSHA256 string `json:"artifact_sha256,omitempty"`
	Worktree       string `json:"worktree,omitempty"`
	TS             string `json:"ts"`
}

func (r Record) BoundTo(treeStateSHA string) bool {
	return treeStateSHA != "" && r.TreeBefore == treeStateSHA && r.TreeAfter == treeStateSHA
}

var runIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func Path(evolveDir, runID string) (string, error) {
	if !runIDPattern.MatchString(runID) {
		return "", fmt.Errorf("launch record: run id %q is not one name of letters, digits, '.', '_' and '-'", runID)
	}
	return filepath.Join(evolveDir, Subdir, runID+".json"), nil
}

func Write(evolveDir string, r Record) (string, error) {
	path, err := Path(evolveDir, r.RunID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("launch record %s: %w", r.RunID, err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("launch record %s: %w", r.RunID, err)
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return path, errors.Join(enc.Encode(r), f.Close())
}

func Read(evolveDir, runID string) (Record, error) {
	path, err := Path(evolveDir, runID)
	if err != nil {
		return Record{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Record{}, fmt.Errorf("launch record %s: %w", runID, err)
	}
	var r Record
	if err := json.Unmarshal(raw, &r); err != nil {
		return Record{}, fmt.Errorf("launch record %s: %w", runID, err)
	}
	if r.RunID != runID {
		return Record{}, fmt.Errorf("launch record %s: the file holds the run id %q", runID, r.RunID)
	}
	return r, nil
}
