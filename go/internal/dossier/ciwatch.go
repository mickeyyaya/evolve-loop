package dossier

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// CIWatchVerdictFile is the workspace artifact the post-push CI watch
// (internal/ciwatch) writes and Build ingests — the durable evidence that the
// remote GitHub CI verdict for the cycle's pushed SHA was actually read
// (cycle-748, push-ci-watch-remote-parity).
const CIWatchVerdictFile = "ci-watch-verdict.json"

// CIWatchRecord is the remote CI verdict for the cycle's pushed commit.
// Defined here (not in internal/ciwatch) so the dossier stays the SSOT type
// the watch writes and Build reads — single source with projection.
type CIWatchRecord struct {
	SHA         string `json:"sha"`
	Conclusion  string `json:"conclusion"`
	RunURL      string `json:"run_url,omitempty"`
	FailingTest string `json:"failing_test,omitempty"`
	CheckedAt   string `json:"checked_at,omitempty"`
}

// ciWatchRecord reads the CI-watch verdict artifact from the workspace.
// Returns ok=false when the artifact is absent or unusable — the verdict is
// evidence and is never fabricated.
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
