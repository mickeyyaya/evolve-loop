package lifecycle

// release.go — the cycle drain (inboxmover.go:653-785 on the base): every
// *.json under processing/cycle-<cycle>/ back to the inbox root, with the
// ADR-0072 S5 bump-and-park when a Policy is given and the ADR-0076 slice-C
// continuation stamp when the cycle's workspace carries a manifest. Split
// into the dir open, the stamp read, the park and the single release.

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

// drained is one item of the drain: where it sits, where it goes, what it is.
type drained struct {
	src, dest, base, taskID string
}

// Release drains processing/cycle-<cycle>/ back to the inbox root. It is
// scoped to the single named cycle dir and idempotent: a missing or already-
// drained dir is a clean no-op. A basename already at the root (double-move
// race) is skipped — the root copy is never clobbered. An empty reason is
// "cycle-release". With a non-nil Policy it is the S5 failure drain: each
// committed item's durable failure_count is bumped first and, at the ceiling
// on a task-level failure, the item is parked in quarantine/ instead of
// released. Fail-open end to end: a per-item read/write fault falls back to a
// plain release so a bookkeeping fault never strands nor wrongly quarantines.
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

// openCycleDir resolves the cycle dir: absent is a logged no-op, a stat fault
// that is not absence is returned, a non-directory is a silent no-op.
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

// readStamp reads the cycle's continuation manifest (ADR-0076 slice C): when
// the FAILed cycle preserved salvageable work, every released item carries
// the stamp IN the release pass (transactional). Missing manifest ⇒ no-op; a
// corrupt one is reported and the items release unstamped.
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

// parkAtCeiling is the ADR-0072 S5 half of the drain: bump the committed
// item's durable failure_count (shedding its continuation stamp in the same
// atomic rewrite once the ceiling is reached — quarantine is terminal parking)
// and park it in quarantine/ at the ceiling. systemLevel gates the BUMP, not
// just the decision (AC4 in full). Every fault falls open to the plain release
// and says so: a bump that cannot rewrite, a park that cannot deliver — the
// un-parked poison item returns to the root and WILL be re-picked.
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

// releaseOne moves one item back to the root: the double-move guard, the
// continuation stamp (best-effort, reported), the rename, the INFO line and
// the ledger line. Reports whether the item moved.
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
