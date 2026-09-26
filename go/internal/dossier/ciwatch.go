package dossier

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// CIWatchVerdictFile is the workspace artifact the post-push CI watch writes
// and Build ingests.
const CIWatchVerdictFile = "ci-watch-verdict.json"

// CIWatchRecord is the remote CI verdict for the cycle's pushed commit. The
// watch writes this type, so writer and reader share one definition.
type CIWatchRecord struct {
	SHA         string `json:"sha"`
	Conclusion  string `json:"conclusion"`
	RunURL      string `json:"run_url,omitempty"`
	FailingTest string `json:"failing_test,omitempty"`
	CheckedAt   string `json:"checked_at,omitempty"`
}

// ciWatchRecord reads the CI-watch verdict; ok is false when it is absent,
// malformed, or lacks a SHA or conclusion.
func ciWatchRecord(workspace string) (*CIWatchRecord, bool) {
	body, err := os.ReadFile(filepath.Join(workspace, CIWatchVerdictFile))
	if err != nil {
		return nil, false
	}
	var rec CIWatchRecord
	if json.Unmarshal(body, &rec) != nil ||
		strings.TrimSpace(rec.SHA) == "" || strings.TrimSpace(rec.Conclusion) == "" {
		return nil, false
	}
	return &rec, true
}
