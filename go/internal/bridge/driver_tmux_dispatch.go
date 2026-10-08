package bridge

import (
	"context"
	"fmt"
	"time"
)

// dispatchTmuxPrompt requires the completion-evidence baseline to be captured
// before any input, and submit verification to observe the pane only after
// the REPL redraws.
func dispatchTmuxPrompt(
	ctx context.Context,
	cfg *Config,
	deps Deps,
	lp tmuxLaunch,
	prep replPreparation,
	human bool,
	phaseName string,
) (dispatchBaseline, pasteOutcome, int, error) {
	deps.sweep.recordPane(ctx, deps, lp.session)
	capture := deps.CaptureBaseline
	if capture == nil {
		capture = captureArtifactBaseline
	}
	dispatchBase := capture(cfg)
	dispatchBase.worktree = captureWorktreeEvidenceBaseline(ctx, cfg, deps)

	if !prep.namedExists && len(cfg.Realization.REPLInput) > 0 {
		seeded := 0
		for index, line := range cfg.Realization.REPLInput {
			if err := deps.Tmux.SendKeys(ctx, lp.session, line, true); err != nil {
				fmt.Fprintf(deps.Stderr, "%s WARN: REPL seed send failed index=%d total=%d detail=%s\n",
					prep.prefix, index+1, len(cfg.Realization.REPLInput), diagnosticField(err.Error()))
				continue
			}
			seeded++
			if observation, ok := modelDispatchFromREPL(line); ok {
				observeModelDispatch(deps, observation)
			}
			deps.Sleep(time.Second)
		}
		fmt.Fprintf(deps.Stderr, "%s seeded %d/%d REPL input line(s)\n", prep.prefix, seeded, len(cfg.Realization.REPLInput))
	}

	if human {
		humanBootPause(deps)
	}
	paste, err := pastePrompt(ctx, deps, prep.prefix, lp.session, prep.resolvedPromptFile, human)
	if err != nil {
		fmt.Fprintf(deps.Stderr, "[bridge] %sphase=%s waited=0s transient=true reason=%q\n",
			artifactTimeoutMarker, phaseName, err.Error())
		return dispatchBase, paste, ExitArtifactTimeout, err
	}
	deps.Sleep(submitVerifySettle)
	fmt.Fprintf(deps.Stderr, "%s prompt delivered\n", prep.prefix)
	return dispatchBase, paste, ExitOK, nil
}
