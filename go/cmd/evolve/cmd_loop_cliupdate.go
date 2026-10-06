package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliupdate"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const cliUpdateSmokeHalt = "cli_update_smoke_halt"

var cliUpdateLogPrefix = map[cliupdate.Status]string{
	cliupdate.StatusUpdateFailed: "[loop] WARN: cli-update: ",
	cliupdate.StatusSkipped:      "[loop] WARN: cli-update: ",
	cliupdate.StatusSmokeFailed:  "[loop] HALT: cli-update: ",
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

func bootCLIUpdateHalts(ctx context.Context, cfg loopConfig, deps orchDeps, lr *loopResult, stdout, stderr io.Writer) bool {
	if cfg.HandedOff {
		return false
	}
	failed := boundaryCLIUpdate(ctx, cfg, stderr)
	if len(failed) == 0 || ctx.Err() != nil {
		return false
	}
	haltOnCLISmokeFailure(deps.Signals, "prepareFreshBatch", failed, lr)
	lr.emitFatal(stdout, stderr, cfg, 0)
	return true
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
	haltOnCLISmokeFailure(b.deps.Signals, "loopBatchCoordinator.updateCLIsAtBoundary", failed, b.result)
	b.result.emitFatal(b.stdout, b.stderr, b.cfg, 0)
	return batchDecision{flow: batchReturn, exitCode: 2}, true
}
