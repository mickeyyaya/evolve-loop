package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliupdate"
	"github.com/mickeyyaya/evolve-loop/go/internal/practicewatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const cliUpdateSmokeHalt = "cli_update_smoke_halt"

var cliUpdateLogPrefix = map[cliupdate.Status]string{
	cliupdate.StatusUpdateFailed:   "[loop] WARN: cli-update: ",
	cliupdate.StatusSkipped:        "[loop] WARN: cli-update: ",
	cliupdate.StatusBootTimeout:    "[loop] WARN: cli-update: ",
	cliupdate.StatusQuotaExhausted: "[loop] WARN: cli-update: ",
	cliupdate.StatusSmokeFailed:    "[loop] HALT: cli-update: ",
}

func boundaryCLIUpdate(ctx context.Context, cfg loopConfig, stderr io.Writer) []cliupdate.Result {
	rep := updateCLIs(ctx, cfg.ProjectRoot, cfg.EvolveDir, false, stderr)
	for _, res := range rep.Results {
		prefix, loud := cliUpdateLogPrefix[res.Status]
		if !loud {
			prefix = "[loop] cli-update: "
		}
		fmt.Fprintln(stderr, prefix+res.Line())
	}
	boundaryPracticeWatch(ctx, cfg, stderr)
	return rep.SmokeFailed()
}

func haltOnCLISmokeFailure(signals *signalcenter.Center, origin string, failed []cliupdate.Result, lr *loopResult) {
	lines := make([]string, len(failed))
	families := make([]string, len(failed))
	versions := make([]string, len(failed))
	for i, res := range failed {
		lines[i], families[i], versions[i] = res.Line(), res.Family, res.Family+"="+res.NewVersion
	}
	lr.StopReason = cliUpdateSmokeHalt
	lr.CLIUpdateHalt = failed
	emitLoopHalt(signals, 0, origin, CodeLoopHalt,
		"boundary CLI update: "+strings.Join(lines, "; ")+"; the next wave is not launched: run `evolve doctor live <family>-tmux`, then reinstall or roll back that CLI",
		map[string]string{"stop_reason": cliUpdateSmokeHalt, "families": strings.Join(families, ","), "versions": strings.Join(versions, ",")})
}

func emitCLISmokeBenched(signals *signalcenter.Center, origin string, res cliupdate.Result) {
	signals.Emit(signalcenter.Event{
		Module:   signalcenter.ModuleLoop,
		Origin:   origin,
		Kind:     signalcenter.KindLoopWarning,
		Severity: signalcenter.SeverityIncident,
		Reason:   fmt.Sprintf("boundary CLI update smoke failed for %s (%s): %s; family benched, loop continues on fallback", res.Family, res.NewVersion, res.Detail),
		Fields:   map[string]string{"family": res.Family, "version": res.NewVersion, "stage": "smoke", "detail": res.Detail},
	})
}

func chainHasRunnable(chain []string, blocked map[string]bool) bool {
	for _, fam := range chain {
		if !blocked[fam] {
			return true
		}
	}
	return false
}

func allChainsRunnable(chains [][]string, blocked map[string]bool) bool {
	for _, chain := range chains {
		if !chainHasRunnable(chain, blocked) {
			return false
		}
	}
	return true
}

func smokeBypassable(deps orchDeps, projectRoot string, failed []cliupdate.Result) bool {
	if deps.Router == nil {
		return false
	}
	p := deps.Router.Policy()
	chains := cliroute.FamilyChains(p.CLIRouting)
	for fam := range p.Floors() {
		chains = append(chains, []string{fam})
	}
	blocked := benchedFamilies(projectRoot)
	for _, res := range failed {
		blocked[res.Family] = true
	}
	return allChainsRunnable(chains, blocked)
}

func handleCLISmokeFailures(cfg loopConfig, deps orchDeps, origin string, failed []cliupdate.Result, lr *loopResult, stdout, stderr io.Writer) bool {
	if !smokeBypassable(deps, cfg.ProjectRoot, failed) {
		haltOnCLISmokeFailure(deps.Signals, origin, failed, lr)
		lr.emitFatal(stdout, stderr, cfg, 0)
		return true
	}
	store := clihealth.NewStore(cfg.ProjectRoot, nil)
	for _, res := range failed {
		_, _ = store.BenchWall(res.Family, clihealth.SmokeFailedPattern, res.Detail)
		emitCLISmokeBenched(deps.Signals, origin, res)
	}
	return false
}

func bootCLIUpdateHalts(ctx context.Context, cfg loopConfig, deps orchDeps, lr *loopResult, stdout, stderr io.Writer) bool {
	if cfg.HandedOff {
		return false
	}
	failed := boundaryCLIUpdate(ctx, cfg, stderr)
	if len(failed) == 0 || ctx.Err() != nil {
		return false
	}
	return handleCLISmokeFailures(cfg, deps, "prepareFreshBatch", failed, lr, stdout, stderr)
}

func (b *loopBatchCoordinator) updateCLIsAtBoundary(iteration int) (batchDecision, bool) {
	if iteration <= b.cfg.ResumeWaves {
		return batchDecision{}, false
	}
	failed := boundaryCLIUpdate(b.ctx, b.cfg, b.stderr)
	if b.ctx.Err() != nil {
		return b.interruptReturn(iteration, "during the boundary CLI update "), true
	}
	if len(failed) == 0 {
		return batchDecision{}, false
	}
	if handleCLISmokeFailures(b.cfg, b.deps, "loopBatchCoordinator.updateCLIsAtBoundary", failed, b.result, b.stdout, b.stderr) {
		return batchDecision{flow: batchReturn, exitCode: 2}, true
	}
	return batchDecision{}, false
}

var practiceWatchAtBoundary = watchPracticeAtBoundary

var practiceBoundaryBudget = 60 * time.Second

func watchPracticeAtBoundary(ctx context.Context, projectRoot string, stderr io.Writer) (practicewatch.Report, error) {
	return runPracticeWatch(ctx, projectRoot, false, stderr)
}

func boundaryPracticeWatch(ctx context.Context, cfg loopConfig, stderr io.Writer) {
	ctx, cancel := context.WithTimeout(ctx, practiceBoundaryBudget)
	defer cancel()
	rep, err := practiceWatchAtBoundary(ctx, cfg.ProjectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: practice-watch: %v; the boundary continues\n", err)
		return
	}
	for _, res := range rep.Results {
		switch res.Status {
		case practicewatch.StatusOff, practicewatch.StatusUnchanged:
		case practicewatch.StatusError, practicewatch.StatusSkipped:
			fmt.Fprintf(stderr, "[loop] WARN: practice-watch: %s %s: %s\n", res.ID, res.Status, res.Detail)
		default:
			fmt.Fprintf(stderr, "[loop] practice-watch: %s %s %s\n", res.ID, res.Status, res.Item)
		}
	}
}
