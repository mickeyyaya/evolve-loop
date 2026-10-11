package bridge

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/launchoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
)

func reportBootLoss(deps Deps, cause launchoutcome.BootLossCause, reason string) {
	fmt.Fprintf(deps.Stderr, "[bridge] boot-loss: cause=%s reason=%q\n", cause, reason)
	if deps.onBootLoss != nil {
		deps.onBootLoss(cause)
	}
}

func reportSessionGoneAtBoot(ctx context.Context, deps Deps, session string) {
	if !deps.Tmux.HasSession(ctx, session) {
		reportBootLoss(deps, launchoutcome.BootLossPaneLost, "has-session found no session "+session+" at the boot deadline")
	}
}

func reportDeadShellAfterPaste(deps Deps, shellCmd string) {
	reportBootLoss(deps, launchoutcome.BootLossDeadShell, "the prompt spilled into the shell "+shellCmd+" after the paste")
}

func markBootLoss(run launchRun) func(*llmcalls.Record) {
	return func(rec *llmcalls.Record) {
		rec.CauseCode = launchoutcome.WithBootLoss(rec.CauseCode, run.code, run.bootLoss)
	}
}
