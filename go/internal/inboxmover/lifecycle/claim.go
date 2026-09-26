package lifecycle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

// ClaimResult describes what happened.
type ClaimResult struct {
	SrcPath  string
	DestPath string
}

// Claim moves taskID's item from the inbox root into processing/cycle-<cycle>/; an absent id is ErrNotFound.
func (m *Mover) Claim(taskID, cycle string) (ClaimResult, error) {
	res := ClaimResult{}
	if taskID == "" || cycle == "" {
		m.linef("ERROR: usage: claim <task_id> <cycle>")
		return res, fmt.Errorf("%w: claim requires task_id and cycle", ErrBadArgs)
	}
	src, err := FindFileByTaskID(m.inboxDir, taskID)
	if err != nil {
		m.warn(fault{code: CodeClaimNotFound, origin: "Mover.Claim", cycle: cycleOf(cycle), legacy: "WARN: ",
			reason: fmt.Sprintf("claim: task '%s' not found in %s", taskID, m.inboxDir),
			fields: map[string]string{"task_id": taskID, "inbox_dir": m.inboxDir, "step": "locate"}})
		return res, fmt.Errorf("%w: %s", ErrNotFound, taskID)
	}
	if reason := consoleRoutedReason(src, m.isProtected); reason != "" {
		m.warn(fault{code: CodeClaimRefused, origin: "Mover.Claim", cycle: cycleOf(cycle), legacy: "WARN: ",
			reason: fmt.Sprintf("claim: task '%s' REFUSED — %s (operator-owned; lanes must not draw it)", taskID, reason),
			fields: map[string]string{"task_id": taskID, "reason": reason, "step": "route"}})
		return res, fmt.Errorf("%w: %s (%s)", ErrConsoleRouted, taskID, reason)
	}
	cycleNum, convErr := strconv.Atoi(cycle)
	if convErr != nil || cycleNum < 1 {
		m.linef("ERROR: claim: cycle %q is not a positive number", cycle)
		return res, fmt.Errorf("%w: claim cycle must be a positive number, got %q", ErrBadArgs, cycle)
	}
	return m.claimInto(taskID, src, cycle, cycleNum)
}

func (m *Mover) claimInto(taskID, src, cycle string, cycleNum int) (ClaimResult, error) {
	res := ClaimResult{}
	base := filepath.Base(src)
	destDir := inboxbatch.ProcessingCycleDir(m.inboxDir, cycleNum)
	dest := filepath.Join(destDir, base)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		m.warn(fault{code: CodeClaimMoveFailed, origin: "Mover.Claim", cycle: cycleNum, legacy: "ERROR: ",
			reason: fmt.Sprintf("claim: mkdir -p '%s' failed: %v", destDir, err),
			fields: map[string]string{"task_id": taskID, "dest_dir": destDir, "err": err.Error(), "step": "mkdir"}})
		return res, fmt.Errorf("%w: mkdir: %v", ErrMvFailed, err)
	}
	if err := os.Rename(src, dest); err != nil {
		m.warn(fault{code: CodeClaimMoveFailed, origin: "Mover.Claim", cycle: cycleNum, legacy: "WARN: ",
			reason: fmt.Sprintf("claim: mv failed for '%s' (may already be claimed): %v", taskID, err),
			fields: map[string]string{"task_id": taskID, "err": err.Error(), "step": "rename"}})
		return res, fmt.Errorf("%w: %v", ErrMvFailed, err)
	}
	res.SrcPath = src
	res.DestPath = dest
	m.linef("claimed: %s → processing/cycle-%s/", base, cycle)
	m.ledgerLine(ledgerEntry{
		Action: "claim",
		TaskID: taskID,
		From:   ".evolve/inbox/" + base,
		To:     ".evolve/inbox/processing/cycle-" + cycle + "/" + base,
		Cycle:  intPtr(cycle),
		Reason: "triage-claim",
	})
	return res, nil
}

// consoleRoutedReason returns why the item is operator-owned, or "" when it is dispatchable.
// An unreadable or malformed item fails open so routing never bricks claiming; LoadDir reports it.
func consoleRoutedReason(path string, isProtected func(string) bool) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var it inboxbatch.Item
	if json.Unmarshal(raw, &it) != nil {
		return ""
	}
	routed, reason := inboxbatch.ConsoleRouted(it, isProtected)
	if !routed {
		return ""
	}
	return reason
}
