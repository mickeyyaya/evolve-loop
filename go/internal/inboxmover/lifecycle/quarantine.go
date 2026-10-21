package lifecycle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func ShouldQuarantine(failureCount, ceiling int, systemLevelFailure bool) bool {
	return ceiling > 0 && !systemLevelFailure && failureCount >= ceiling
}

type Policy struct {
	Ceiling      int
	SystemLevel  bool
	Committed    map[string]bool
	Routed       bool
	RoutedIDs    map[string]bool
	FailureClass string
	Fingerprint  string
}

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
		item[inboxbatch.FailureCountField] = json.RawMessage("0")
		delete(item, inboxbatch.LastFailureReasonField)
		delete(item, inboxbatch.LastFailureClassField)
		delete(item, inboxbatch.LastFailureFingerprintField)
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
