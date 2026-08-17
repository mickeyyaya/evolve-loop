package inboxmover

// continuation_retire.go — the RELEASE half of the continuation-registry
// lifecycle (ADR-0076 slice C, G2). `continuation.DeleteRegistryEntry*` has
// existed since the 2026-08-10 immortal-entries stall, but nothing called it
// when an item LEFT the pending pool, so dispatch had two stores — inbox items
// and scope-keyed registry bindings — and every retirement path touched only
// one. Live burn: cycle-1487 parked `context-fill-telemetry-and-cap` out of
// .evolve/inbox and the next wave dispatched it anyway from cycle-1484's
// binding, burning a third lane on the same deterministic collision.
//
// Two rules, both here so the definitions cannot drift apart:
//
//  1. Retirement releases (releaseContinuationOnRetire) — the binding VALUE is
//     preserved into the retired item file's released_continuations[] first, so
//     the salvage pointer survives the release; the delete is
//     DeleteRegistryEntryIfCycle so a sibling lane that rebound the scope
//     between the read and the release keeps its fresh binding.
//  2. Liveness is the batch loader's own reach (scopeHasLiveItem) — an id is
//     LIVE iff it sits in the inbox ROOT (LoadDir's non-recursive scan) or in
//     processing/cycle-*/ (a lane currently holding it). consumed/, quarantine/,
//     processed/, rejected/ and retry/ are NOT live: LoadDir skips subdirs,
//     which is exactly why a parked item stops being picked.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

// releasedContinuation is the preserved pointer written onto a retired item.
// The Continuation is EMBEDDED (not nested under a new key) so the preserved
// value keeps the one schema every other continuation reader already parses —
// a registry-only shape is the drift the continuation package exists to
// prevent.
type releasedContinuation struct {
	continuation.Continuation
	ReleasedAt string `json:"released_at"`
	Reason     string `json:"reason"`
}

// releaseContinuationOnRetire releases taskID's registry binding as part of the
// SAME operation that took the item out of the pending pool, preserving the
// released value into the retired item at itemPath. Best-effort and LOUD like
// the rest of the lifecycle: a preservation or release problem never blocks the
// retirement that already happened, but it never happens silently either.
//
// Ordering is deliberate: preserve first, release second. A crash between the
// two leaves a live binding plus a preserved copy (the read-side guard in
// ResolveContinuationForScope then refuses it) — the reverse order would lose
// the pointer outright.
func releaseContinuationOnRetire(opts Options, itemPath, taskID, reason string) {
	if taskID == "" || taskID == "unknown" || opts.ProjectRoot == "" {
		return
	}
	c, ok, err := continuation.ReadRegistryEntry(opts.ProjectRoot, taskID)
	if err != nil {
		opts.logf("WARN: ", "continuation registry unreadable while retiring '%s' (%v) — binding NOT released, it will be refused at the next scope read", taskID, err)
		return
	}
	if !ok {
		return
	}
	// The preserved pointer rides the ship commit into a TRACKED .evolve/inbox
	// item on a public remote, so the absolute host paths are collapsed to "~"
	// first (audit cycle-1507 M1). Only Worktree/FindingsPath change; the
	// snapshot/base/branch refs salvage actually resumes from are untouched.
	if perr := appendReleasedContinuation(itemPath, continuation.RedactHostPaths(c), reason, opts.Now().UTC()); perr != nil {
		opts.logf("WARN: ", "retire '%s': preserved pointer (snapshot %s) NOT written to %s: %v — releasing the binding anyway", taskID, c.SnapshotSHA, itemPath, perr)
	}
	released, derr := continuation.DeleteRegistryEntryIfCycle(opts.ProjectRoot, taskID, c.Cycle)
	switch {
	case derr != nil:
		opts.logf("WARN: ", "retire '%s': continuation binding release failed: %v", taskID, derr)
	case !released:
		opts.logf("WARN: ", "retire '%s': continuation binding was rebound by another lane between read and release — left intact (cycle %d no longer owns it)", taskID, c.Cycle)
	default:
		opts.logf("", "retire '%s': released continuation binding (snapshot %s, cycle %d) — pointer preserved in %s", taskID, c.SnapshotSHA, c.Cycle, filepath.Base(itemPath))
	}
}

// appendReleasedContinuation appends c to path's released_continuations[],
// preserving every other field (updateItemJSON is atomic write-tmp + rename).
// An existing array that is not an array is replaced rather than dropped
// silently — the entry that matters is the one being written now.
func appendReleasedContinuation(path string, c continuation.Continuation, reason string, at time.Time) error {
	entry, err := json.Marshal(releasedContinuation{
		Continuation: c,
		ReleasedAt:   at.Format(time.RFC3339),
		Reason:       reason,
	})
	if err != nil {
		return err
	}
	return updateItemJSON(path, func(m map[string]json.RawMessage) {
		var list []json.RawMessage
		if raw, ok := m["released_continuations"]; ok {
			_ = json.Unmarshal(raw, &list)
		}
		list = append(list, entry)
		if out, merr := json.Marshal(list); merr == nil {
			m["released_continuations"] = out
		}
	})
}

// retirementDirs are the inbox subtrees an item lands in when it LEAVES the
// pending pool. LoadDir skips subdirs, which is exactly why an item parked here
// stops being picked — so a binding keyed on an id found only in one of these
// is a ghost. processed/ and rejected/ nest a cycle-N level (promoteDestPath),
// so the scan is recursive.
var retirementDirs = []string{"consumed", "quarantine", "processed", "rejected", "retry"}

// scopeHasLiveItem reports whether scopeID still names an item the batch loader
// can reach: the inbox ROOT (LoadDir's non-recursive scan) or a
// processing/cycle-*/ claim (a lane currently holding it).
func scopeHasLiveItem(opts Options, scopeID string) bool {
	if strings.TrimSpace(scopeID) == "" {
		return false
	}
	if _, err := FindFileByTaskID(opts.InboxDir, scopeID); err == nil {
		return true
	}
	entries, err := os.ReadDir(filepath.Join(opts.InboxDir, "processing"))
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "cycle-") {
			continue
		}
		if _, err := FindFileByTaskID(filepath.Join(opts.InboxDir, "processing", e.Name()), scopeID); err == nil {
			return true
		}
	}
	return false
}

// scopeRetiredAt returns the path of the retired copy holding scopeID and the
// retirement subtree it sits in, or ("", "") when the id is not retired
// anywhere. The PATH is returned as well as the subtree name because both
// read-side consumers need it: the guard preserves the released pointer into
// that exact file, and the recency test reads its cycle stamp.
//
// The guard refuses on POSITIVE retirement evidence, not on mere absence, and
// the distinction is load-bearing in both directions:
//
//   - A lane scope that names no inbox item at all is NOT proof of retirement —
//     the wave planner also mints lane scopes from carryoverTodos, which never
//     have an inbox file (the cycle-1078 orphan class this registry exists to
//     serve). Treating absence as death would trade the re-dispatch defect for
//     the salvage-loss defect: every carryover lane's preserved work released
//     out from under it.
//   - An id sitting in consumed/, quarantine/, processed/, rejected/ or retry/
//     IS proof: those are the pool exits, and that is the exact cycle-1487/1497
//     shape (item parked, binding immortal, lane minted anyway).
//
// Residual gap, stated rather than papered over: an item whose file was deleted
// outright leaves no evidence for this belt to find. That case is closed on the
// WRITE side by the transactional retire (releaseContinuationOnRetire /
// consume), which is the primary fix; this read-side guard is the belt.
func scopeRetiredAt(opts Options, scopeID string) (string, string) {
	if strings.TrimSpace(scopeID) == "" {
		return "", ""
	}
	for _, sub := range retirementDirs {
		root := filepath.Join(opts.InboxDir, sub)
		found := ""
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || found != "" {
				return nil
			}
			if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
				return nil
			}
			if itemIDAt(path) == scopeID {
				found = path
			}
			return nil
		})
		if found != "" {
			return found, sub
		}
	}
	return "", ""
}

// itemIDAt returns the .id an inbox item carries, or "" when unreadable.
func itemIDAt(path string) string {
	body, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var doc struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(body, &doc) != nil {
		return ""
	}
	return doc.ID
}
