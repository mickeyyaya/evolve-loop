package bridge

import (
	"context"
	"fmt"
	"time"
)

// dispatchTmuxPrompt snapshots pre-existing completion evidence, applies any
// one-time REPL seed, and submits the phase prompt. It owns the ordering
// contract between those operations: the baseline must precede every input,
// and submit verification must observe the pane only after the REPL redraws.
func dispatchTmuxPrompt(
	ctx context.Context,
	cfg *Config,
	deps Deps,
	lp tmuxLaunch,
	prep replPreparation,
	human bool,
	phaseName string,
) (artifactBaseline, int, error) {
	capture := deps.CaptureBaseline
	if capture == nil {
		capture = captureArtifactBaseline
	}
	artifactBase := capture(cfg)

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
	if err := pastePrompt(ctx, deps, lp.session, prep.resolvedPromptFile, human); err != nil {
		fmt.Fprintf(deps.Stderr, "[bridge] %sphase=%s waited=0s transient=true reason=%q\n",
			artifactTimeoutMarker, phaseName, err.Error())
		return artifactBase, ExitArtifactTimeout, err
	}
	deps.Sleep(submitVerifySettle)
	fmt.Fprintf(deps.Stderr, "%s prompt delivered\n", prep.prefix)
	return artifactBase, ExitOK, nil
}
