package dossier

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
)

// ShipBindingFile is the ship phase's proof of delivery, written only after the
// commit exists. core.detectLostLanding keys on the same file.
const ShipBindingFile = "ship-binding.json"

// ShipBinding is the ship phase's delivery sidecar. phases/ship marshals this
// type, so a field rename is a compile error rather than an empty record.
type ShipBinding struct {
	AuditBoundTreeSHA string `json:"audit_bound_tree_sha,omitempty"`
	TreeSHACommitted  string `json:"tree_sha_committed,omitempty"`
	// CommitSHA has no omitempty on purpose: its emptiness is the "did not
	// deliver" signal.
	CommitSHA string `json:"commit_sha"`
	Cycle     int    `json:"cycle,omitempty"`
}

// shippedCommit returns the commit and tree the ship phase bound to this cycle;
// ok is false when the binding is absent or carries no commit.
func shippedCommit(workspace string) (commit, tree string, ok bool) {
	body, err := os.ReadFile(filepath.Join(workspace, ShipBindingFile))
	if err != nil {
		return "", "", false
	}
	var binding ShipBinding
	if json.Unmarshal(body, &binding) != nil {
		return "", "", false
	}
	commit = strings.TrimSpace(binding.CommitSHA)
	return commit, strings.TrimSpace(binding.TreeSHACommitted), commit != ""
}

// committedTasks delegates to internal/committedset, the single projection of
// what a cycle committed to (lane pin, else triage's top_n, minus deferrals).
func committedTasks(workspace string) ([]string, bool) {
	return committedset.Committed(workspace)
}
