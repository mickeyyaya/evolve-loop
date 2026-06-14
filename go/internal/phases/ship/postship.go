// postship.go — post-ship hooks: lastCycleNumber advance, inbox
// lifecycle promote, post-cycle self-update SHA repin.
//
// Mirrors ship.sh sections 7-9 trailing logic (lines 843-958).
package ship

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

// postShip runs the side-effects that follow a successful commit+push.
// Errors are returned but the caller (Run) treats them as WARNings, not
// ship failures — the commit is already on remote.
func postShip(ctx context.Context, opts *Options, res *RunResult) error {
	if opts.DryRun {
		return nil
	}

	if opts.Class == ClassCycle {
		if err := advanceLastCycleNumber(opts, res); err != nil {
			return err
		}
		if err := promoteInbox(ctx, opts, res); err != nil {
			// Inbox promote failures never block ship (idempotency in Triage Step 0a).
			res.Logs = append(res.Logs, "[ship] WARN: inbox promote: "+err.Error())
		}
		if err := repinPostCycle(opts, res); err != nil {
			return err
		}
	}

	res.Logs = append(res.Logs, fmt.Sprintf("[ship] DONE: shipped %s at %s", res.ClassUsed, res.CommitSHA))
	return nil
}

// advanceLastCycleNumber reads cycle-state.json:cycle_id and writes it
// into state.json:lastCycleNumber atomically. Only fires for class=cycle.
//
// This is the v8.34.0 fix for stuck-counter: pre-v8.34, only failure
// paths wrote lastCycleNumber, so successful ships left the counter at
// the previous cycle → dispatcher's next iteration computed
// ran_cycle = last_before + 1 = the SAME cycle just shipped → 5-repeat
// circuit-breaker fired prematurely on legitimate runs.
func advanceLastCycleNumber(opts *Options, res *RunResult) error {
	csPath := opts.cycleStateFile() // ADR-0049 S3 / G3: run-scoped (cycle_id)
	stPath := filepath.Join(opts.ProjectRoot, ".evolve", "state.json")
	csMap, err := readStateMap(csPath)
	if err != nil {
		return err
	}
	cid, ok := stateInt(csMap, "cycle_id")
	if !ok {
		// No cycle_id → nothing to advance. Bash silently skips.
		return nil
	}
	// ADR-0049 S2 / G2: serialize the state.json RMW under the shared lock so
	// it can't lose (or be lost to) a concurrent allocator/UpdateState write.
	// Preserve the pre-lock contract: a READ error propagates (fail ship);
	// only a write/lock error is the non-fatal WARN.
	var readErr error
	lockErr := withStateLock(stPath, func() error {
		stMap, err := readStateMap(stPath)
		if err != nil {
			readErr = err
			return err
		}
		stMap["lastCycleNumber"] = cid
		return writeStateMap(stPath, stMap)
	})
	if readErr != nil {
		return readErr
	}
	if lockErr != nil {
		res.Logs = append(res.Logs, "[ship] WARN: could not advance lastCycleNumber (state.json write failed)")
		return nil // WARN — don't fail ship
	}
	res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: advanced state.json:lastCycleNumber to %d", cid))
	return nil
}

// promoteInbox calls the inboxmover Go library directly (v11.8.1+; prior
// versions shelled out to legacy/scripts/lifecycle/inbox-mover.sh). Moves
// shipped inbox tasks to processed/. Best-effort: failures log WARN and
// don't block ship (Layer 1 idempotency catches residual in next cycle's
// Triage).
func promoteInbox(ctx context.Context, opts *Options, res *RunResult) error {
	csPath := opts.cycleStateFile() // ADR-0049 S3 / G3: run-scoped (cycle_id)
	csMap, err := readStateMap(csPath)
	if err != nil {
		return err
	}
	cid, ok := stateInt(csMap, "cycle_id")
	if !ok {
		return nil
	}
	mvOpts := inboxmover.Options{
		ProjectRoot: opts.ProjectRoot,
		Stderr:      opts.Stderr,
	}

	// Promote top_n[] + skip_shipped[] to processed/. The companion the agent is
	// instructed to emit is in practice almost never written (cycles 308/316/
	// 320-322 all missing it), so triageDecisionBytes DETERMINISTICALLY PROJECTS
	// it from triage-report.md when absent — single source, guaranteed present
	// (triage-decision-json-not-emitted; ADR-0047 single-source-with-projection).
	cycleDir := filepath.Join(opts.ProjectRoot, ".evolve", "runs", fmt.Sprintf("cycle-%d", cid))
	body, logLine := triageDecisionBytes(cycleDir, cid)
	res.Logs = append(res.Logs, logLine)
	if body != nil {
		commitShort := ""
		if len(res.CommitSHA) >= 8 {
			commitShort = res.CommitSHA[:8]
		}
		for _, id := range extractIDs(body) {
			_, _ = inboxmover.Promote(mvOpts, id, "processed", inboxmover.PromoteOpts{
				Cycle:     fmt.Sprintf("%d", cid),
				CommitSHA: commitShort,
			})
		}
	}

	// ALWAYS drain residual claims: every item still in processing/cycle-<cid>/
	// is released back to the inbox root so the next cycle's triage re-scans it
	// (Step 0a reads only inbox/ root, maxdepth 1). This MUST run even when
	// triage-decision.json is absent — the early-return that used to skip it
	// stranded EVERY claimed item invisibly (inbox-promote-on-ship-missing;
	// orphans across cycles 124/265/294/295/308).
	if _, releaseErr := inboxmover.ReleaseCycleProcessing(mvOpts, cid); releaseErr != nil {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: residual claim release for cycle %d: %v", cid, releaseErr))
	}
	res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: inbox lifecycle drain complete for cycle %d", cid))
	return nil
}

// triageDecisionBytes returns the cycle's triage-decision.json bytes for
// promotion plus a log line. Preference order:
//  1. the agent-authored companion if present (carries skip_shipped, the
//     git-log-verified resolution signal the markdown cannot express);
//  2. otherwise a deterministic projection of triage-report.md — guaranteed
//     present so promote-to-processed runs every cycle (the projection emits
//     top_n only; skip_shipped is empty, so it promotes exactly what a shipped
//     cycle committed to);
//  3. nil when neither exists — promotion is skipped, the residual drain (the
//     caller's safety net) still releases claims.
func triageDecisionBytes(cycleDir string, cid int) ([]byte, string) {
	companion := filepath.Join(cycleDir, "triage-decision.json")
	body, err := os.ReadFile(companion)
	if err == nil {
		return body, fmt.Sprintf("[ship] OK: triage-decision.json present for cycle %d — promoting", cid)
	}
	if !os.IsNotExist(err) {
		// Present but unreadable (corrupt/permission) — distinct from absent: a
		// real IO error keeps its WARN signal, never demoted to INFO.
		return nil, fmt.Sprintf("[ship] WARN: triage-decision.json unreadable for cycle %d (%v) — promote-to-processed skipped (residual claims still drained)", cid, err)
	}
	// Absent — project the companion from the report below.
	report, err := os.ReadFile(filepath.Join(cycleDir, triagecap.TriageArtifactName()))
	if err != nil {
		return nil, fmt.Sprintf("[ship] INFO: no triage-decision.json or report for cycle %d — promote-to-processed skipped (residual claims still drained)", cid)
	}
	body, perr := triagecap.ProjectDecisionJSON(string(report), cid)
	if perr != nil {
		return nil, fmt.Sprintf("[ship] WARN: triage-decision projection failed for cycle %d (%v) — promote-to-processed skipped (residual claims still drained)", cid, perr)
	}
	// Persist so downstream readers (a re-run, forensics) see the same companion.
	_ = os.WriteFile(companion, body, 0o644)
	return body, fmt.Sprintf("[ship] OK: projected triage-decision.json for cycle %d from the report (agent omitted it)", cid)
}

// extractIDs walks triage-decision.json JSON and returns the union of
// .top_n[].id and .skip_shipped[].task_id (deduped, order-preserving).
func extractIDs(body []byte) []string {
	var d struct {
		TopN []struct {
			ID string `json:"id"`
		} `json:"top_n"`
		SkipShipped []struct {
			TaskID string `json:"task_id"`
		} `json:"skip_shipped"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil
	}
	seen := map[string]struct{}{}
	out := []string{}
	for _, e := range d.TopN {
		if e.ID != "" {
			if _, dup := seen[e.ID]; !dup {
				seen[e.ID] = struct{}{}
				out = append(out, e.ID)
			}
		}
	}
	for _, e := range d.SkipShipped {
		if e.TaskID != "" {
			if _, dup := seen[e.TaskID]; !dup {
				seen[e.TaskID] = struct{}{}
				out = append(out, e.TaskID)
			}
		}
	}
	return out
}

// repinPostCycle handles the case where the just-shipped commit
// modified the ship binary itself. The on-disk SHA has changed; the
// next cycle's TOFU would fail. Re-pin to the new SHA.
//
// Mirrors ship.sh lines 947-958.
func repinPostCycle(opts *Options, res *RunResult) error {
	binPath := opts.ShipBinaryPath
	if binPath == "" {
		var err error
		binPath, err = os.Executable()
		if err != nil {
			return nil // best-effort
		}
	}

	var postSHA string
	relBin, relErr := filepath.Rel(opts.ProjectRoot, binPath)
	if relErr == nil && !strings.HasPrefix(relBin, "..") {
		postSHA = committedBinSHA(context.Background(), opts, filepath.ToSlash(relBin))
	}

	if postSHA == "" {
		var err error
		postSHA, err = sha256File(binPath)
		if err != nil {
			return nil
		}
	}

	statePath := filepath.Join(opts.ProjectRoot, ".evolve", "state.json")
	// ADR-0049 S2 / G2: serialize the whole read→check→write under the shared
	// state.json lock. Any error (lock/read/write) propagates, as before.
	return withStateLock(statePath, func() error {
		stMap, err := readStateMap(statePath)
		if err != nil {
			return err
		}
		if stateString(stMap, "expected_ship_sha") == postSHA {
			return nil
		}
		pluginVer := pluginVersion(opts.PluginRoot)
		stMap["expected_ship_sha"] = postSHA
		stMap["expected_ship_version"] = pluginVer
		if err := writeStateMap(statePath, stMap); err != nil {
			return err
		}
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] TOFU: post-cycle self-update (ship binary changed in this commit) — pinned ship binary SHA + plugin version='%s'", pluginVer))
		return nil
	})
}
