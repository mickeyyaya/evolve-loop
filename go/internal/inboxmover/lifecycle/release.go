package lifecycle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

type drained struct {
	src, dest, base, taskID string
}

// Release drains processing/cycle-<cycle>/ to the inbox root; a non-nil Policy bumps failures and parks at the ceiling.
func (m *Mover) Release(cycle int, reason string, q *Policy) (RecoverResult, error) {
	if reason == "" {
		reason = "cycle-release"
	}
	res := RecoverResult{Paths: []string{}}
	cycleDir, ok, err := m.openCycleDir(cycle)
	if err != nil || !ok {
		return res, err
	}
	stamp := m.readStamp(cycle)
	files, _ := jsonEntries(cycleDir) // an unreadable dir is an empty release (preserved)
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
	for _, f := range files {
		d := drained{src: filepath.Join(cycleDir, f.Name()), dest: filepath.Join(m.inboxDir, f.Name()), base: f.Name()}
		d.taskID = readTaskIDOrUnknown(d.src)
		if parked, at := m.parkAtCeiling(d, reason, cycle, q); parked {
			res.Recovered++
			res.Paths = append(res.Paths, at)
			continue
		}
		if m.releaseOne(d, reason, cycle, stamp) {
			res.Recovered++
			res.Paths = append(res.Paths, d.dest)
		}
	}
	m.linef("release-cycle: %d file(s) released from cycle-%d", res.Recovered, cycle)
	return res, nil
}

func (m *Mover) openCycleDir(cycle int) (string, bool, error) {
	cycleDir := inboxbatch.ProcessingCycleDir(m.inboxDir, cycle)
	info, err := os.Stat(cycleDir)
	if err != nil {
		if os.IsNotExist(err) {
			m.linef("release-cycle: processing/cycle-%d/ absent — nothing to release", cycle)
			return "", false, nil
		}
		return "", false, fmt.Errorf("release-cycle: stat processing/cycle-%d: %w", cycle, err)
	}
	if !info.IsDir() {
		return "", false, nil
	}
	return cycleDir, true, nil
}

// readStamp reads the cycle's continuation manifest so the drain stamps each item in the same pass.
func (m *Mover) readStamp(cycle int) *continuation.Continuation {
	if m.runWorkspace == nil {
		return nil
	}
	ws := m.runWorkspace(cycle)
	c, ok, err := continuation.ReadManifest(ws)
	if err != nil {
		m.warn(fault{code: CodeContinuationManifestUnreadable, origin: "Mover.Release", cycle: cycle, legacy: "WARN: ",
			reason: fmt.Sprintf("release-cycle: continuation manifest unreadable for cycle %d: %v (items release unstamped)", cycle, err),
			fields: map[string]string{"workspace": ws, "err": err.Error(), "step": "manifest"}})
		return nil
	}
	if !ok {
		return nil
	}
	return &c
}

// parkAtCeiling bumps a committed item's failure_count and parks it in quarantine/ at the ceiling.
// Every fault falls open to a plain release and is reported: the item WILL be re-picked.
func (m *Mover) parkAtCeiling(d drained, reason string, cycle int, q *Policy) (bool, string) {
	if q == nil || q.SystemLevel || q.Routed || (q.Committed != nil && !q.Committed[d.taskID]) {
		return false, ""
	}
	count, bumpErr := bumpWith(d.src, reason, func(n int) bool { return ShouldQuarantine(n, q.Ceiling, q.SystemLevel) })
	if bumpErr != nil {
		m.warn(fault{code: CodeItemRewriteFailed, origin: "Mover.Release", cycle: cycle, legacy: "WARN: ",
			reason: fmt.Sprintf("release-cycle: failure_count bump failed for %s (%v) — quarantine skipped, releasing to inbox root", d.base, bumpErr),
			fields: map[string]string{"task_id": d.taskID, "path": d.src, "err": bumpErr.Error(), "step": "failure_bump"}})
		return false, ""
	}
	if !ShouldQuarantine(count, q.Ceiling, q.SystemLevel) {
		return false, ""
	}
	pr, pErr := m.Promote(d.taskID, "quarantine", PromoteOpts{Cycle: strconv.Itoa(cycle)})
	fields := map[string]string{"task_id": d.taskID, "failure_count": strconv.Itoa(count), "ceiling": strconv.Itoa(q.Ceiling), "step": "quarantine"}
	switch {
	case pErr != nil:
		fields["outcome"], fields["err"] = "error", pErr.Error()
		m.warn(fault{code: CodeQuarantineFailed, origin: "Mover.Release", cycle: cycle, legacy: "ERROR: ", fields: fields,
			reason: fmt.Sprintf("quarantine failed for '%s' (task-level failure #%d >= ceiling %d) — releasing to inbox root instead, it WILL be re-picked: %v", d.taskID, count, q.Ceiling, pErr)})
		return false, ""
	case pr.NoOp:
		fields["outcome"] = "noop"
		m.warn(fault{code: CodeQuarantineFailed, origin: "Mover.Release", cycle: cycle, legacy: "WARN: ", fields: fields,
			reason: fmt.Sprintf("quarantine no-op for '%s' (task-level failure #%d >= ceiling %d) — not parked, releasing to inbox root instead", d.taskID, count, q.Ceiling)})
		return false, ""
	}
	m.linef("quarantined: %s (task-level failure #%d >= ceiling %d) ← processing/cycle-%d/", d.base, count, q.Ceiling, cycle)
	return true, pr.DestPath
}

// releaseOne moves one item to the inbox root, never over a root twin, and reports whether it moved.
func (m *Mover) releaseOne(d drained, reason string, cycle int, stamp *continuation.Continuation) bool {
	if _, statErr := os.Stat(d.dest); statErr == nil {
		m.warn(fault{code: CodeReleaseDoubleMove, origin: "Mover.Release", cycle: cycle, legacy: "WARN: ",
			reason: fmt.Sprintf("release-cycle: %s already at inbox root (double-move for %s) — skipping", d.base, d.taskID),
			fields: map[string]string{"task_id": d.taskID, "base": d.base, "step": "release_cycle"}})
		return false
	}
	if stamp != nil {
		if serr := UpdateItemJSON(d.src, func(item map[string]json.RawMessage) {
			cb, _ := json.Marshal(stamp)
			item["continuation"] = cb
		}); serr != nil {
			m.warn(fault{code: CodeItemRewriteFailed, origin: "Mover.Release", cycle: cycle, legacy: "WARN: ",
				reason: fmt.Sprintf("release-cycle: continuation stamp failed for %s: %v (releasing unstamped)", d.base, serr),
				fields: map[string]string{"task_id": d.taskID, "path": d.src, "err": serr.Error(), "step": "continuation_stamp"}})
		}
	}
	if mvErr := os.Rename(d.src, d.dest); mvErr != nil {
		m.warn(fault{code: CodeReleaseMoveFailed, origin: "Mover.Release", cycle: cycle, legacy: "WARN: ",
			reason: fmt.Sprintf("release-cycle: mv failed for %s (leaving in processing/cycle-%d/): %v", d.base, cycle, mvErr),
			fields: map[string]string{"task_id": d.taskID, "base": d.base, "err": mvErr.Error(), "step": "release_cycle"}})
		return false
	}
	m.linef("released: %s ← processing/cycle-%d/", d.base, cycle)
	m.ledgerLine(ledgerEntry{
		Action: "recover",
		TaskID: d.taskID,
		From:   fmt.Sprintf(".evolve/inbox/processing/cycle-%d/%s", cycle, d.base),
		To:     ".evolve/inbox/" + d.base,
		Cycle:  intPtr(strconv.Itoa(cycle)),
		Reason: reason,
	})
	return true
}
