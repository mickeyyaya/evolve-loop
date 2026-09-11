package bridge

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliadmit"
	"github.com/mickeyyaya/evolve-loop/go/internal/sessionrecord"
)

// bootTmuxREPL creates and boots a new session. Existing named sessions need no
// boot work. The returned release keeps provider admission for the full launch.
func bootTmuxREPL(
	ctx context.Context,
	cfg *Config,
	deps Deps,
	lp tmuxLaunch,
	prep replPreparation,
	ar *autoResponder,
) (release func(), exitCode int, err error) {
	if prep.namedExists {
		return func() {}, ExitOK, nil
	}

	admitMax := envInt(deps, "EVOLVE_CLI_MAX_CONCURRENT_"+strings.ToUpper(lp.name), 0)
	admitRelease, admitErr := cliadmit.Acquire(ctx, lp.name, admitMax, cliadmit.DefaultTTL)
	if admitErr != nil {
		fmt.Fprintf(deps.Stderr, "%s WARN cliadmit: %v (proceeding uncapped)\n", prep.prefix, admitErr)
	}
	releaseOnError := true
	defer func() {
		if releaseOnError {
			admitRelease()
		}
	}()

	if starter, ok := deps.Tmux.(workdirSessionStarter); ok {
		if err := starter.NewSessionIn(ctx, lp.session, tmuxPaneWidth, tmuxPaneHeight, prep.workingDir); err != nil {
			return nil, ExitBadFlags, fmt.Errorf("%s new-session: %w", prep.prefix, err)
		}
	} else if err := deps.Tmux.NewSession(ctx, lp.session, tmuxPaneWidth, tmuxPaneHeight); err != nil {
		return nil, ExitBadFlags, fmt.Errorf("%s new-session: %w", prep.prefix, err)
	}

	if err := sessionrecord.Append(sessionrecord.PathIn(cfg.Workspace), sessionrecord.Record{
		Session: lp.session, RunID: cfg.RunID, Cycle: cfg.Cycle,
		Agent: cfg.Agent, PID: os.Getpid(), CreatedAt: deps.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		fmt.Fprintf(deps.Stderr, "%s WARN session registry append failed: %v (session %s will not be registry-reapable)\n", prep.prefix, err, lp.session)
	}

	deps.Sleep(time.Second)
	_ = deps.Tmux.SendKeys(ctx, lp.session, "cd "+prep.workingDir, true)
	deps.Sleep(time.Second)
	launchCmd := lp.launchCmd
	if len(cfg.ExtraFlags) > 0 {
		launchCmd += " " + strings.Join(cfg.ExtraFlags, " ")
	}
	terminalPath, err := sandboxTerminalPath(ctx, deps, cfg, lp.session)
	if err != nil {
		fmt.Fprintf(deps.Stderr, "%s safety gate: terminal lookup failed: %v\n", prep.prefix, err)
		return nil, ExitSafetyGate, nil
	}
	if prefix, ok := sandboxPrefixForLaunch(deps, cfg, terminalPath); ok {
		launchCmd = joinPrefixForTmux(prefix) + " " + launchCmd
		fmt.Fprintf(deps.Stderr, "%s sandbox prefix applied (%d argv elements)\n", prep.prefix, len(prefix))
	} else if sandboxRequiredButUnavailable(deps, cfg, false) {
		fmt.Fprintf(deps.Stderr, "%s safety gate: activated Build explanation contract requires OS sandbox confinement\n", prep.prefix)
		return nil, ExitSafetyGate, nil
	}
	_ = deps.Tmux.SendKeys(ctx, lp.session, launchCmd, true)
	fmt.Fprintf(deps.Stderr, "%s launching: %s\n", prep.prefix, launchCmd)

	interval := lp.bootIntervalS
	if interval <= 0 {
		interval = 1
	}
	bootDeadlineS := defaultIfZero(deps.BootTimeoutS, tmuxREPLBootTimeoutS)
	const fixedReadinessWaits = 2
	bootWaitMS := int64(fixedReadinessWaits) * 1000
	promptSeen := false
	for elapsed := 0; elapsed < bootDeadlineS; elapsed += interval {
		deps.Sleep(time.Duration(interval) * time.Second)
		bootWaitMS += int64(interval) * 1000
		pane, _ := deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
		if lp.tickDuringBoot {
			// A once-only dialog can contain the normal prompt marker. Re-poll
			// after dismissal so prompt delivery cannot land in the stale dialog.
			ar.tick(ctx, lp.session)
			if ar.firedOnceThisTick {
				continue
			}
		}
		if !strings.Contains(pane, lp.promptMarker) {
			continue
		}
		if lp.bootMenuSkip != "" && tmuxPaneLooksLikeUpdateMenu(pane) {
			_ = deps.Tmux.SendKeys(ctx, lp.session, lp.bootMenuSkip, true)
			fmt.Fprintf(deps.Stderr, "%s boot interstitial dismissed before prompt delivery\n", prep.prefix)
			continue
		}
		if lp.guardDeadShell {
			if shellCmd, isShell := paneShellProcess(ctx, deps.Tmux, lp.session); isShell {
				fmt.Fprintf(deps.Stderr, "%s marker visible but pane process is a shell (%s) — not ready (dead-shell guard)\n", prep.prefix, shellCmd)
				continue
			}
		}
		promptSeen = true
		fmt.Fprintf(deps.Stderr, "%s REPL prompt (%s) detected\n", prep.prefix, lp.promptMarker)
		break
	}
	if !promptSeen {
		fmt.Fprintf(deps.Stderr, "%s FAIL: REPL prompt never appeared after %ds\n", prep.prefix, bootDeadlineS)
		return nil, ExitREPLBootTimeout, nil
	}
	if deps.OnBoot != nil {
		deps.OnBoot(bootWaitMS)
	}

	releaseOnError = false
	return admitRelease, ExitOK, nil
}
