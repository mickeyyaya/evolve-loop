package lifecycle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ShouldQuarantine reports whether a task-level failure count reached a positive ceiling.
// See ADR-0072.
func ShouldQuarantine(failureCount, ceiling int, systemLevelFailure bool) bool {
	return ceiling > 0 && !systemLevelFailure && failureCount >= ceiling
}

// Policy holds a failure drain's quarantine inputs; a nil *Policy is a plain release.
type Policy struct {
	Ceiling     int
	SystemLevel bool            // a system-level failure: nothing bumps or parks
	Committed   map[string]bool // the ids that accrue failures; nil means every drained item
	// Routed means the closeout already routed the refused item: nothing bumps or parks,
	// and the failure is not mislabelled system-level.
	Routed bool
}

// ReleaseFromQuarantine moves a quarantined item back to the inbox root with its failure_count reset to 0.
func (m *Mover) ReleaseFromQuarantine(taskID string) (PromoteResult, error) {
	res := PromoteResult{}
	if taskID == "" {
		return res, fmt.Errorf("%w: release-from-quarantine requires task_id", ErrBadArgs)
	}
	qDir := filepath.Join(m.inboxDir, "quarantine")
	src, err := FindFileByTaskID(qDir, taskID)
	if err != nil {
		return res, fmt.Errorf("%w: %s (not in quarantine)", ErrNotFound, taskID)
	}
	base := filepath.Base(src)
	dest := filepath.Join(m.inboxDir, base)
	if _, statErr := os.Stat(dest); statErr == nil {
		return res, fmt.Errorf("%w: %s already at inbox root", ErrMvFailed, base)
	}
	if rerr := UpdateItemJSON(src, func(item map[string]json.RawMessage) {
		item["failure_count"] = json.RawMessage("0")
		delete(item, "last_failure_reason")
	}); rerr != nil {
		m.warn(fault{code: CodeItemRewriteFailed, origin: "Mover.ReleaseFromQuarantine", legacy: "WARN: ",
			reason: fmt.Sprintf("quarantine-release: failure_count reset failed for '%s' (%v) — released with its stale count", taskID, rerr),
			fields: map[string]string{"task_id": taskID, "path": src, "err": rerr.Error(), "step": "counter_reset"}})
	}
	if mvErr := os.Rename(src, dest); mvErr != nil {
		return res, fmt.Errorf("%w: %v", ErrMvFailed, mvErr)
	}
	res.SrcPath = src
	res.DestPath = dest
	m.linef("released from quarantine: %s → inbox/", base)
	m.ledgerLine(ledgerEntry{
		Action: "quarantine-release",
		TaskID: taskID,
		From:   ".evolve/inbox/quarantine/" + base,
		To:     ".evolve/inbox/" + base,
		Reason: "operator-quarantine-release",
	})
	return res, nil
}
