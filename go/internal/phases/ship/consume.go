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
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

// consumeCommittedItems moves this cycle's committed inbox items into the
// tracked consumed/ dir INSIDE the ship tree and stages the moves, so they
// ride the ship commit. Gated on ClassCycle + a PASS acs verdict (WARN ships
// under the fluent posture but the work may be partial — the item must stay
// pickable). Every step is per-item fail-open and LOUD: a consumption problem
// must never block a ship that already earned its verdict.
func consumeCommittedItems(ctx context.Context, opts *Options, res *RunResult, dir string) {
	if opts.Class != ClassCycle {
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
	ids := extractIDs(body)
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
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: consumed inbox item %q into %s (rides this ship commit)", id, dstRel))
	}
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
