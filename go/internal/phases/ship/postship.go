package ship

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

func postShip(ctx context.Context, opts *Options, res *RunResult) error {
	if opts.DryRun {
		return nil
	}

	if opts.Class == ClassCycle {
		if err := advanceLastCycleNumber(opts, res); err != nil {
			return err
		}
		if err := promoteInbox(ctx, opts, res); err != nil {
			res.Logs = append(res.Logs, "[ship] WARN: inbox promote: "+err.Error())
		}
		if err := repinPostCycle(opts, res); err != nil {
			return err
		}
	}

	res.Logs = append(res.Logs, fmt.Sprintf("[ship] DONE: shipped %s at %s", res.ClassUsed, res.CommitSHA))
	return nil
}

func retireCommittedCarryover(opts *Options, res *RunResult, cid int, committedIDs []string) {
	if len(committedIDs) == 0 {
		return
	}
	stPath := filepath.Join(opts.ProjectRoot, ".evolve", "state.json")
	if _, err := os.Stat(stPath); err != nil {
		return
	}
	var retiredCount int
	if err := withStateLock(stPath, func() error {
		stMap, err := readStateMap(stPath)
		if err != nil {
			return err
		}
		raw, ok := stMap["carryoverTodos"].([]any)
		if !ok || len(raw) == 0 {
			return nil
		}
		todos := make([]core.CarryoverTodo, len(raw))
		for i, e := range raw {
			em, _ := e.(map[string]any)
			todos[i] = core.CarryoverTodo{ID: stateString(em, "id"), Action: stateString(em, "action")}
		}
		kept := core.RetireCarryoverTodos(todos, committedIDs)
		if len(kept) == len(todos) {
			return nil
		}
		keep := make(map[string]bool, len(kept))
		for _, t := range kept {
			keep[t.ID] = true
		}
		out := make([]any, 0, len(kept))
		for i, e := range raw {
			if keep[todos[i].ID] {
				out = append(out, e)
			}
		}
		retiredCount = len(raw) - len(out)
		stMap["carryoverTodos"] = out
		return writeStateMap(stPath, stMap)
	}); err != nil {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: carryover retirement for cycle %d: %v", cid, err))
		return
	}
	if retiredCount > 0 {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: retired %d carryover todo(s) for cycle %d: %v", retiredCount, cid, committedIDs))
	}
}

func advanceLastCycleNumber(opts *Options, res *RunResult) error {
	csPath := opts.cycleStateFile()
	stPath := filepath.Join(opts.ProjectRoot, ".evolve", "state.json")
	csMap, err := readStateMap(csPath)
	if err != nil {
		return err
	}
	cid, ok := stateInt(csMap, "cycle_id")
	if !ok {
		return nil
	}
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
		return nil
	}
	res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: advanced state.json:lastCycleNumber to %d", cid))
	return nil
}

func promoteInbox(ctx context.Context, opts *Options, res *RunResult) error {
	csPath := opts.cycleStateFile()
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

	cycleDir := filepath.Join(opts.ProjectRoot, ".evolve", "runs", fmt.Sprintf("cycle-%d", cid))
	body, logLine := triageDecisionBytes(cycleDir, cid)
	res.Logs = append(res.Logs, logLine)
	unlandedShip := false
	commitShort := ""
	if len(res.CommitSHA) >= 8 {
		commitShort = res.CommitSHA[:8]
	}
	var committedIDs []string
	var laneFallbackIDs []string
	if body == nil {
		if laneFallbackIDs = cycleoutcome.LaneScopeIDs(cycleDir); len(laneFallbackIDs) > 0 {
			res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: no triage decision for cycle %d — committed set from lane-scope pin: %v", cid, laneFallbackIDs))
		}
	}
	markerIDs := inboxmover.ClosesInboxIDs(readBuildReport(cycleDir))
	if len(markerIDs) > 0 {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: build-report Closes-Inbox marker for cycle %d: %v", cid, markerIDs))
	}
	if body != nil || len(laneFallbackIDs) > 0 || len(markerIDs) > 0 {
		if res.CommitSHA != "" && !isLanded(ctx, opts, res.CommitSHA) {
			unlandedShip = true
			res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: promotion skipped: unlanded — commit %s is not an ancestor of HEAD or origin; inbox items for cycle %d left in processing/ for re-triage", commitShort, cid))
		} else {
			res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: promoted: landed — commit %s verified in durable history for cycle %d", commitShort, cid))
			committedIDs = committedInboxIDs(cycleDir, body, workspaceACSVerdict(cycleDir) == "PASS")
			if retired, rErr := inboxmover.ReconcileSuperseded(mvOpts, inboxmover.SupersededInboxIDs(body), "processed", inboxmover.PromoteOpts{
				Cycle:     fmt.Sprintf("%d", cid),
				CommitSHA: commitShort,
			}); rErr != nil {
				res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: superseded reconcile for cycle %d: %v", cid, rErr))
			} else if len(retired) > 0 {
				res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: retired %d superseded inbox item(s) for cycle %d: %v", len(retired), cid, retired))
			}
			retireCommittedCarryover(opts, res, cid, committedIDs)
		}
	}

	releaseReason := ""
	if unlandedShip {
		releaseReason = "cycle-release-unlanded-ship-retry"
	}
	if or, outcomeErr := inboxmover.ApplyCycleOutcome(mvOpts, inboxmover.CycleOutcome{
		Cycle:        cid,
		Passed:       true,
		CommittedIDs: committedIDs,
		CommitSHA:    commitShort,
		Reason:       releaseReason,
	}); outcomeErr != nil {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: inbox outcome apply for cycle %d: %v", cid, outcomeErr))
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: inbox lifecycle drain INCOMPLETE for cycle %d — items may remain in processing/ for re-triage", cid))
	} else {
		if len(or.Promoted) > 0 {
			res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: promoted %d committed inbox item(s) for cycle %d: %v", len(or.Promoted), cid, or.Promoted))
		}
		if len(or.RetiredUnbacked) > 0 {
			res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: retired %d committed id(s) no inbox item backs for cycle %d: %v", len(or.RetiredUnbacked), cid, or.RetiredUnbacked))
		}
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: inbox lifecycle drain complete for cycle %d", cid))
	}
	return nil
}

func isLanded(ctx context.Context, opts *Options, sha string) bool {
	if strings.TrimSpace(sha) == "" {
		return false
	}
	if isAncestor(ctx, opts, sha, "HEAD") {
		return true
	}
	branch, _ := currentBranch(ctx, opts)
	if branch == "" {
		return false
	}
	return isAncestor(ctx, opts, sha, "origin/"+branch)
}

func readBuildReport(cycleDir string) []byte {
	body, err := os.ReadFile(filepath.Join(cycleDir, "build-report.md"))
	if err != nil {
		return nil
	}
	return body
}

func unionIDs(a, b []string) []string {
	seen := make(map[string]bool, len(a))
	for _, id := range a {
		seen[id] = true
	}
	for _, id := range b {
		if !seen[id] {
			seen[id] = true
			a = append(a, id)
		}
	}
	return a
}

func triageDecisionBytes(cycleDir string, cid int) ([]byte, string) {
	companion := filepath.Join(cycleDir, "triage-decision.json")
	body, err := os.ReadFile(companion)
	if err == nil {
		return body, fmt.Sprintf("[ship] OK: triage-decision.json present for cycle %d — promoting", cid)
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Sprintf("[ship] WARN: triage-decision.json unreadable for cycle %d (%v) — promote-to-processed skipped (residual claims still drained)", cid, err)
	}
	report, err := os.ReadFile(filepath.Join(cycleDir, triagecap.TriageArtifactName()))
	if err != nil {
		return nil, fmt.Sprintf("[ship] INFO: no triage-decision.json or report for cycle %d — promote-to-processed skipped (residual claims still drained)", cid)
	}
	body, perr := triagecap.ProjectDecisionJSON(string(report), cid)
	if perr != nil {
		return nil, fmt.Sprintf("[ship] WARN: triage-decision projection failed for cycle %d (%v) — promote-to-processed skipped (residual claims still drained)", cid, perr)
	}
	_ = os.WriteFile(companion, body, 0o644)
	return body, fmt.Sprintf("[ship] OK: projected triage-decision.json for cycle %d from the report (agent omitted it)", cid)
}

func extractIDs(body []byte) []string {
	return inboxmover.CommittedIDs(body)
}

func committedInboxIDs(cycleDir string, body []byte, landedPASS bool) []string {
	ids := inboxmover.CommittedIDs(body)
	if !landedPASS {
		if body == nil {
			ids = cycleoutcome.LaneScopeIDs(cycleDir)
		}
		return unionIDs(ids, inboxmover.ClosesInboxIDs(readBuildReport(cycleDir)))
	}
	deferred := map[string]bool{}
	for _, id := range inboxmover.DeferredIDs(body) {
		deferred[id] = true
	}
	closedDrop := map[string]bool{}
	for _, id := range inboxmover.ClosedDroppedIDs(body) {
		closedDrop[id] = true
	}
	committed := map[string]bool{}
	for _, id := range ids {
		committed[id] = true
	}
	scopeIDs := cycleoutcome.LaneScopeIDs(cycleDir)
	engagedByName := false
	for _, id := range scopeIDs {
		if committed[id] {
			engagedByName = true
			break
		}
	}
	scopeEligible := body == nil || len(ids) > 0
	var scope []string
	for _, id := range scopeIDs {
		if deferred[id] {
			continue
		}
		if closedDrop[id] || (scopeEligible && (!engagedByName || committed[id])) {
			scope = append(scope, id)
		}
	}
	ids = unionIDs(ids, scope)
	return unionIDs(ids, inboxmover.ClosesInboxIDs(readBuildReport(cycleDir)))
}

func repinPostCycle(opts *Options, res *RunResult) error {
	binPath := opts.ShipBinaryPath
	if binPath == "" {
		var err error
		binPath, err = os.Executable()
		if err != nil {
			return nil
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
