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

const paneTmuxDetachLine = "unset TMUX TMUX_PANE"

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
		ar.endBoot()
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

	if err := deps.startSession(ctx, lp.session, prep.workingDir); err != nil {
		return nil, ExitBadFlags, fmt.Errorf("%s new-session: %w", prep.prefix, err)
	}

	if err := sessionrecord.Append(sessionrecord.PathIn(cfg.Workspace), sessionrecord.Record{
		Session: lp.session, RunID: cfg.RunID, Cycle: cfg.Cycle,
		Agent: cfg.Agent, PID: os.Getpid(), CreatedAt: deps.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		fmt.Fprintf(deps.Stderr, "%s WARN session registry append failed: %v (session %s will not be registry-reapable)\n", prep.prefix, err, lp.session)
	}

	deps.Sleep(time.Second)
	if err := primePaneShell(ctx, deps, cfg, lp.session, prep.workingDir); err != nil {
		fmt.Fprintf(deps.Stderr, "%s FAIL: %v\n", prep.prefix, err)
		return nil, ExitBadFlags, fmt.Errorf("%s %w", prep.prefix, err)
	}
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
	if err := deps.Tmux.SendKeys(ctx, lp.session, launchCmd, true); err != nil {
		fmt.Fprintf(deps.Stderr, "%s FAIL: CLI launch send failed session=%s detail=%s\n",
			prep.prefix, diagnosticField(lp.session), diagnosticField(err.Error()))
		return nil, ExitBadFlags, fmt.Errorf("%s send CLI launch to session %q: %w", prep.prefix, lp.session, err)
	}
	observeModelDispatch(deps, lp.modelDispatch)
	fmt.Fprintf(deps.Stderr, "%s launching: %s\n", prep.prefix, launchCmd)

	interval := defaultIfZero(lp.bootIntervalS, 1)
	bootDeadlineS := defaultIfZero(deps.BootTimeoutS, tmuxREPLBootTimeoutS)
	const fixedReadinessWaits = 2
	bootWaitMS := int64(fixedReadinessWaits) * 1000
	promptSeen := false
	for elapsed := 0; elapsed < bootDeadlineS; elapsed += interval {
		deps.Sleep(time.Duration(interval) * time.Second)
		bootWaitMS += int64(interval) * 1000
		pane, capErr := deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
		if capErr != nil {
			continue
		}
		if lp.tickDuringBoot {
			repoll, err := ar.bootTick(ctx, lp.session, pane)
			if err != nil {
				fmt.Fprintf(deps.Stderr, "%s FAIL: %v\n", prep.prefix, err)
				return nil, ExitREPLBootTimeout, nil
			}
			if repoll {
				continue
			}
		}
		if !strings.Contains(pane, lp.promptMarker) {
			continue
		}
		if lp.markerOverDeadShell(ctx, deps, prep.prefix) {
			continue
		}
		promptSeen = true
		fmt.Fprintf(deps.Stderr, "%s REPL prompt (%s) detected\n", prep.prefix, lp.promptMarker)
		break
	}
	if !promptSeen {
		fmt.Fprintf(deps.Stderr, "%s FAIL: REPL prompt never appeared after %ds\n", prep.prefix, bootDeadlineS)
		return nil, ExitREPLBootTimeout, nil
	}
	ar.endBoot()
	if deps.OnBoot != nil {
		deps.OnBoot(bootWaitMS)
	}

	releaseOnError = false
	return admitRelease, ExitOK, nil
}

func primePaneShell(ctx context.Context, deps Deps, cfg *Config, session, workingDir string) error {
	_ = deps.Tmux.SendKeys(ctx, session, "cd "+workingDir, true)
	if cfg.ProjectRoot != "" {
		_ = deps.Tmux.SendKeys(ctx, session, "export EVOLVE_PROJECT_ROOT="+shellQuotePOSIX(cfg.ProjectRoot), true)
	}
	if err := deps.Tmux.SendKeys(ctx, session, paneTmuxDetachLine, true); err != nil {
		return fmt.Errorf("detach the pane shell from the run tmux server: %w", err)
	}
	for _, line := range append([]string{dispatchTagLine(cfg.DispatchID)}, exportLines(cfg.Realization.Env)...) {
		_ = deps.Tmux.SendKeys(ctx, session, line, true)
	}
	return nil
}

func (deps Deps) startSession(ctx context.Context, session, workingDir string) error {
	if starter, ok := deps.Tmux.(workdirSessionStarter); ok {
		return starter.NewSessionIn(ctx, session, tmuxPaneWidth, tmuxPaneHeight, workingDir)
	}
	return deps.Tmux.NewSession(ctx, session, tmuxPaneWidth, tmuxPaneHeight)
}

func (lp tmuxLaunch) markerOverDeadShell(ctx context.Context, deps Deps, prefix string) bool {
	if !lp.guardDeadShell {
		return false
	}
	shellCmd, isShell := paneShellProcess(ctx, deps.Tmux, lp.session)
	if isShell {
		fmt.Fprintf(deps.Stderr, "%s marker visible but pane process is a shell (%s) — not ready (dead-shell guard)\n", prefix, shellCmd)
	}
	return isShell
}
