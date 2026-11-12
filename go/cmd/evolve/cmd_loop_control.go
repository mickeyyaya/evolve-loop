package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotareset"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/sessionreaper"
	"github.com/mickeyyaya/evolve-loop/go/internal/sessionrecord"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

var wireOrchestratorDepsFn = wireOrchestratorDeps

var disableWorkspaceGuardForTest bool

type dispatchPolicy int

const (
	dispatchPolicyVerify dispatchPolicy = iota
	dispatchPolicyOff
	dispatchPolicyStop
)

const defaultCircuitBreakerThreshold = 5

const orphanGCTimeout = sessionreaper.DefaultReapTimeout

func resolveDispatchPolicy(policyVal string, stderr io.Writer) dispatchPolicy {
	if policyVal != "" {
		switch policyVal {
		case "off":
			return dispatchPolicyOff
		case "stop":
			return dispatchPolicyStop
		case "verify":
			return dispatchPolicyVerify
		default:
			fmt.Fprintf(stderr, "[loop] WARN: unknown dispatch policy %q — defaulting to verify\n", policyVal)
			return dispatchPolicyVerify
		}
	}
	return dispatchPolicyVerify
}

func resolveCircuitBreakerThreshold(threshold int) int {
	if threshold <= 0 {
		return defaultCircuitBreakerThreshold
	}
	return threshold
}

func loadDispatchConfig(evolveDir string) policy.DispatchConfig {
	return policy.LoadOrDefaults(filepath.Join(evolveDir, "policy.json"), os.Stderr).DispatchConfig()
}

func loadWorkflowConfig(evolveDir string) policy.WorkflowConfig {
	return policy.LoadOrDefaults(filepath.Join(evolveDir, "policy.json"), os.Stderr).WorkflowConfig()
}

func loadClassifyConfig(evolveDir string) policy.ClassifyPolicy {
	return policy.LoadOrDefaults(filepath.Join(evolveDir, "policy.json"), os.Stderr).ClassifyConfig()
}

func recordAbsorbedFail(cfg loopConfig, ranCycle int, stderr io.Writer) {
	workspace := cycleWorkspace(cfg.ProjectRoot, ranCycle)
	cls := cycleclassify.Classify(workspace)
	_, err := failurelog.Record(
		filepath.Join(cfg.EvolveDir, "state.json"),
		filepath.Join(cfg.EvolveDir, "runs"),
		failurelog.RecordRequest{
			Cycle:          ranCycle,
			Classification: string(cls.Class),
			ReportPath:     filepath.Join(workspace, "orchestrator-report.md"),
			Now:            time.Now().UTC(),
		})
	if err != nil && !errors.Is(err, failurelog.ErrStateMissing) {
		fmt.Fprintf(stderr, "[loop] WARN: could not record absorbed FAIL for cycle %d: %v\n", ranCycle, err)
	}
}

type loopCycleRunner interface {
	RunCycle(context.Context, core.CycleRequest) (core.CycleResult, error)
	RunCycleFromPhase(context.Context, core.CycleRequest, *core.ResumePoint) (core.CycleResult, error)
	SignalSummary() signalcenter.Summary
}

var loopOrchOverride loopCycleRunner

var loopClock = time.Now

func consecutiveFailBreaker(failed bool, streak, max int) (newStreak int, stop bool) {
	if !failed {
		return 0, false
	}
	streak++
	return streak, streak >= max
}

func readLastCycleNumber(ctx context.Context, st core.Storage) (int, error) {
	state, err := st.ReadState(ctx)
	if err != nil {
		return 0, err
	}
	return state.LastCycleNumber, nil
}

func readBatchWindowFloor(ctx context.Context, st core.Storage) (int, error) {
	state, err := st.ReadState(ctx)
	if err != nil {
		return 0, err
	}
	return state.HighestCycleNumber(), nil
}

func unfinishedCycle(cs core.CycleState, lastCycleNumber int) bool {
	return cs.CycleID != 0 && cs.CycleID > lastCycleNumber
}

func reconcileStaleCycleState(ctx context.Context, st core.Storage, projectRoot string, now time.Time, stderr io.Writer) {
	if st == nil {
		return
	}
	cs, err := st.ReadCycleState(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: state-coherence: canonical cycle-state unreadable, left untouched: %v\n", err)
		return
	}
	if !claimsLivePhase(cs) {
		return
	}
	last, err := readLastCycleNumber(ctx, st)
	if err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: state-coherence: cycle %d phase=%s not checked, state.json unreadable: %v\n", cs.CycleID, cs.Phase, err)
		return
	}
	if unfinishedCycle(cs, last) {
		return
	}
	owner, dead, err := cycleOwnerDead(cs, projectRoot, now)
	if err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: state-coherence: cycle %d phase=%s not reconciled, owner liveness unknown: %v\n", cs.CycleID, cs.Phase, err)
		return
	}
	if !dead {
		return
	}
	stalePhase := cs.Phase
	cs.Phase = string(core.PhaseAborted)
	cs.ActiveAgent = ""
	if err := st.WriteCycleState(ctx, cs); err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: state-coherence: stale cycle %d phase=%s (%s) — reconcile write failed: %v\n", cs.CycleID, stalePhase, owner, err)
		return
	}
	fmt.Fprintf(stderr, "[loop] WARN: state-coherence: canonical cycle-state claimed live phase=%s for dead cycle %d (%s; last completed cycle %d) — reconciled to phase=aborted\n",
		stalePhase, cs.CycleID, owner, last)
}

func claimsLivePhase(cs core.CycleState) bool {
	if cs.Phase == "" || core.IsTerminalPhase(cs.Phase) {
		return false
	}
	return cs.CycleID != 0 && !slices.Contains(cs.CompletedPhases, cs.Phase)
}

func cycleOwnerDead(cs core.CycleState, projectRoot string, now time.Time) (owner string, dead bool, err error) {
	runDir := cs.WorkspacePath
	if runDir == "" {
		runDir = cycleWorkspace(projectRoot, cs.CycleID)
	} else if !filepath.IsAbs(runDir) {
		runDir = filepath.Join(projectRoot, runDir)
	}
	lease, ok, err := runlease.Read(runDir)
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "no run lease", true, nil
	}
	if runlease.OwnerLive(lease, now, runlease.DefaultTTL, runlease.PIDAlive) {
		return "", false, nil
	}
	if !runlease.Fresh(lease, now, runlease.DefaultTTL) {
		return "run lease heartbeat stale", true, nil
	}
	return fmt.Sprintf("run lease owner pid %d not running", lease.OwnerPID), true, nil
}

func cycleWorkspace(projectRoot string, cycle int) string {
	return filepath.Join(projectRoot, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
}

func reapCycleSessions(projectRoot string, cycle int, stderr io.Writer) {
	if cycle <= 0 {
		return
	}
	path := sessionrecord.PathIn(cycleWorkspace(projectRoot, cycle))
	rep := swarm.ReapRunSessions(context.Background(), path, swarm.ExecTmuxKill)
	if rep.Killed > 0 || rep.Errors > 0 || rep.Skipped > 0 {
		fmt.Fprintf(stderr, "[loop] session registry reap cycle=%d: killed=%d skipped=%d errors=%d\n",
			cycle, rep.Killed, rep.Skipped, rep.Errors)
	}
	gcOrphanSessions(fmt.Sprintf("cycle=%d", cycle), stderr)
}

func gcOrphanSessions(label string, stderr io.Writer) {
	ctx, cancel := context.WithTimeout(context.Background(), orphanGCTimeout)
	defer cancel()
	reapers := gcReapers{sessions: swarm.ExecReapOrphans, sockets: swarm.ExecReapOrphanSockets}
	if gcInjectedReapers != nil {
		reapers = *gcInjectedReapers
	}
	rep := reapers.sessions(ctx)
	if len(rep.Killed) > 0 || len(rep.Errors) > 0 {
		fmt.Fprintf(stderr, "[loop] orphan-session GC (%s): killed=%d skipped(live=%d foreign=%d no-pid=%d) errors=%d\n",
			label, len(rep.Killed), rep.SkippedLive, rep.SkippedForeign, rep.SkippedUnparseable, len(rep.Errors))
	}
	srep := reapers.sockets(ctx)
	if len(srep.Killed) > 0 || len(srep.Errors) > 0 {
		fmt.Fprintf(stderr, "[loop] orphan-socket GC (%s): killed=%d dead per-run socket(s) skipped-live=%d errors=%d\n",
			label, len(srep.Killed), srep.SkippedLive, len(srep.Errors))
	}
}

func updateBreaker(prev, streak, ranCycle, threshold int) (newPrev, newStreak int, tripped bool) {
	if ranCycle == prev {
		streak++
	} else {
		streak = 1
		prev = ranCycle
	}
	return prev, streak, streak >= threshold
}

type quotaPause struct {
	Cycle          int
	WakeAt         string
	Source         string
	Attempts       int
	MaxAttempts    int
	OperatorAction string
	Wall           core.Wall
	Phase          string
	Workspace      string
	WallErr        string
}

const (
	minResumeDelay = time.Minute
	maxResumeDelay = time.Hour
)

func (qp quotaPause) resumeDelay(now time.Time) (time.Duration, bool) {
	if qp.MaxAttempts <= qp.Attempts {
		return 0, false
	}
	wake, err := quotareset.ParseISO(qp.WakeAt)
	if err != nil {
		return 0, false
	}
	return min(maxResumeDelay, max(minResumeDelay, wake.Sub(now)+minResumeDelay)), true
}

func quotaPauseOf(cycle int, cp checkpoint.Checkpoint) quotaPause {
	return quotaPause{
		Cycle:          cycle,
		WakeAt:         cp.QuotaResetAt,
		Source:         cmp.Or(cp.QuotaResetSource, "unknown"),
		Attempts:       cp.AutoResumeAttempts,
		MaxAttempts:    cp.AutoResumeMaxAttempts,
		OperatorAction: cp.OperatorAction,
		Wall:           core.Wall{Ends: cp.QuotaWall},
	}
}

func quotaPauseLine(qp quotaPause, now time.Time) string {
	head := fmt.Sprintf("QUOTA-PAUSE: cycle=%d wake-at=%s source=%s attempts=%d/%d", qp.Cycle, qp.WakeAt, qp.Source, qp.Attempts, qp.MaxAttempts)
	if delay, ok := qp.resumeDelay(now); ok {
		return fmt.Sprintf("%s resume-in=%ds", head, int(delay.Seconds()))
	}
	return head + " auto-resume=off: " + cmp.Or(qp.OperatorAction, "the auto-resume budget is spent or the wake time is unreadable; run `evolve loop --resume` by hand")
}

func loadActiveQuotaLikelyCheckpoint(evolveDir string) (blob, active map[string]any, ok bool) {
	return loadActiveCheckpoint(evolveDir, string(checkpoint.ReasonQuotaLikely))
}

func loadActiveCheckpoint(evolveDir, want string) (blob, checkpoint map[string]any, ok bool) {
	path := core.ResolveCycleStatePath(evolveDir)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, false
	}
	if err := json.Unmarshal(raw, &blob); err != nil {
		return nil, nil, false
	}
	cp, ok := blob["checkpoint"].(map[string]any)
	if !ok {
		return nil, nil, false
	}
	enabled, _ := cp["enabled"].(bool)
	if !enabled {
		return nil, nil, false
	}
	reason, _ := cp["reason"].(string)
	if reason != want {
		return nil, nil, false
	}
	return blob, cp, true
}

func quotaPauseFromCheckpoint(blob, cp map[string]any) quotaPause {
	qp := quotaPause{MaxAttempts: 3}
	if v, ok := blob["cycle_id"].(float64); ok {
		qp.Cycle = int(v)
	} else if v, ok := blob["cycle"].(float64); ok {
		qp.Cycle = int(v)
	}
	if v, ok := cp["quotaResetAt"].(string); ok {
		qp.WakeAt = v
	}
	if v, ok := cp["quotaResetSource"].(string); ok && v != "" {
		qp.Source = v
	} else {
		qp.Source = "unknown"
	}
	if v, ok := cp["autoResumeAttempts"].(float64); ok {
		qp.Attempts = int(v)
	}
	if v, ok := cp["autoResumeMaxAttempts"].(float64); ok {
		qp.MaxAttempts = int(v)
	}
	qp.Phase, _ = cp["resumeFromPhase"].(string)
	qp.Workspace, _ = blob["workspace_path"].(string)
	if v, ok := cp["operatorAction"].(string); ok {
		qp.OperatorAction = v
	}
	raw, err := json.Marshal(cp["quotaWall"])
	if err == nil {
		err = json.Unmarshal(raw, &qp.Wall.Ends)
	}
	if err != nil {
		qp.Wall, qp.WallErr = core.Wall{}, err.Error()
	}
	return qp
}

func detectQuotaPause(evolveDir string) (quotaPause, bool) {
	blob, cp, ok := loadActiveQuotaLikelyCheckpoint(evolveDir)
	if !ok {
		return quotaPause{}, false
	}
	return quotaPauseFromCheckpoint(blob, cp), true
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func (qp quotaPause) clauses() string {
	if qp.WallErr != "" {
		return "the recorded cause could not be read: " + qp.WallErr
	}
	return qp.Wall.Clauses()
}

func (qp quotaPause) timed(now time.Time) bool {
	_, ok := qp.resumeDelay(now)
	return ok
}

func (qp quotaPause) stopReason(now time.Time) string {
	return qp.Wall.StopReasonFor(qp.timed(now))
}

func (qp quotaPause) wallPauseLine(next string, now time.Time) string {
	delay, ok := qp.resumeDelay(now)
	if !ok {
		return fmt.Sprintf("WALL-PAUSE: cycle=%d %s. Next: %s", qp.Cycle, qp.clauses(), next)
	}
	return fmt.Sprintf("WALL-PAUSE: cycle=%d wake-at=%s source=%s attempts=%d/%d resume-in=%ds %s. Next: %s",
		qp.Cycle, qp.WakeAt, qp.Source, qp.Attempts, qp.MaxAttempts, int(delay.Seconds()), qp.clauses(), next)
}

func (qp quotaPause) report(now time.Time) []string {
	next := core.WallNextStep(qp.Wall, qp.Workspace)
	switch {
	case qp.Wall.HasLogin():
		lines := []string{fmt.Sprintf("LOGIN-REQUIRED: cycle=%d %s. Next: %s", qp.Cycle, qp.Wall.Clauses(), next)}
		if qp.Wall.HasQuota() {
			lines = append(lines, quotaPauseLine(qp, now))
		}
		return lines
	case qp.Wall.HasQuota(), len(qp.Wall.Ends) == 0 && qp.WakeAt != "" && qp.WallErr == "":
		return []string{fmt.Sprintf("%s (%s)", quotaPauseLine(qp, now), qp.clauses()), "[loop]   next: " + next}
	}
	return []string{qp.wallPauseLine(next, now)}
}

func (lr *loopResult) emitQuotaPause(cfg loopConfig, result core.CycleResult, stdout, stderr io.Writer) {
	now := time.Now()
	qp, onDisk := detectQuotaPause(cfg.EvolveDir)
	switch {
	case len(result.QuotaWall) > 0 && result.WallWriteErr != "":
		qp = quotaPause{Cycle: result.Cycle, Wall: core.Wall{Ends: result.QuotaWall}, Phase: result.WallPhase}
		fmt.Fprintf(stderr, "WALL-PAUSE: cycle=%d the checkpoint write failed: %s; resume re-runs from the last boundary\n", result.Cycle, result.WallWriteErr)
	case len(result.QuotaWall) > 0:
		qp.Wall, qp.WallErr, qp.Phase = core.Wall{Ends: result.QuotaWall}, "", result.WallPhase
		if qp.Cycle == 0 {
			qp.Cycle = result.Cycle
		}
	case !onDisk:
		qp = quotaPause{Cycle: result.Cycle}
		fmt.Fprintf(stderr, "WALL-PAUSE: cycle=%d no checkpoint block is on disk, so no cause was recorded; resume re-runs from the last boundary: evolve loop --resume\n", result.Cycle)
		lr.StopReason = qp.stopReason(now)
		lr.emit(stdout)
		return
	}
	for _, line := range qp.report(now) {
		fmt.Fprintln(stderr, line)
	}
	lr.StopReason = qp.stopReason(now)
	lr.emit(stdout)
}
