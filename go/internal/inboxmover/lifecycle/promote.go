package lifecycle

// promote.go — processing/ (or inbox/) → processed | rejected | retry |
// quarantine (inboxmover.go:261-430 on the base), split along its comment
// seams: validate → locate → the landing gate → deliver → the retire+ledger
// tail.

import (
	"fmt"
	"os"
	"path/filepath"
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
	NoOp     bool // true if source was not found (ship.sh compat → exit 0)
}

// promotion is one promote's resolved source and destination.
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

// Promote moves a file from processing/ (or inbox/ fallback) to
// processed|rejected|retry|quarantine/. Exits 0-equivalent even when source
// not found — ship.sh must never block on this.
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
		return res, nil // ship.sh compat: NoOp success
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
	// Transactional retire (park-consume-releases-continuation-binding): every
	// Promote destination is OUT of the batch loader's reach, so the item's
	// registry binding must go with it in this same operation (cycle-1487).
	// The reason override lands BEFORE the hook so the preserved pointer and
	// the ledger entry describe the same transaction with the same word
	// (audit cycle-1507 L1).
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

// validatePromote is the usage and state check (both lines kept verbatim —
// the stale `processed|rejected|retry` text is a preserved quirk).
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

// locateSource resolves the item — a processing claim first, then the inbox
// root (Locate is the one walk) — and the srcRel the ledger From path spells.
func (m *Mover) locateSource(taskID string) (src, srcRel string) {
	if loc, err := Locate(m.inboxDir, taskID); err == nil {
		src, srcRel = loc.Path, "inbox"
		if loc.Cycle > 0 {
			srcRel = "processing"
		}
	}
	return src, srcRel
}

// landingGate is the delivery-evidence gate (inbox-promotion-requires-landed-
// ship): a processed-promotion carrying a ship SHA must be backed by that
// commit actually landing on main, or it reroutes to retry/. Empty SHA and
// non-processed states skip the check. A probe error fails OPEN (the sha is
// treated as landed) — and, since the unit, says so.
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

// deliver creates the destination dir and renames the item. A mkdir failure
// is an INFRASTRUCTURE non-delivery: the task is still where it was and
// nothing promoted it, so it returns ErrMvFailed with NoOp false (a loud error
// that also lost the file would be worse — inboxmover-promote-mkdir-fail-loud).
// A rename failure keeps the historical (NoOp=true, nil) compat contract.
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

// promoteDestPath computes (destDir, dest) for a given new state:
//
//	processed:  <inbox>/processed/cycle-<cycle|0>/[<sha8>-]<base>
//	rejected:   <inbox>/rejected/cycle-<cycle|0>/<base>
//	retry:      <inbox>/retry/<base>
//	quarantine: <inbox>/quarantine/<base> (flat: terminal, not per-cycle)
func promoteDestPath(inboxDir, base, newState string, p PromoteOpts) (string, string) {
	switch newState {
	case "processed":
		destDir := filepath.Join(inboxDir, "processed", "cycle-"+cycleOrZero(p.Cycle))
		if p.CommitSHA != "" {
			sha8 := p.CommitSHA
			if len(sha8) > 8 {
				sha8 = sha8[:8]
			}
			return destDir, filepath.Join(destDir, sha8+"-"+base)
		}
		return destDir, filepath.Join(destDir, base)
	case "rejected":
		destDir := filepath.Join(inboxDir, "rejected", "cycle-"+cycleOrZero(p.Cycle))
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

// cycleOrZero is the ONE spelling of the destination's default cycle segment.
func cycleOrZero(cycle string) string {
	if cycle == "" {
		return "0"
	}
	return cycle
}
