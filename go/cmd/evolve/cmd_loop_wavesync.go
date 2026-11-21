package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxstamps"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainsync"
	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const waveSyncTimeout = 60 * time.Second

var errMainCIRed = errors.New("main CI red")

type mainCheckRuns struct {
	CheckRuns []struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
	} `json:"check_runs"`
}

func mainCIRedForSHA(ctx context.Context, projectRoot, sha string) (bool, []string, error) {
	cmd := sysexec.Command(ctx, "gh", "api", "--method", "GET",
		"repos/{owner}/{repo}/commits/"+sha+"/check-runs", "-f", "filter=latest", "-f", "per_page=100")
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, nil, fmt.Errorf("gh api check-runs: %w: %s", err, strings.TrimSpace(string(out)))
	}
	var payload mainCheckRuns
	if err := json.Unmarshal(out, &payload); err != nil {
		return false, nil, fmt.Errorf("decode check-runs: %w", err)
	}
	completed := 0
	var failed []string
	for _, run := range payload.CheckRuns {
		if run.Status != "completed" {
			continue
		}
		completed++
		switch run.Conclusion {
		case "success", "neutral", "skipped":
		default:
			failed = append(failed, run.Name+"="+run.Conclusion)
		}
	}
	if completed == 0 {
		return false, nil, fmt.Errorf("no completed check-runs visible for %.12s", sha)
	}
	return len(failed) > 0, failed, nil
}

func syncMainFromOriginAtWaveBoundary(ctx context.Context, projectRoot string, warn io.Writer) (synced bool, halt error) {
	if info, err := plane.Classify(projectRoot); err != nil || info.Branch != "main" {
		return false, nil
	}
	sctx, cancel := context.WithTimeout(ctx, waveSyncTimeout)
	defer cancel()
	g := gitexec.Git{Dir: projectRoot, Exec: sysexec.DefaultRunner}
	if _, _, code, err := g.Capture(sctx, "remote", "get-url", "origin"); err != nil || code != 0 {
		return false, nil
	}
	res := mainsync.Run(sctx, g, mainsync.Policy{Gate: waveCIGate(projectRoot, warn), Around: waveInboxStamps(warn)})
	return reportWaveSync(warn, res)
}

func reportWaveSync(warn io.Writer, res mainsync.Result) (synced bool, halt error) {
	rel := res.Relation
	switch res.Outcome {
	case mainsync.OutcomeFetchFailed:
		fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: fetch origin failed (rc=%d %v: %s) — planning against the local main\n", res.Code, res.Err, res.Stderr)
	case mainsync.OutcomeRelationFailed:
		fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: %v — planning against the local main\n", res.Err)
	case mainsync.OutcomeGated:
		return false, res.Err
	case mainsync.OutcomeAhead:
		fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: %s; the next lane ship's push publishes them\n", rel)
	case mainsync.OutcomeDiverged:
		fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: %s\n", rel)
		return false, fmt.Errorf("wave-boundary sync: %s", rel)
	case mainsync.OutcomeFastForwarded:
		fmt.Fprintf(warn, "[loop] wave-boundary sync: fast-forwarded main to origin/main (%.12s)\n", rel.Remote)
		reportAbsorbed(warn, "[loop] wave-boundary sync", res.Absorbed)
		warnAbsorbFailure(warn, res.AbsorbErr)
		return true, nil
	}
	return false, nil
}

func waveCIGate(projectRoot string, warn io.Writer) func(context.Context, gitexec.MainRelation) error {
	return func(ctx context.Context, rel gitexec.MainRelation) error {
		red, failed, err := mainCIRedForSHA(ctx, projectRoot, rel.Remote)
		switch {
		case err != nil:
			fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: main CI status unavailable (%v) — proceeding without false-green or false-red classification\n", err)
		case red:
			return fmt.Errorf("%w at origin/main %.12s: %s", errMainCIRed, rel.Remote, strings.Join(failed, ", "))
		}
		return nil
	}
}

func waveInboxStamps(warn io.Writer) func(context.Context, gitexec.Git) func(mainsync.Result) {
	return func(ctx context.Context, g gitexec.Git) func(mainsync.Result) {
		plan := prepareInboxStampsForFastForward(ctx, g, warn)
		return func(res mainsync.Result) {
			if res.Outcome == mainsync.OutcomeBlocked {
				warnBlockedFastForward(ctx, g, warn, res)
				restoreInboxStampsAfterBlockedFastForward(g.Dir, plan, warn)
				return
			}
			replayInboxStampsAfterFastForward(g.Dir, plan, warn)
		}
	}
}

func warnAbsorbFailure(warn io.Writer, err error) {
	if err != nil {
		fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: %v — the planner keeps the root copy off the lane menu while the claim holds it\n", err)
	}
}

func warnBlockedFastForward(ctx context.Context, g gitexec.Git, warn io.Writer, res mainsync.Result) {
	fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: local tracked changes block the fast-forward (rc=%d: %s) — resolve the dirt (or console-lease it) rather than shipping it\n", res.Code, res.Stderr)
	if blocking := blockingInboxFiles(ctx, g); len(blocking) > 0 {
		fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: %d plane-side inbox file(s) block it: %s — remove the plane copy, then run evolve sync-main, which lands the loop's own stamps (runtime-reference: the console route at a boundary)\n", len(blocking), strings.Join(blocking, ", "))
	}
}

func prepareInboxStampsForFastForward(ctx context.Context, g gitexec.Git, warn io.Writer) inboxstamps.Plan {
	partition, err := inboxstamps.Classify(ctx, g)
	if err != nil {
		fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: reading the tree's changes failed: %v — the loop's inbox stamps stay as they are\n", err)
		return inboxstamps.Plan{}
	}
	if len(partition.Other) > 0 || len(partition.Stamps) == 0 {
		return inboxstamps.Plan{}
	}
	plan, err := inboxstamps.PlanAgainst(ctx, g, partition.Stamps, "origin/main")
	if err == nil {
		err = plan.Prepare(ctx, g)
	}
	if err != nil {
		fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: %v — the loop's inbox stamps stay as they are\n", err)
		warnOnFailedRestore(plan.Restore(g.Dir), warn)
		return inboxstamps.Plan{}
	}
	return plan
}

func replayInboxStampsAfterFastForward(root string, plan inboxstamps.Plan, warn io.Writer) {
	if err := plan.ApplyReplay(root); err != nil {
		fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: %v\n", err)
	}
	warnStamps(warn, "dropped %d plane-side inbox stamp(s) on items origin retired", plan.Retired)
	warnStamps(warn, "origin superseded %d plane-side inbox stamp(s), changing every field they set", plan.Superseded)
	warnStamps(warn, "replayed %d plane-side inbox stamp(s) onto origin's edits, uncommitted until evolve sync-main", plan.Replay)
}

func restoreInboxStampsAfterBlockedFastForward(root string, plan inboxstamps.Plan, warn io.Writer) {
	if warnOnFailedRestore(plan.Restore(root), warn) {
		return
	}
	warnStamps(warn, "restored %d plane-side inbox stamp(s) after the blocked fast-forward", slices.Concat(plan.Retired, plan.Superseded, plan.Replay))
}

func warnOnFailedRestore(err error, warn io.Writer) bool {
	if err != nil {
		fmt.Fprintf(warn, "[loop] WARN: wave-boundary sync: restoring the loop's inbox stamps failed: %v\n", err)
	}
	return err != nil
}

func warnStamps(warn io.Writer, format string, stamps []inboxstamps.Stamp) {
	if len(stamps) > 0 {
		fmt.Fprintf(warn, "[loop] wave-boundary sync: "+format+": %s\n", len(stamps), strings.Join(inboxstamps.Paths(stamps), ", "))
	}
}

func blockingInboxFiles(ctx context.Context, g gitexec.Git) []string {
	local, _, code, err := g.Capture(ctx, "status", "--porcelain", "-z", "--untracked-files=all", "--", inboxstamps.Pathspec)
	if err != nil || code != 0 {
		return nil
	}
	changed, err := inboxstamps.ChangedOnRemote(ctx, g, "origin/main")
	if err != nil {
		return nil
	}
	var blocking []string
	for _, entry := range strings.Split(local, "\x00") {
		if len(entry) > 3 && entry[1] != 'D' && changed[entry[3:]] {
			blocking = append(blocking, entry[3:])
		}
	}
	sort.Strings(blocking)
	return blocking
}
