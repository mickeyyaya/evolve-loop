package bridge

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/bridge/recipe"
	"github.com/mickeyyaya/evolveloop/go/internal/envchain"
	"github.com/mickeyyaya/evolveloop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolveloop/go/internal/sessionrecord"
)

// recipe_adapter.go wires the recipe.Engine (pure, in internal/bridge/recipe)
// to the bridge's tmux primitives. The recipe package owns the SessionDriver
// port; this file is its production implementation — reusing the exact INJECT
// (injectText / SendKeys), READ (CapturePane), and DECIDE (autoResponder.tick)
// primitives the artifact-wait driver uses, so a recipe drives the REPL
// identically to a human and identically to a normal cycle phase.

const (
	// recipeBootScrollback is the universal capture depth for recipe READs. A
	// scrollback of 200 surfaces the prompt marker + recent output for both
	// visible-pane CLIs (claude/ollama) and alt-screen CLIs (codex/agy) — the
	// extra history is harmless for the former, mandatory for the latter — so
	// the adapter needs no per-CLI scrollback table.
	recipeBootScrollback = 200
	// recipeSessionPrefix names ephemeral recipe sessions distinctly from the
	// cycle drivers' sessions.
	recipeSessionPrefix = "evolve-recipe-"
)

// sleepClock adapts deps.Sleep to recipe.Clock so tests' no-op sleep flows
// through to the engine's poll loop.
type sleepClock struct{ sleep func(time.Duration) }

func (c sleepClock) Sleep(d time.Duration) { c.sleep(d) }

// shellSingleQuote wraps s in single quotes for safe use in a shell command
// line sent via tmux send-keys (e.g. `cd <dir>`), escaping interior single
// quotes. Keeps a workspace path containing spaces or shell metacharacters
// from being re-interpreted by the inner shell.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// recipeSessionDriver is the production recipe.SessionDriver. It owns one tmux
// session for the lifetime of a recipe run.
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

// EnsureSession attaches to an already-running session or boots a fresh one
// (spawn → cd → launch → wait for the prompt marker, ticking the
// auto-responder so a boot trust modal is dismissed before we declare ready).
func (d *recipeSessionDriver) EnsureSession(ctx context.Context) error {
	if d.deps.Tmux.HasSession(ctx, d.session) {
		fmt.Fprintf(d.deps.Stderr, "[recipe] attaching to existing session %s\n", d.session)
		return nil
	}
	// CB.2: bind the pane cwd at session birth when the controller can; the
	// cd keystroke stays as the second layer (capability-less controllers).
	if ws, ok := d.deps.Tmux.(workdirSessionStarter); ok {
		if err := ws.NewSessionIn(ctx, d.session, tmuxPaneWidth, tmuxPaneHeight, d.workingDir); err != nil {
			return fmt.Errorf("new-session: %w", err)
		}
	} else if err := d.deps.Tmux.NewSession(ctx, d.session, tmuxPaneWidth, tmuxPaneHeight); err != nil {
		return fmt.Errorf("new-session: %w", err)
	}
	// CB.5: record in the run registry like runTmuxREPL does — the recipe
	// path is a second creation surface; a crash before the caller's own
	// teardown must still leave the session registry-reapable. (The attach
	// branch above skips this: the session was recorded by its creator.)
	if err := sessionrecord.Append(sessionrecord.PathIn(d.cfg.Workspace), sessionrecord.Record{
		Session: d.session, RunID: d.cfg.RunID, Cycle: d.cfg.Cycle,
		Agent: d.cfg.Agent, PID: os.Getpid(),
	}); err != nil {
		fmt.Fprintf(d.deps.Stderr, "[recipe] WARN session registry append failed: %v (session %s will not be registry-reapable)\n", err, d.session)
	}
	d.deps.Sleep(time.Second)
	_ = d.deps.Tmux.SendKeys(ctx, d.session, "cd "+shellSingleQuote(d.workingDir), true)
	d.deps.Sleep(time.Second)
	_ = d.deps.Tmux.SendKeys(ctx, d.session, d.launchCmd, true)
	fmt.Fprintf(d.deps.Stderr, "[recipe] launching: %s\n", d.launchCmd)
	bootDeadlineS := defaultIfZero(d.deps.BootTimeoutS, tmuxREPLBootTimeoutS)
	for elapsed := 0; elapsed < bootDeadlineS; elapsed++ {
		d.deps.Sleep(time.Second)
		if d.ar != nil {
			d.ar.tick(ctx, d.session) // dismiss boot trust modals (codex/agy); no-op otherwise
		}
		pane, _ := d.deps.Tmux.CapturePane(ctx, d.session, d.scrollback)
		if d.marker != "" && strings.Contains(pane, d.marker) {
			fmt.Fprintf(d.deps.Stderr, "[recipe] REPL prompt (%s) detected\n", d.marker)
			return nil
		}
	}
	return fmt.Errorf("REPL prompt %q never appeared after %ds", d.marker, bootDeadlineS)
}

func (d *recipeSessionDriver) Capture(ctx context.Context) (string, error) {
	return d.deps.Tmux.CapturePane(ctx, d.session, d.scrollback)
}

// SendCommand pastes body then Enter (the slash-command idiom) and propagates
// any transport error so the engine surfaces a dead session immediately
// instead of waiting out the step's full timeout.
func (d *recipeSessionDriver) SendCommand(ctx context.Context, body string) error {
	return injectText(ctx, d.cfg, d.deps, d.session, body)
}

func (d *recipeSessionDriver) SendKeys(ctx context.Context, tokens string) error {
	return d.deps.Tmux.SendKeys(ctx, d.session, tokens, false)
}

// AutoRespond runs one auto-responder tick and reports escalation. rc 85
// (policy=escalate / missing keys) and 86 (loop-guard) are the abandon codes.
func (d *recipeSessionDriver) AutoRespond(ctx context.Context) (bool, error) {
	if d.ar == nil {
		return false, nil
	}
	_, rc := d.ar.tick(ctx, d.session)
	return rc == 85 || rc == 86, nil
}

// newRecipeDriver builds a recipeSessionDriver for cli, resolving the session
// name, launch command, prompt marker, and auto-responder from the manifest +
// config. Shared by the recipe engine and the /help-capture path.
func newRecipeDriver(cfg *Config, deps Deps, cli string) (*recipeSessionDriver, string, error) {
	m, err := LoadManifest(cli)
	if err != nil {
		return nil, "", fmt.Errorf("recipe: %w", err)
	}
	// CB.3: the recipe path bypasses the engine's CLIPreflight chokepoint, so
	// codex-family sessions must pretrust their worktree/workspace here or the
	// first boot in a fresh worktree renders the cycle-122 trust modal. Same
	// best-effort semantics as the driver preflight (the boot auto-responder
	// remains the downstream defense).
	if strings.HasPrefix(cli, "codex") {
		if err := pretrustCodexProjects(cfg); err != nil {
			fmt.Fprintf(deps.Stderr, "[recipe] WARN codex pretrust: %v (continuing — best-effort)\n", err)
		}
	}
	session, _ := resolveSession(cfg, deps, recipeSessionPrefix)
	workingDir := cfg.Worktree
	if workingDir == "" {
		// CB.2: same fail-closed contract as runTmuxREPL — the recipe path is
		// a second launch surface with the identical silent-cwd hazard.
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

// newRecipeEngine builds a recipe.Engine wired to a recipeSessionDriver for cli.
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

// helpCaptureSettleTicks bounds the post-/help poll for the prompt marker.
const helpCaptureSettleTicks = 8

// captureControl launches-or-attaches the CLI's REPL, sends a single command,
// polls up to settleTicks for the prompt marker to reappear, and returns the
// captured pane. It is the table-driven generalization of the original /help
// capture: the command is a parameter, so the same primitive drives `/help`
// (introspect) and any abstract control event (usage/status/clean_ctx) whose
// concrete command the caller resolved from the mapping table.
func captureControl(ctx context.Context, cfg *Config, deps Deps, cli, command string, settleTicks int) (string, error) {
	deps = deps.withDefaults()
	drv, marker, err := newRecipeDriver(cfg, deps, cli)
	if err != nil {
		return "", err
	}
	if err := drv.EnsureSession(ctx); err != nil {
		return "", fmt.Errorf("recipe: ensure session: %w", err)
	}
	if err := drv.SendCommand(ctx, command); err != nil {
		return "", err
	}
	var pane string
	for i := 0; i < settleTicks; i++ {
		deps.Sleep(time.Second)
		if drv.ar != nil {
			drv.ar.tick(ctx, drv.session) // dismiss any modal; harmless otherwise
		}
		pane, _ = drv.Capture(ctx)
		if marker != "" && strings.Contains(pane, marker) {
			break
		}
	}
	return pane, nil
}

// CaptureHelp launches-or-attaches the CLI's REPL, sends `/help`, and returns
// the resulting pane — the live half of `evolve bridge introspect`. The parse
// + catalog diff is done by the capabilities package on the returned text.
func CaptureHelp(ctx context.Context, cfg *Config, deps Deps, cli string) (string, error) {
	return captureControl(ctx, cfg, deps, cli, "/help", helpCaptureSettleTicks)
}

// RunRecipe loads the named recipe and executes it against cli using the tmux
// primitives configured in cfg/deps. It is the independent, orchestrator-free
// entry the `evolve bridge recipe run` CLI drives — the bridge can install a
// plugin (or run any scripted slash-command sequence) with nothing but a CLI
// name, a workspace, and parameters.
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
