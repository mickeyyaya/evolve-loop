package bridge

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/recipe"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/sessionrecord"
)

const (
	recipeBootScrollback = 200
	recipeSessionPrefix  = "evolve-recipe-"
)

type sleepClock struct{ sleep func(time.Duration) }

func (c sleepClock) Sleep(d time.Duration) { c.sleep(d) }

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

type recipeSessionDriver struct {
	cfg        *Config
	deps       Deps
	session    string
	launchCmd  string
	workingDir string
	marker     string
	scrollback int
	ar         *autoResponder
}

func (d *recipeSessionDriver) attachExisting(ctx context.Context) (bool, error) {
	if !d.deps.Tmux.HasSession(ctx, d.session) {
		return false, nil
	}
	if err := d.deps.checkReusedSessionEnv(ctx, d.cfg, d.session); err != nil {
		return false, err
	}
	fmt.Fprintf(d.deps.Stderr, "[recipe] attaching to existing session %s\n", d.session)
	d.ar.endBoot()
	return true, nil
}

func (d *recipeSessionDriver) EnsureSession(ctx context.Context) error {
	if attached, err := d.attachExisting(ctx); attached || err != nil {
		return err
	}
	if err := d.deps.startSession(ctx, d.session, d.workingDir, d.deps.paneEnv(d.cfg)); err != nil {
		return fmt.Errorf("new-session: %w", err)
	}
	if err := sessionrecord.Append(sessionrecord.PathIn(d.cfg.Workspace), sessionrecord.Record{
		Session: d.session, RunID: d.cfg.RunID, Cycle: d.cfg.Cycle,
		Agent: d.cfg.Agent, PID: os.Getpid(),
	}); err != nil {
		fmt.Fprintf(d.deps.Stderr, "[recipe] WARN session registry append failed: %v (session %s will not be registry-reapable)\n", err, d.session)
	}
	d.deps.Sleep(time.Second)
	_ = d.deps.Tmux.SendKeys(ctx, d.session, "cd "+shellSingleQuote(d.workingDir), true)
	_ = d.deps.Tmux.SendKeys(ctx, d.session, paneTmuxDetachLine, true)
	for _, line := range append(exportLines(d.cfg.Realization.Env), exportLines(d.deps.cellMarkers(d.cfg))...) {
		_ = d.deps.Tmux.SendKeys(ctx, d.session, line, true)
	}
	d.deps.Sleep(time.Second)
	_ = d.deps.Tmux.SendKeys(ctx, d.session, d.launchCmd, true)
	fmt.Fprintf(d.deps.Stderr, "[recipe] launching: %s\n", d.launchCmd)
	bootDeadlineS := defaultIfZero(d.deps.BootTimeoutS, tmuxREPLBootTimeoutS)
	for elapsed := 0; elapsed < bootDeadlineS; elapsed++ {
		if ctx.Err() != nil {
			return fmt.Errorf("REPL boot abandoned: %w", ctx.Err())
		}
		d.deps.Sleep(time.Second)
		pane, capErr := d.deps.Tmux.CapturePane(ctx, d.session, d.scrollback)
		if capErr != nil {
			continue
		}
		if d.ar != nil {
			repoll, err := d.ar.bootTick(ctx, d.session, pane)
			if err != nil {
				return fmt.Errorf("REPL boot abandoned: %w", err)
			}
			if repoll {
				continue
			}
		}
		if d.marker != "" && strings.Contains(pane, d.marker) {
			fmt.Fprintf(d.deps.Stderr, "[recipe] REPL prompt (%s) detected\n", d.marker)
			d.ar.endBoot()
			return nil
		}
	}
	return fmt.Errorf("REPL prompt %q never appeared after %ds", d.marker, bootDeadlineS)
}

func (d *recipeSessionDriver) Capture(ctx context.Context) (string, error) {
	return d.deps.Tmux.CapturePane(ctx, d.session, d.scrollback)
}

func (d *recipeSessionDriver) SendCommand(ctx context.Context, body string) error {
	if d.ar != nil {
		d.ar.injectedPrompt += body + "\n"
	}
	return injectText(ctx, d.cfg, d.deps, d.session, body)
}

func (d *recipeSessionDriver) SendKeys(ctx context.Context, tokens string) error {
	return d.deps.Tmux.SendKeys(ctx, d.session, tokens, false)
}

func (d *recipeSessionDriver) AutoRespond(ctx context.Context) (bool, error) {
	if d.ar == nil {
		return false, nil
	}
	_, rc := d.ar.tick(ctx, d.session)
	return rc == 85 || rc == 86, nil
}

func newRecipeDriver(cfg *Config, deps Deps, cli string) (*recipeSessionDriver, string, error) {
	m, err := LoadManifest(cli)
	if err != nil {
		return nil, "", fmt.Errorf("recipe: %w", err)
	}
	if strings.HasPrefix(cli, "codex") {
		if err := pretrustCodexProjects(cfg); err != nil {
			fmt.Fprintf(deps.Stderr, "[recipe] WARN codex pretrust: %v (continuing — best-effort)\n", err)
		}
	}
	deps.logUnsandboxed(PurposeProbe, m.Binary)
	session, _ := resolveSession(cfg, deps, recipeSessionPrefix)
	workingDir := cfg.Worktree
	if workingDir == "" {
		if v, _ := lookupEnv(deps, ipcenv.FleetKey); envchain.BoolValue(v, false) {
			return nil, "", fmt.Errorf("recipe: %w", errWorktreeRequired)
		}
		workingDir, _ = os.Getwd()
		fmt.Fprintf(deps.Stderr, "[recipe] WARN no worktree designated — falling back to process cwd %s (single-driver mode only; fleet mode refuses this)\n", workingDir)
	}
	drv := &recipeSessionDriver{
		cfg:        cfg,
		deps:       deps,
		session:    session,
		launchCmd:  launchCmdLine(resolveBinary(deps, m.Binary), cfg.Realization.LaunchFlags),
		workingDir: workingDir,
		marker:     m.PromptMarker,
		scrollback: recipeBootScrollback,
		ar:         newAutoResponder(cli, cfg.Workspace, deps, false, recipeBootScrollback),
	}
	return drv, m.PromptMarker, nil
}

func newRecipeEngine(cfg *Config, deps Deps, cli string) (*recipe.Engine, error) {
	drv, marker, err := newRecipeDriver(cfg, deps, cli)
	if err != nil {
		return nil, err
	}
	return &recipe.Engine{
		Driver:       drv,
		Clock:        sleepClock{sleep: deps.Sleep},
		Log:          deps.Stderr,
		PromptMarker: marker,
	}, nil
}

const helpCaptureSettleTicks = 8

func captureControl(ctx context.Context, cfg *Config, deps Deps, cli, command string, settleTicks int) (string, error) {
	deps = deps.withDefaults()
	drv, marker, err := newRecipeDriver(cfg, deps, cli)
	if err != nil {
		return "", err
	}
	defer reapEphemeralSession(cfg, deps, drv.session)
	if err := drv.EnsureSession(ctx); err != nil {
		return "", fmt.Errorf("recipe: ensure session: %w", err)
	}
	if err := drv.SendCommand(ctx, command); err != nil {
		return "", err
	}
	var pane string
	for i := 0; i < settleTicks; i++ {
		if ctx.Err() != nil {
			return pane, ctx.Err()
		}
		deps.Sleep(time.Second)
		if drv.ar != nil {
			drv.ar.tick(ctx, drv.session)
		}
		pane, _ = drv.Capture(ctx)
		if marker != "" && strings.Contains(pane, marker) {
			break
		}
	}
	return pane, nil
}

func reapEphemeralSession(cfg *Config, deps Deps, session string) {
	if cfg.SessionName != "" {
		return
	}
	if err := killSessionSwept(context.Background(), deps, session); err != nil {
		fmt.Fprintf(deps.Stderr, "[recipe] WARN session %s not reaped: %v\n", session, err)
	}
}

func CaptureHelp(ctx context.Context, cfg *Config, deps Deps, cli string) (string, error) {
	return captureControl(ctx, cfg, deps, cli, "/help", helpCaptureSettleTicks)
}

func RunRecipe(ctx context.Context, cfg *Config, deps Deps, cli, recipeName string, params map[string]string) (recipe.Result, error) {
	deps = deps.withDefaults()
	r, err := recipe.LoadRecipe(recipeName)
	if err != nil {
		return recipe.Result{}, err
	}
	eng, err := newRecipeEngine(cfg, deps, cli)
	if err != nil {
		return recipe.Result{}, err
	}
	return eng.Run(ctx, r, cli, recipe.Params(params))
}
