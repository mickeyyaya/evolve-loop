package bridge

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

var errWorktreeRequired = errors.New("fleet mode: explicit worktree required (refusing process-cwd fallback)")

const (
	tmuxPromptMarkerDefault  = "❯"
	tmuxREPLBootTimeoutS     = 60
	tmuxArtifactTimeoutS     = 300
	transientRedispatchDelay = 15 * time.Second
	tmuxPaneWidth            = 220
	tmuxPaneHeight           = 80
	tmuxArtifactScrollback   = 10000

	maxInjectDefer        = 10
	injectInterruptSettle = 500 * time.Millisecond
)

type tmuxKey struct {
	keys   string
	enter  bool
	pauseS int
}

type tmuxLaunch struct {
	name            string
	session         string
	named           bool
	launchCmd       string
	modelDispatch   modelDispatch
	promptMarker    string
	inputLineMarker string
	bootScrollback  int
	bootIntervalS   int
	tickDuringBoot  bool
	exitSeq         []tmuxKey
	bootOnly        bool
	guardDeadShell  bool
	modelCheck      launchModelCheck
}

func launchCmdLine(binary string, flags []string) string {
	if len(flags) == 0 {
		return binary
	}
	quoted := make([]string, len(flags))
	for i, f := range flags {
		quoted[i] = shellQuotePOSIX(f)
	}
	return binary + " " + strings.Join(quoted, " ")
}

func runTmuxREPL(ctx context.Context, cfg *Config, deps Deps, lp tmuxLaunch) (int, error) {
	prep, code, err := prepareTmuxREPL(ctx, cfg, deps, lp)
	if code != ExitOK || err != nil {
		return code, err
	}
	if prep.namedExists {
		observeModelDispatch(deps, modelDispatch{source: modelDispatchResumed})
	}
	pfx := prep.prefix
	resolvedPrompt := prep.resolvedPrompt
	defer tmuxCleanup(ctx, deps, lp.name, lp.session, paneCapture{shared: prep.scrollbackFile, dispatch: prep.dispatchPaneFile, lines: prep.artifactScrollback}, lp.named)

	human := cfg.HumanInput
	ar := newLaunchAutoResponder(cfg.Workspace, deps, lp, human)
	ar.injectedPrompt = resolvedPrompt

	phaseName := orDefault(cfg.Agent, lp.name)
	irec := interaction.NewRecorder(cfg.Workspace)
	ar.rec, ar.phase, ar.cycle = irec, phaseName, cfg.Cycle
	ar.broker = interaction.NewKernelAnswerer(interaction.KernelFacts{
		ArtifactPath: cfg.Artifact,
		Workspace:    cfg.Workspace,
		Worktree:     cfg.Worktree,
		Cycle:        strconv.Itoa(cfg.Cycle),
	})
	ar.brokerStage = recoveryStageFromEnv(deps)
	ar.prompts = append(ar.prompts, loadPromotedPrompts(cfg.ProjectRoot)...)
	ar.shadowRules = loadShadowObservers(cfg.ProjectRoot)
	defer ar.flushPending()

	admitRelease, code, err := bootTmuxREPL(ctx, cfg, deps, lp, prep, ar)
	if code != ExitOK || err != nil {
		return code, err
	}
	defer admitRelease()

	if code := lp.verifyBootedModel(ctx, bootedModelRun{cfg: cfg, deps: deps, responder: ar}); code != ExitOK {
		return code, nil
	}
	if lp.bootOnly {
		return lp.endBootSmoke(ctx, deps, pfx), nil
	}

	dispatchBase, paste, code, err := dispatchTmuxPrompt(ctx, cfg, deps, lp, prep, human, phaseName)
	if code != ExitOK || err != nil {
		return code, err
	}

	cursor := newReplInboxCursor(cfg)
	channel := openReplLiveChannel(cfg, deps, lp)
	defer channel.close()
	waitResult, code := (replWaiter{
		ctx: ctx, cfg: cfg, deps: deps, launch: lp,
		prefix: pfx, phaseName: phaseName, resolvedPrompt: resolvedPrompt,
		dispatchBase: dispatchBase, paste: paste, responder: ar, recorder: irec, cursor: cursor, channel: channel,
	}).wait()
	if code != ExitOK {
		return code, nil
	}

	raw, capErr := deps.Tmux.CapturePane(ctx, lp.session, prep.artifactScrollback)
	if raw == "" && capErr != nil {
		raw = waitResult.lastGoodPane
	}
	waitResult.recordTokens(raw)
	_ = os.WriteFile(cfg.StderrLog, []byte(raw+"\n"), 0o644)
	_ = os.WriteFile(cfg.StdoutLog, []byte(stripANSI(raw)+"\n"), 0o644)
	writeTokenUsage(cfg.Workspace, waitResult.peakTokens)
	fmt.Fprintf(deps.Stderr, "%s scrollback captured\n", pfx)

	switch {
	case lp.named:
		fmt.Fprintf(deps.Stderr, "%s RESUME-PRESERVE: skipping exit; REPL stays running for next launch\n", pfx)
	case ctx.Err() != nil:
		fmt.Fprintf(deps.Stderr, "%s exit sequence skipped (ctx done) — session kill handles teardown\n", pfx)
	default:
		lp.sendExitSeq(ctx, deps)
	}
	contract := completionContractName(cfg.Completion)
	fmt.Fprintf(deps.Stderr, "%s DONE: %s completion verdict = SUCCESS\n", pfx, contract)
	return 0, nil
}

func (lp tmuxLaunch) endBootSmoke(ctx context.Context, deps Deps, pfx string) int {
	if !lp.named {
		lp.sendExitSeq(ctx, deps)
	}
	fmt.Fprintf(deps.Stderr, "%s BOOT-SMOKE: REPL booted; exiting without prompt\n", pfx)
	return ExitOK
}

func (lp tmuxLaunch) sendExitSeq(ctx context.Context, deps Deps) {
	for _, k := range lp.exitSeq {
		_ = deps.Tmux.SendKeys(ctx, lp.session, k.keys, k.enter)
		if k.pauseS > 0 {
			deps.Sleep(time.Duration(k.pauseS) * time.Second)
		}
	}
}
