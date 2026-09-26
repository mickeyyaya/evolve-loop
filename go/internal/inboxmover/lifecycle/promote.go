package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

// PromoteOpts gathers the optional flag-bearing arguments.
type PromoteOpts struct {
	Cycle     string // empty → "0"
	CommitSHA string // empty → no SHA prefix
}

// PromoteResult describes what happened.
type PromoteResult struct {
	SrcPath  string
	DestPath string
	NoOp     bool // nothing moved: the id was not found or the rename failed
}

type promotion struct {
	taskID  string
	src     string
	srcRel  string // "processing" for a claim, "inbox" for a root item (the ledger From quirk)
	base    string
	state   string
	destDir string
	dest    string
	opts    PromoteOpts
}

// Promote moves taskID's item to processed, rejected, retry or quarantine; an absent item is a NoOp success.
func (m *Mover) Promote(taskID, newState string, p PromoteOpts) (PromoteResult, error) {
	res := PromoteResult{}
	if err := m.validatePromote(taskID, newState); err != nil {
		return res, err
	}
	src, srcRel := m.locateSource(taskID)
	if src == "" {
		m.warn(fault{code: CodePromoteNotFound, origin: "Mover.Promote", cycle: cycleOf(p.Cycle), legacy: "WARN: ",
			reason: fmt.Sprintf("promote: task '%s' not found in processing/ or inbox/ — already moved?", taskID),
			fields: map[string]string{"task_id": taskID, "state": newState, "step": "locate"}})
		res.NoOp = true
		return res, nil // a missing item never blocks the ship
	}
	newState, reroutedUnlanded := m.landingGate(taskID, newState, p)
	pr := promotion{taskID: taskID, src: src, srcRel: srcRel, base: filepath.Base(src), state: newState, opts: p}
	pr.destDir, pr.dest = promoteDestPath(m.inboxDir, pr.base, newState, p)
	noop, err := m.deliver(pr)
	if err != nil || noop {
		res.NoOp = noop
		return res, err
	}
	res.SrcPath = src
	res.DestPath = pr.dest
	m.linef("promoted: %s → %s/", pr.base, newState)
	reason := "ship-promote-" + newState
	// The override precedes the retire hook so the preserved pointer and the ledger line carry one reason.
	if reroutedUnlanded {
		reason = "ship-promote-retry-unlanded-sha"
	}
	m.retire(pr.dest, taskID, reason)
	m.ledgerLine(ledgerEntry{
		Action: "promote",
		TaskID: taskID,
		From:   ".evolve/inbox/" + srcRel + "/" + pr.base,
		To:     pr.dest,
		Cycle:  intPtr(p.Cycle),
		GitSHA: strPtr(p.CommitSHA),
		Reason: reason,
	})
	return res, nil
}

// validatePromote's bad-state line omits quarantine on purpose: a pinned, preserved quirk.
func (m *Mover) validatePromote(taskID, newState string) error {
	if taskID == "" || newState == "" {
		m.linef("ERROR: usage: promote <task_id> <new_state> [<cycle>] [--commit-sha <sha>]")
		return fmt.Errorf("%w: promote requires task_id and new_state", ErrBadArgs)
	}
	if !validStates[newState] {
		m.linef("ERROR: promote: invalid state '%s'; must be processed|rejected|retry", newState)
		return fmt.Errorf("%w: %s", ErrBadState, newState)
	}
	return nil
}

func (m *Mover) locateSource(taskID string) (src, srcRel string) {
	if loc, err := Locate(m.inboxDir, taskID); err == nil {
		src, srcRel = loc.Path, "inbox"
		if loc.Cycle > 0 {
			srcRel = "processing"
		}
	}
	return src, srcRel
}

// landingGate reroutes a processed promotion to retry/ when its ship sha has not landed on main.
func (m *Mover) landingGate(taskID, newState string, p PromoteOpts) (string, bool) {
	if newState != "processed" || p.CommitSHA == "" {
		return newState, false
	}
	landed, err := m.landed(p.CommitSHA)
	if err != nil {
		landed = true // fail-open: never block a promotion on a gate error
		m.warn(fault{code: CodeLandedCheckFailed, origin: "Mover.Promote", cycle: cycleOf(p.Cycle), legacy: "WARN: ",
			reason: fmt.Sprintf("promote: landed check for '%s' failed (%v) — treating %s as landed (fail-open)", taskID, err, p.CommitSHA),
			fields: map[string]string{"task_id": taskID, "sha": p.CommitSHA, "err": err.Error(), "step": "landing"}})
	}
	if landed {
		return newState, false
	}
	m.warn(fault{code: CodePromoteUnlandedSHA, origin: "Mover.Promote", cycle: cycleOf(p.Cycle), legacy: "WARN: ",
		reason: fmt.Sprintf("promote: ship SHA %s for '%s' not landed on main — rerouting to retry/ instead of processed/", p.CommitSHA, taskID),
		fields: map[string]string{"task_id": taskID, "sha": p.CommitSHA, "state": "retry", "step": "landing"}})
	return "retry", true
}

// deliver moves the item into its destination. A mkdir failure is ErrMvFailed with the item in
// place; a rename failure stays the (NoOp=true, nil) success that callers depend on.
func (m *Mover) deliver(pr promotion) (noop bool, err error) {
	warnEntry := ledgerEntry{Action: "promote-warn", TaskID: pr.taskID, From: ".evolve/inbox/" + pr.srcRel + "/" + pr.base,
		To: pr.dest, Cycle: intPtr(pr.opts.Cycle), GitSHA: strPtr(pr.opts.CommitSHA)}
	fields := func(step string, err error) map[string]string {
		return map[string]string{"task_id": pr.taskID, "state": pr.state, "src_rel": pr.srcRel, "dest": pr.dest, "err": err.Error(), "step": step}
	}
	if err := os.MkdirAll(pr.destDir, 0o755); err != nil {
		m.warn(fault{code: CodePromoteMoveFailed, origin: "Mover.Promote", cycle: cycleOf(pr.opts.Cycle), legacy: "ERROR: ",
			reason: fmt.Sprintf("promote: mkdir -p '%s' failed — leaving file in %s/: %v", pr.destDir, pr.srcRel, err),
			fields: fields("mkdir", err)})
		warnEntry.Reason = "mkdir-failed"
		m.ledgerLine(warnEntry)
		return false, fmt.Errorf("%w: mkdir %s: %v", ErrMvFailed, pr.destDir, err)
	}
	if err := os.Rename(pr.src, pr.dest); err != nil {
		m.warn(fault{code: CodePromoteMoveFailed, origin: "Mover.Promote", cycle: cycleOf(pr.opts.Cycle), legacy: "WARN: ",
			reason: fmt.Sprintf("promote: mv failed for '%s' → %s (leaving in %s/): %v", pr.taskID, pr.state, pr.srcRel, err),
			fields: fields("rename", err)})
		warnEntry.Reason = "mv-failed"
		m.ledgerLine(warnEntry)
		return true, nil
	}
	return false, nil
}

// promoteDestPath returns the destination dir and path; quarantine is flat because it is terminal.
func promoteDestPath(inboxDir, base, newState string, p PromoteOpts) (string, string) {
	switch newState {
	case "processed":
		destDir := inboxbatch.CycleDir(filepath.Join(inboxDir, "processed"), cycleOrZero(p.Cycle))
		if p.CommitSHA != "" {
			sha8 := p.CommitSHA
			if len(sha8) > 8 {
				sha8 = sha8[:8]
			}
			return destDir, filepath.Join(destDir, sha8+"-"+base)
		}
		return destDir, filepath.Join(destDir, base)
	case "rejected":
		destDir := inboxbatch.CycleDir(filepath.Join(inboxDir, "rejected"), cycleOrZero(p.Cycle))
		return destDir, filepath.Join(destDir, base)
	case "retry":
		destDir := filepath.Join(inboxDir, "retry")
		return destDir, filepath.Join(destDir, base)
	case "quarantine":
		destDir := filepath.Join(inboxDir, "quarantine")
		return destDir, filepath.Join(destDir, base)
	}
	return "", ""
}

func cycleOrZero(cycle string) string {
	if cycle == "" {
		return "0"
	}
	return cycle
}
