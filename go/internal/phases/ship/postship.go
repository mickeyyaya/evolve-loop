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
	csPath := filepath.Join(opts.ProjectRoot, ".evolve", "cycle-state.json")
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
	stMap, err := readStateMap(stPath)
	if err != nil {
		return err
	}
	stMap["lastCycleNumber"] = cid
	if err := writeStateMap(stPath, stMap); err != nil {
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
	csPath := filepath.Join(opts.ProjectRoot, ".evolve", "cycle-state.json")
	csMap, err := readStateMap(csPath)
	if err != nil {
		return err
	}
	cid, ok := stateInt(csMap, "cycle_id")
	if !ok {
		return nil
	}
	triagePath := filepath.Join(opts.ProjectRoot, ".evolve", "runs", fmt.Sprintf("cycle-%d", cid), "triage-decision.json")
	if _, err := os.Stat(triagePath); err != nil {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] INFO: no triage-decision.json for cycle %d — inbox promote skipped", cid))
		return nil
	}
	// Promote top_n[] + skip_shipped[] — read IDs from triage-decision.json.
	body, err := os.ReadFile(triagePath)
	if err != nil {
		return err
	}
	ids := extractIDs(body)
	if len(ids) == 0 {
		return nil
	}
	commitShort := ""
	if len(res.CommitSHA) >= 8 {
		commitShort = res.CommitSHA[:8]
	}
	mvOpts := inboxmover.Options{
		ProjectRoot: opts.ProjectRoot,
		Stderr:      opts.Stderr,
	}
	for _, id := range ids {
		_, _ = inboxmover.Promote(mvOpts, id, "processed", inboxmover.PromoteOpts{
			Cycle:     fmt.Sprintf("%d", cid),
			CommitSHA: commitShort,
		})
	}
	res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: inbox lifecycle promote complete for cycle %d", cid))
	return nil
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
	stMap, err := readStateMap(statePath)
	if err != nil {
		return err
	}
	expected := stateString(stMap, "expected_ship_sha")
	if expected == postSHA {
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
}
