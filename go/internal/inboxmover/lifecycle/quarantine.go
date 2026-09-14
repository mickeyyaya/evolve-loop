package lifecycle

// quarantine.go — the ADR-0072 S5 decision, the drain's park policy and the
// operator's release out of quarantine/ (inboxmover.go:432-487, :636-651).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ShouldQuarantine is the pure ADR-0072 S5 decision: quarantine a task once its
// task-level failure count reaches the configured ceiling. A zero (or negative)
// ceiling disables quarantine entirely, and a system-level failure NEVER
// quarantines — the S3 floor halt takes precedence (AC4). The caller passes the
// ceiling (FailureThresholds.TaskRetryCeiling, default 2) and the system-level
// flag; the leaf deliberately does not import internal/policy so the package
// layering stays intact.
func ShouldQuarantine(failureCount, ceiling int, systemLevelFailure bool) bool {
	return ceiling > 0 && !systemLevelFailure && failureCount >= ceiling
}

// Policy carries the ADR-0072 S5 decision inputs for a failure drain: the
// task-level retry ceiling and whether this cycle's failure was system-level
// (an S3 floor halt), which suppresses the bump and the quarantine (AC4).
// Committed restricts the failure_count bump (and therefore quarantine) to the
// ids triage actually COMMITTED to the cycle; nil means "every item in the
// drain" — the legacy whole-dir behavior an outcome with no committed ids
// still selects (wave lanes claim a whole menu but work only the committed
// subset). A nil *Policy is the plain release-to-root drain.
type Policy struct {
	Ceiling     int
	SystemLevel bool
	Committed   map[string]bool
	// Routed: the closeout already routed the refused item console-manual, so
	// this cycle's disposition is taken — the drain bumps and parks nothing,
	// without pretending the failure was system-level (a separate knob so the
	// next behaviour keyed to SystemLevel never applies to a routed refusal).
	Routed bool
}

// ReleaseFromQuarantine is the operator escape hatch for ADR-0072 S5: it moves
// an item out of quarantine/ back to the inbox root and resets its
// failure_count to 0, so the next cycle's triage can re-pick it. Returns
// ErrNotFound when no quarantined item carries taskID. Idempotent-safe: a
// basename already present at the inbox root is left untouched (never
// clobbered) and reported as ErrMvFailed. The counter reset precedes the
// rename (a preserved quirk: a failed rename leaves a zeroed quarantined item)
// and is best-effort — a rewrite failure never blocks the release, but since
// the unit it is reported.
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
