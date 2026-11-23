// Package auditledger is the one reader of auditor rows in
// .evolve/ledger.jsonl.
package auditledger

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/ledgerscan"
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
	var rows []Entry
	err := scanAuditors(ledgerPath, func(e Entry) bool {
		rows = append(rows, e)
		return false
	})
	return rows, err
}

func scanAuditors(ledgerPath string, visit func(Entry) (stop bool)) error {
	err := ledgerscan.Reverse(ledgerPath, auditorFilter(), func(line []byte) (bool, error) {
		trimmed := bytes.TrimSpace(line)
		var e Entry
		if len(trimmed) == 0 || json.Unmarshal(trimmed, &e) != nil || !IsAuditorRow(e) {
			return false, nil
		}
		return visit(e), nil
	})
	if err != nil {
		return fmt.Errorf("read ledger %s: %w", ledgerPath, err)
	}
	return nil
}

func BindRun(rows []Entry, runID string) (Entry, error) {
	binder := newRunBinder(runID)
	for _, e := range rows {
		if binder.offer(e) {
			break
		}
	}
	return binder.result()
}

type runBinder struct {
	runID  string
	newest Entry
	seen   bool
	bound  bool
}

func newRunBinder(runID string) *runBinder { return &runBinder{runID: runID} }

func (b *runBinder) offer(e Entry) (stop bool) {
	if !b.seen {
		b.newest, b.seen = e, true
	}
	if b.runID == "" || e.RunID == b.runID {
		b.newest, b.bound = e, true
	}
	return b.bound
}

func (b *runBinder) result() (Entry, error) {
	switch {
	case b.bound:
		return b.newest, nil
	case !b.seen && b.runID == "":
		return Entry{}, fmt.Errorf("%w found", ErrNoAuditorForRun)
	case !b.seen:
		return Entry{}, fmt.Errorf("%w for run %s", ErrNoAuditorForRun, b.runID)
	}
	foreign := b.newest.RunID
	if foreign == "" {
		foreign = "<unstamped>"
	}
	return Entry{}, fmt.Errorf("%w for run %s (refused to bind foreign run %s, git_head=%s)",
		ErrNoAuditorForRun, b.runID, foreign, b.newest.GitHEAD)
}

func LatestAuditorEntry(ledgerPath, runID string) (Entry, error) {
	binder := newRunBinder(runID)
	if err := scanAuditors(ledgerPath, binder.offer); err != nil {
		return Entry{}, err
	}
	return binder.result()
}

func (e Entry) AuditedBase() string {
	if e.WorktreeBaseSHA != "" {
		return e.WorktreeBaseSHA
	}
	return e.GitHEAD
}

func IsAuditorRow(e Entry) bool {
	return e.Kind == AuditorKind && e.Role == AuditorRole
}

const (
	AuditorKind = "agent_subprocess"
	AuditorRole = "auditor"
)

func auditorFilter() ledgerscan.Filter {
	return ledgerscan.Filter{Needles: [][]byte{ledgerscan.ContainsValue(AuditorKind), ledgerscan.ContainsValue(AuditorRole)}}
}
