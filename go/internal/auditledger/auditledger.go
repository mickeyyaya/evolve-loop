// Package auditledger is the one reader of auditor rows in
// .evolve/ledger.jsonl.
package auditledger

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

var ErrNoAuditorForRun = errors.New("no Auditor ledger entry")

type Entry struct {
	TS              string `json:"ts"`
	Role            string `json:"role"`
	Kind            string `json:"kind"`
	RunID           string `json:"run_id,omitempty"`
	ExitCode        int    `json:"exit_code"`
	ArtifactPath    string `json:"artifact_path"`
	ArtifactSHA256  string `json:"artifact_sha256"`
	GitHEAD         string `json:"git_head"`
	TreeStateSHA    string `json:"tree_state_sha"`
	WorktreeTreeSHA string `json:"worktree_tree_sha"`
	WorktreeBaseSHA string `json:"worktree_base_sha"`
}

func AuditorRows(ledgerPath string) ([]Entry, error) {
	raw, err := os.ReadFile(ledgerPath)
	if err != nil {
		return nil, fmt.Errorf("read ledger %s: %w", ledgerPath, err)
	}
	lines := strings.Split(string(raw), "\n")
	var rows []Entry
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		if IsAuditorRow(e) {
			rows = append(rows, e)
		}
	}
	return rows, nil
}

func BindRun(rows []Entry, runID string) (Entry, error) {
	if runID == "" {
		if len(rows) > 0 {
			return rows[0], nil
		}
		return Entry{}, fmt.Errorf("%w found", ErrNoAuditorForRun)
	}
	for _, e := range rows {
		if e.RunID == runID {
			return e, nil
		}
	}
	if len(rows) == 0 {
		return Entry{}, fmt.Errorf("%w for run %s", ErrNoAuditorForRun, runID)
	}
	foreign := rows[0].RunID
	if foreign == "" {
		foreign = "<unstamped>"
	}
	return Entry{}, fmt.Errorf("%w for run %s (refused to bind foreign run %s, git_head=%s)",
		ErrNoAuditorForRun, runID, foreign, rows[0].GitHEAD)
}

func LatestAuditorEntry(ledgerPath, runID string) (Entry, error) {
	rows, err := AuditorRows(ledgerPath)
	if err != nil {
		return Entry{}, err
	}
	return BindRun(rows, runID)
}

func (e Entry) AuditedBase() string {
	if e.WorktreeBaseSHA != "" {
		return e.WorktreeBaseSHA
	}
	return e.GitHEAD
}

func IsAuditorRow(e Entry) bool {
	return e.Kind == "agent_subprocess" && e.Role == "auditor"
}
