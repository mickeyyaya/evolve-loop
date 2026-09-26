// Package auditledger is the one reader of auditor rows in .evolve/ledger.jsonl.
// It owns the row schema, the row identity (kind=agent_subprocess, role=auditor)
// and the run-scope rule; each consumer maps its errors onto its own vocabulary.
// It is a leaf: it must not import a consumer or internal/core.
package auditledger

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// ErrNoAuditorForRun reports a ledger that holds no auditor row bindable to the requested run.
var ErrNoAuditorForRun = errors.New("no Auditor ledger entry")

// Entry is an auditor row of the ledger.
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
}

// AuditorRows returns every auditor row of the ledger, newest first; a read failure wraps the os error.
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
		// An unparseable line is not an auditor row: alien lines must not break a reader.
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		if e.Kind == "agent_subprocess" && e.Role == "auditor" {
			rows = append(rows, e)
		}
	}
	return rows, nil
}

// BindRun returns the first of rows (newest first) stamped with runID, or the first row when runID is empty.
// A miss wraps ErrNoAuditorForRun and names the newest refused row.
func BindRun(rows []Entry, runID string) (Entry, error) {
	if runID == "" {
		if len(rows) > 0 {
			return rows[0], nil
		}
		return Entry{}, fmt.Errorf("%w found", ErrNoAuditorForRun)
	}
	// An unstamped row never binds a run: the ledger is shared by every fleet lane.
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

// LatestAuditorEntry returns the newest auditor row of runID's run, or of any run when runID is empty.
func LatestAuditorEntry(ledgerPath, runID string) (Entry, error) {
	rows, err := AuditorRows(ledgerPath)
	if err != nil {
		return Entry{}, err
	}
	return BindRun(rows, runID)
}
