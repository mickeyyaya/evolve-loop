package ship

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

// consumeCommittedItems moves this cycle's committed inbox items into the
// tracked consumed/ dir inside the ship tree and stages the moves so they
// ride the ship commit. Every step is per-item fail-open and logs loudly: a
// consumption problem must never block a ship that already earned its verdict.
func consumeCommittedItems(ctx context.Context, opts *Options, res *RunResult, dir string) {
	switch opts.Class {
	case ClassCycle:
		// A lane ship always has a cycle behind it, so workspace evidence is
		// mandatory; its absence is logged loudly below.
	case ClassManual:
		// A manual ship with no WorkspacePath has nothing to consume and
		// returns silently; the loud missing-verdict warn below is for ships
		// that claim a workspace and can't prove a PASS.
		if opts.WorkspacePath == "" {
			return
		}
	default:
		return
	}
	if v := workspaceACSVerdict(opts.WorkspacePath); v != "PASS" {
		if v == "" {
			// A silent no-consume here would let the re-pick class resurrect
			// the item invisibly after a verdict-path drift.
			res.Logs = append(res.Logs, "[ship] WARN: inbox consumption skipped: acs-verdict.json missing/unreadable in workspace — no verdict, no consuming")
		} else {
			res.Logs = append(res.Logs, fmt.Sprintf("[ship] inbox consumption skipped: acs verdict %s (only PASS consumes — partial work stays pickable)", v))
		}
		return
	}
	root := dir
	var prefix []string
	if dir == "" {
		root = opts.ProjectRoot
	} else {
		prefix = []string{"-C", dir}
	}
	body, note := triageDecisionBytes(opts.WorkspacePath, 0)
	if body == nil && note != "" {
		res.Logs = append(res.Logs, note)
	}
	ids := committedInboxIDs(opts.WorkspacePath, body, true)
	if len(ids) == 0 {
		return
	}
	cid := stateString(mustStateMap(opts), "cycle_id")
	inboxDir := filepath.Join(root, ".evolve", "inbox")
	consumedRel := ".evolve/inbox/consumed"
	for _, id := range ids {
		src, err := inboxmover.FindFileByTaskID(inboxDir, id)
		if err != nil {
			if !errors.Is(err, inboxmover.ErrNotFound) {
				res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume lookup %q: %v", id, err))
			}
			continue
		}
		raw, err := os.ReadFile(src)
		if err != nil {
			res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume read %q: %v", id, err))
			continue
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume parse %q: %v — leaving item in place", id, err))
			continue
		}
		doc["consumed"] = map[string]any{
			"at":    time.Now().UTC().Format(time.RFC3339),
			"via":   "ship",
			"cycle": cid,
		}
		bound, boundOK := readBindingForConsume(opts, res, id)
		if boundOK {
			doc["released_continuations"] = appendReleasedForConsume(doc, bound, cid)
		}
		out, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume marshal %q: %v", id, err))
			continue
		}
		base := filepath.Base(src)
		dstAbs := filepath.Join(root, filepath.FromSlash(consumedRel), base)
		if err := os.MkdirAll(filepath.Dir(dstAbs), 0o755); err != nil {
			res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume mkdir for %q: %v", id, err))
			continue
		}
		if err := os.WriteFile(dstAbs, append(out, '\n'), 0o644); err != nil {
			res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume write %q: %v", id, err))
			continue
		}
		if err := os.Remove(src); err != nil {
			res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume remove root %q: %v", id, err))
			_ = os.Remove(dstAbs) // do not leave a half-move
			continue
		}
		srcRel := ".evolve/inbox/" + base
		dstRel := consumedRel + "/" + base
		if opts.internalAuditBoundTreeSHA != "" {
			boundArgs := append(append([]string{}, prefix...), "show", opts.internalAuditBoundTreeSHA+":"+srcRel)
			var boundOut strings.Builder
			exit, runErr := opts.run(ctx, "git", boundArgs, &boundOut, io.Discard)
			if runErr != nil || exit != 0 || boundOut.String() != string(raw) {
				_ = os.Remove(dstAbs)
				if werr := os.WriteFile(src, raw, 0o644); werr != nil {
					res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume provenance rollback write %q: %v", id, werr))
				}
				res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume %q REFUSED sanctioning: item bytes are not what the audit-bound tree carries (absent or tampered post-binding) — move rolled back, item stays pickable", id))
				continue
			}
		}
		args := append(append([]string{}, prefix...), "add", "-A", "--", srcRel, dstRel)
		if exit, runErr := opts.run(ctx, "git", args, io.Discard, io.Discard); runErr != nil || exit != 0 {
			// Roll back: on shipDirect this tree IS the runtime plane, so an
			// unstaged deletion would strand state no commit records.
			_ = os.Remove(dstAbs)
			if werr := os.WriteFile(src, raw, 0o644); werr != nil {
				res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume rollback write %q: %v", id, werr))
			}
			res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume stage %q rc=%d err=%v — move rolled back, item stays pickable", id, exit, runErr))
			continue
		}
		opts.internalConsumedPaths = append(opts.internalConsumedPaths, srcRel, dstRel)
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: consumed inbox item %q into %s (rides this ship commit)", id, dstRel))
		if boundOK {
			releaseBindingForConsume(opts, res, id, bound)
		}
	}
}

// readBindingForConsume best-effort reads id's continuation binding from the
// root-owned registry (opts.ProjectRoot, even when consumption runs in a ship
// worktree); an unreadable registry leaves the binding in place rather than
// blocking a ship that earned its verdict.
func readBindingForConsume(opts *Options, res *RunResult, id string) (continuation.Continuation, bool) {
	c, ok, err := continuation.ReadRegistryEntry(opts.ProjectRoot, id)
	if err != nil {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume %q: continuation registry unreadable (%v) — binding NOT released", id, err))
		return continuation.Continuation{}, false
	}
	return c, ok
}

// appendReleasedForConsume returns doc's released_continuations[] with the
// released binding appended, preserving any entries an earlier retirement left.
// The consumed item rides the ship commit to the public remote, so the
// absolute host paths are collapsed to "~" first: worktree/findings_path carry
// the operator account name, while the snapshot, base and branch refs salvage
// resumes from are unaffected.
func appendReleasedForConsume(doc map[string]any, c continuation.Continuation, cycleID string) []any {
	return continuation.AppendReleased(doc, c, "ship-consume-"+cycleID)
}

// releaseBindingForConsume deletes the binding once the consumption move is
// staged. DeleteRegistryEntryIfCycle (not the unconditional delete) so a
// sibling lane that rebound the scope between the read and here keeps its
// fresh binding — the TOCTOU lost-update the registry's own doc calls out.
func releaseBindingForConsume(opts *Options, res *RunResult, id string, c continuation.Continuation) {
	released, err := continuation.DeleteRegistryEntryIfCycle(opts.ProjectRoot, id, c.Cycle)
	switch {
	case err != nil:
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume %q: continuation binding release failed: %v", id, err))
	case !released:
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume %q: continuation binding was rebound by another lane (cycle %d no longer owns it) — left intact", id, c.Cycle))
	default:
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: released continuation binding for %q (snapshot %s, cycle %d) — pointer preserved in the consumed item", id, c.SnapshotSHA, c.Cycle))
	}
}

// treeDriftExplainedByConsumption reports whether every path differing between
// the audit-bound tree and the actual tree is one of the ship's own sanctioned
// consumption moves. It fails closed: no consumed paths, or a diff-tree that
// cannot run, explains nothing. The second return names the unsanctioned
// drift paths for the refusal message ("" when there are none).
func treeDriftExplainedByConsumption(ctx context.Context, opts *Options, gitDir, boundTree, actualTree string) (bool, string) {
	if len(opts.internalConsumedPaths) == 0 {
		return false, ""
	}
	sanctioned := make(map[string]bool, len(opts.internalConsumedPaths))
	for _, p := range opts.internalConsumedPaths {
		sanctioned[p] = true
	}
	args := []string{}
	if gitDir != "" {
		args = append(args, "-C", gitDir)
	}
	// rawPathRead/unquoteGitPath: without them a non-ASCII byte in an item
	// filename comes back C-quoted, never matches the sanctioned set, and
	// false-refuses a legitimate consumption ship.
	args = append(args, rawPathRead("diff-tree", "-r", "--name-only", boundTree, actualTree)...)
	var out strings.Builder
	if exit, err := opts.run(ctx, "git", args, &out, io.Discard); err != nil || exit != 0 {
		return false, ""
	}
	var offenders []string
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		p := unquoteGitPath(strings.TrimSpace(line))
		if p == "" {
			continue
		}
		if !sanctioned[p] {
			offenders = append(offenders, p)
		}
	}
	if len(offenders) == 0 {
		return true, ""
	}
	// Bounded: the refusal travels into digests/escalations.
	const maxNamed = 8
	extra := ""
	if len(offenders) > maxNamed {
		extra = fmt.Sprintf(" and %d more", len(offenders)-maxNamed)
		offenders = offenders[:maxNamed]
	}
	return false, " (unsanctioned drift path(s): " + strings.Join(offenders, ", ") + extra + ")"
}

// workspaceACSVerdict reads the deterministic verdict the gate stamped, or ""
// when unreadable (fail-closed for consumption: no verdict, no consuming).
func workspaceACSVerdict(workspace string) string {
	raw, err := os.ReadFile(filepath.Join(workspace, "acs-verdict.json"))
	if err != nil {
		return ""
	}
	var doc struct {
		Verdict string `json:"verdict"`
	}
	if json.Unmarshal(raw, &doc) != nil {
		return ""
	}
	return doc.Verdict
}

// mustStateMap best-effort reads the cycle state map for annotation fields —
// never blocks consumption ("" cycle id on failure).
func mustStateMap(opts *Options) map[string]any {
	m, err := readStateMap(opts.cycleStateFile())
	if err != nil {
		return map[string]any{}
	}
	return m
}
