package ship

// Cross-plane note (review): the ship commit's tracked root deletion can meet
// a runtime plane whose LOCAL copy carries an unstaged failure-count
// write-back — `git merge origin/main` then aborts (modified-vs-delete).
// Recovery: `git checkout -- .evolve/inbox/<file>` and re-merge; the window is
// narrow (claimed items leave the root at dispatch) but real for
// root-resident failure-bumped items consumed via skip_shipped.
//
// consume.go — transactional inbox consumption (consumption-rides-landing-ship
// 0.92; live burns: cycles 1448, 1464, 1471). The PASS closeout promotes items
// to the GITIGNORED processed/ on the runtime plane AFTER the commit, so main
// keeps the tracked item and every fresh lane worktree re-picks it. The class
// fix: the PASS ship commit ITSELF carries the consumption — the tracked root
// file moves to the tracked inbox/consumed/ with a consumption annotation, and
// both paths are staged into the very commit that lands the work. The post-ship
// promotion then resolves the id nowhere and takes its documented idempotent
// no-op path.

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
// tracked consumed/ dir INSIDE the ship tree and stages the moves, so they
// ride the ship commit. Gated on ClassCycle + a PASS acs verdict (WARN ships
// under the fluent posture but the work may be partial — the item must stay
// pickable). Every step is per-item fail-open and LOUD: a consumption problem
// must never block a ship that already earned its verdict.
func consumeCommittedItems(ctx context.Context, opts *Options, res *RunResult, dir string) {
	switch opts.Class {
	case ClassCycle:
		// The lane path: workspace evidence is mandatory, and its absence is
		// LOUD below — a lane ship always has a cycle behind it.
	case ClassManual:
		// consumption_must_ride_the_landing, the manual half (the lane half
		// shipped as #466; cycle-1547's red-first reproduction pins this one):
		// a reviewed console ship that closes an inbox item must retire that
		// item IN THE LANDING COMMIT, or every later cycle rediscovers it —
		// three recorded live burns. A manual ship WITHOUT cycle evidence
		// (ordinary console commit, no WorkspacePath) has nothing to consume
		// and returns silently: the loud missing-verdict warn below is for
		// ships that CLAIM a workspace and can't prove a PASS.
		if opts.WorkspacePath == "" {
			return
		}
	default:
		return
	}
	if v := workspaceACSVerdict(opts.WorkspacePath); v != "PASS" {
		if v == "" {
			// LOUD fail-closed (review): a silent no-consume here is how the
			// re-pick class resurrects invisibly after a verdict-path drift.
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
		res.Logs = append(res.Logs, note) // corrupt/absent decision stays visible
	}
	// Same resolver the post-ship promotion uses — precedence and rationale live
	// once, at committedInboxIDs (postship.go). Reading triage alone here is what
	// kept consumption at zero for eight cycles on carryover-driven lanes.
	//
	// Blast radius worth knowing: this id set also drives the continuation-binding
	// release below (readBindingForConsume / released_continuations / the registry
	// delete), so lane-scope and Closes-Inbox ids now reach it too. That is
	// consistent with shipped behavior — postship's retireCommittedCarryover
	// already retires the carryover twins for the identical set — but it is a
	// wider set than the triage-only one this path used to see.
	ids := committedInboxIDs(opts.WorkspacePath, body, true) // this site is PASS-gated above
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
			continue // absent = already consumed or never tracked — a no-op
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
		// Transactional retire, registry half (park-consume-releases-continuation-
		// binding): consumption takes the item out of the batch loader's reach, so
		// its scope-keyed continuation binding must go too — otherwise the next
		// wave mints a lane straight off the immortal binding (cycles 1487, 1497).
		// The pointer is PRESERVED into the consumed item here; the registry delete
		// happens only after the move is staged, so a rollback below leaves both
		// stores exactly as they were.
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
		// Review HIGH (cycle-1506 fix hardening): when an audit binding is set,
		// the drift tolerance downstream will SANCTION these paths — so the
		// consumed bytes must be exactly what the AUDITED tree carried. A file
		// planted or tampered in the post-binding window (the one window the
		// binding floor exists to close) rolls back loudly and stays pickable;
		// it does not ship unaudited. shipDirect (no binding, no drift checks)
		// is untouched — plane-side failure-count write-backs keep consuming.
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
			// Roll the fs move BACK (review): on the shipDirect path this tree
			// IS the runtime plane — an unstaged deletion would strand plane
			// state no commit records. raw is still in hand.
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

// readBindingForConsume reads id's continuation binding from the ROOT-owned
// registry (always opts.ProjectRoot — the registry is root-owned even when the
// consumption itself happens in a ship worktree). Best-effort and LOUD: an
// unreadable registry leaves the binding in place, where the read-side live-
// scope guard refuses it, rather than blocking a ship that earned its verdict.
func readBindingForConsume(opts *Options, res *RunResult, id string) (continuation.Continuation, bool) {
	c, ok, err := continuation.ReadRegistryEntry(opts.ProjectRoot, id)
	if err != nil {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: consume %q: continuation registry unreadable (%v) — binding NOT released", id, err))
		return continuation.Continuation{}, false
	}
	return c, ok
}

// appendReleasedForConsume returns doc's released_continuations[] with the
// released binding appended, preserving any entries an earlier retirement left
// (a non-array value is replaced — the entry being written now is the one that
// carries the live salvage pointer).
// The consumed item is TRACKED and rides the ship commit to the public remote,
// so the absolute host paths are collapsed to "~" first (audit cycle-1507 M1):
// worktree/findings_path carry the operator account name, while the snapshot,
// base and branch refs salvage resumes from are unaffected.
func appendReleasedForConsume(doc map[string]any, c continuation.Continuation, cycleID string) []any {
	c = continuation.RedactHostPaths(c)
	var list []any
	if prev, ok := doc["released_continuations"].([]any); ok {
		list = prev
	}
	return append(list, map[string]any{
		"worktree":      c.Worktree,
		"branch":        c.Branch,
		"snapshot_sha":  c.SnapshotSHA,
		"base_sha":      c.BaseSHA,
		"findings_path": c.FindingsPath,
		"cycle":         c.Cycle,
		"released_at":   time.Now().UTC().Format(time.RFC3339),
		"reason":        "ship-consume-" + cycleID,
	})
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
// consumption moves (opts.internalConsumedPaths). gitDir=="" runs in the
// process cwd (the direct/plane path); otherwise `git -C gitDir`. Fail-CLOSED:
// no consumed paths, or a diff-tree that cannot run, explains nothing — the
// caller then refuses exactly as before. The second return is a rendered
// offender suffix for the refusal message ("" when nothing to add), so the
// operator sees WHICH unsanctioned paths drifted.
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
	// rawPathRead/unquoteGitPath (cycle-1108): without them a non-ASCII byte in
	// an item filename comes back C-quoted, never matches the sanctioned set,
	// and false-refuses a legitimate consumption ship (review M4).
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
	// Bounded suffix (review LOW): the refusal travels into digests/escalations.
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
