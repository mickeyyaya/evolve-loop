package landing

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const intentDir = "landing"

type IntentStatus string

const (
	IntentPrepared IntentStatus = "prepared"
	IntentComplete IntentStatus = "complete"
	IntentUnwound  IntentStatus = "unwound"
	IntentStale    IntentStatus = "stale"
)

type Intent struct {
	Cycle                 int          `json:"cycle"`
	RunID                 string       `json:"run_id"`
	AuditArtifactSHA256   string       `json:"audit_artifact_sha256"`
	AuditedTree           string       `json:"audited_tree"`
	LaneTree              string       `json:"lane_tree"`
	WorktreeBaseSHA       string       `json:"worktree_base_sha"`
	CommitSHA             string       `json:"commit_sha"`
	CommitTree            string       `json:"commit_tree"`
	ConsumedPaths         []string     `json:"consumed_paths"`
	ExplanationViewSHA256 string       `json:"explanation_view_sha256"`
	PreMain               string       `json:"pre_main"`
	Branch                string       `json:"branch"`
	LaneBranch            string       `json:"lane_branch"`
	Status                IntentStatus `json:"status"`
}

func IntentPath(evolveDir string, cycle int) string {
	return filepath.Join(evolveDir, intentDir, fmt.Sprintf("cycle-%d.json", cycle))
}

func WriteIntent(path string, in Intent) error {
	return writeJSON(path, "landing-intent.*.tmp", in)
}

func ReadIntent(path string) (Intent, bool, error) {
	body, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Intent{}, false, nil
	}
	if err != nil {
		return Intent{}, false, err
	}
	var in Intent
	if err := json.Unmarshal(body, &in); err != nil {
		return Intent{}, false, err
	}
	return in, true, nil
}
