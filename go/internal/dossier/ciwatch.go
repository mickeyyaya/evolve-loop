package dossier

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const CIWatchVerdictFile = "ci-watch-verdict.json"

type CIWatchRecord struct {
	SHA         string `json:"sha"`
	Conclusion  string `json:"conclusion"`
	RunURL      string `json:"run_url,omitempty"`
	FailingTest string `json:"failing_test,omitempty"`
	CheckedAt   string `json:"checked_at,omitempty"`
}

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
