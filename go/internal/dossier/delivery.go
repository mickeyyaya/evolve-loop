package dossier

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
)

const ShipBindingFile = "ship-binding.json"

type ShipBinding struct {
	AuditBoundTreeSHA string `json:"audit_bound_tree_sha,omitempty"`
	TreeSHACommitted  string `json:"tree_sha_committed,omitempty"`
	CommitSHA         string `json:"commit_sha"`
	Cycle             int    `json:"cycle,omitempty"`
}

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

func committedTasks(workspace string) ([]string, bool) {
	return committedset.Committed(workspace)
}
