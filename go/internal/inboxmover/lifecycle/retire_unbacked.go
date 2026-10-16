package lifecycle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

var unbackedStates = map[string]bool{"processed": true, "rejected": true}

type unbackedRecord struct {
	ID       string `json:"id"`
	Unbacked bool   `json:"unbacked"`
	Reason   string `json:"retired_reason"`
	Cycle    int    `json:"retired_cycle"`
	GitSHA   string `json:"git_sha,omitempty"`
}

func (m *Mover) RetireUnbacked(taskID, newState string, p PromoteOpts, reason string) (string, error) {
	if taskID == "" || taskID == "." || taskID == ".." || filepath.Base(taskID) != taskID {
		return "", fmt.Errorf("%w: retire-unbacked needs a plain task id, got %q", ErrBadArgs, taskID)
	}
	if !unbackedStates[newState] {
		return "", fmt.Errorf("%w: retire-unbacked: %q is neither processed nor rejected", ErrBadState, newState)
	}
	destDir, dest := promoteDestPath(m.inboxDir, taskID+".json", newState, p)
	if _, err := os.Stat(dest); err == nil {
		return dest, nil
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("retire-unbacked: mkdir %s: %w", destDir, err)
	}
	body, _ := json.Marshal(unbackedRecord{ID: taskID, Unbacked: true, Reason: reason, Cycle: cycleOf(p.Cycle), GitSHA: p.CommitSHA})
	if err := writeRecord(dest, body); err != nil {
		return "", fmt.Errorf("retire-unbacked: write %s: %w", dest, err)
	}
	m.linef("retired unbacked: %s → %s/ (%s)", taskID, newState, reason)
	m.ledgerLine(ledgerEntry{Action: "retire-unbacked", TaskID: taskID, To: dest, Cycle: intPtr(p.Cycle), GitSHA: strPtr(p.CommitSHA), Reason: reason})
	return dest, nil
}

func writeRecord(dest string, body []byte) error {
	tmp := fmt.Sprintf("%s.tmp.%d", dest, os.Getpid())
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return commitTmp(tmp, dest)
}
