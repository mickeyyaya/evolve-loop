package bridge

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/launchoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

// SSOT IPC-protocol-allowed: bridge engine -> REPL subprocess pidfile handoff,
// not an operator dial (split so the flagreaders AST guard does not flag it).
const bridgePidfileEnv = "EVOLVE_" + "BRIDGE_PIDFILE"

// CmdRunner is the subprocess seam: it runs name with args/env in dir,
// writing to stdout/stderr, and returns the exit code, or a non-nil err
// only on an unrecoverable failure (binary not found, context cancellation).
type CmdRunner func(ctx context.Context, name, dir string, args, env []string,
	stdin io.Reader, stdout, stderr io.Writer) (exitCode int, err error)

// Deps carries the injectable seams shared by the Engine and its Drivers; a
// zero-value field is replaced with its production default in NewEngine, so
// callers only set what they want to override.
type Deps struct {
	Runner CmdRunner
	Now    func() time.Time
	// NewChallengeToken mints the dry-run / artifact challenge token.
	NewChallengeToken func() (string, error)
	CaptureBaseline   func(*Config) dispatchBaseline
	// Env is the request-local environment overlay consulted ahead of
	// os.Getenv (via envchain); nil is empty.
	Env map[string]string
	// RecoveryStage is the Unified Phase Recovery rollout stage; empty
	// resolves to shadow (channel.ResolveStage).
	RecoveryStage string
	// FatalPaneStage is the fatal-pane fast-fail's own rollout stage; empty
	// resolves to shadow.
	FatalPaneStage string
	// ContextFillWarnPct is the policy-resolved context-fill WARN threshold
	// (percent of context window); zero uses the bridge's built-in default,
	// matching policy's own.
	ContextFillWarnPct int
	// Typed timing fields (from BridgePolicy). Zero = use bridge built-in default.
	ScrollbackLines    int
	BootTimeoutS       int
	ArtifactTimeoutS   int
	ArtifactMaxExtends int
	// PhaseArtifactTimeoutS is the policy-resolved per-phase artifact-wait
	// budget (seconds) keyed on agent label; a missing or non-positive entry
	// falls open to the built-in default.
	PhaseArtifactTimeoutS map[string]int
	// CorroborateWall is the out-of-band truth check behind the exhaustion
	// fast-fail; nil falls back to the pane match being the verdict.
	CorroborateWall WallCorroborator
	// Stdout/Stderr are the bridge's own diagnostic streams, not the inner
	// CLI's (a driver redirects those to the log files in Config).
	Stdout io.Writer
	Stderr io.Writer
	// LookupEnv resolves an environment variable, like os.LookupEnv; the
	// credential-isolation guards consult it to detect a key (e.g.
	// ANTHROPIC_API_KEY) the inner CLI would inherit via driverEnv.
	LookupEnv func(key string) (string, bool)
	Tmux      TmuxController
	// Sleep paces the *-tmux REPL-boot and artifact-wait poll loops; the loop
	// bound is an iteration counter, not wall clock, so a no-op Sleep in
	// tests still terminates.
	Sleep    func(time.Duration)
	LookPath func(file string) (string, error)
	// Reviewer adjudicates a pipeline StopEvent (the artifact wait elapsing a
	// review interval) into extend/pause/stop.
	Reviewer StopReviewer
	// SandboxWrap computes the OS-sandbox prefix argv for a source-writing
	// phase: (argv, true) when the host can sandbox and policy allows it,
	// (nil, false) when sandboxing is unavailable or disabled, in which case
	// drivers run unwrapped. cfg.Worktree=="" callers can skip this seam.
	SandboxWrap SandboxWrapper
	// BootTimeoutStore records driver-scoped boot-timeout bench strikes;
	// reaching clihealth.DefaultBootBenchThreshold promotes the driver to an
	// active bench for llmroute.ApplyDriverBench to demote. Nil disables.
	BootTimeoutStore *clihealth.Store
	OnStopReview     func(phase, action, reason string)
	// OnBoot is called once by a tmux-REPL driver when the REPL prompt
	// marker first appears, reporting cold-boot latency in milliseconds.
	OnBoot func(bootMS int64)
	// onModelDispatch is a call-local hook Launch installs; deliberately
	// package-private since model-attempt persistence has one writer.
	onModelDispatch func(modelDispatch)
	// onFatalPane is a call-local hook Launch installs: the wait loop
	// reports a preempting fatal-pane verdict's cause, session named-ness
	// and wait interval — the one channel the fresh-session retry reads.
	// Package-private like onModelDispatch.
	onFatalPane func(fatalPaneObservation)
	// KeychainProbe reports whether a macOS login-Keychain generic-password
	// item exists for the given service; doctorAuth needs it because
	// claude's OAuth token lives in the Keychain (service "Claude
	// Code-credentials"), not a file, so a file-only check would
	// false-negative.
	KeychainProbe func(service string) bool
	// MkScratchDir creates a fresh private scratch directory per dispatch so
	// transient files (the macOS SBPL sandbox profile) don't collide when
	// two same-phase dispatches share one workspace.
	MkScratchDir func(dir, pattern string) (string, error)
	// LivenessCenter is the authoritative liveness source the stop-review
	// checkpoint observes; nil has the driver build its own per run. An
	// injected instance must be per-dispatch — newReplWaitState registers a
	// handler with no unregister, so reuse would accumulate stale handlers.
	LivenessCenter *panestream.LivenessCenter
	// Signals is the Signal Center the engine emits into: bridge.warning/
	// bridge.tripwire from attempt telemetry, and pane.liveness through the
	// tmux driver's LivenessHandler. nil is the Null Object for tests.
	Signals *signalcenter.Center
	// TokenResolver recovers token usage for a completed Launch window; nil
	// leaves counts unavailable while the ledger still records
	// dispatch/latency/outcome, and a resolver error fails open (WARNed,
	// never fails the Launch).
	TokenResolver func(tokenusage.Window) (tokenusage.Result, error)
}

// SandboxWrapper is the bridge's view of the sandbox decision: a named
// function type so tests can substitute without naming the function type
// inline.
type SandboxWrapper func(req SandboxWrapRequest) (prefixArgv []string, available bool)

// SandboxWrapRequest carries everything SandboxWrap needs to decide and
// emit a sandbox prefix.
type SandboxWrapRequest struct {
	TerminalPath  string   // assigned tmux tty; empty for headless launches
	DenyPaths     []string // absolute write denials
	DenyReadPaths []string // absolute read denials
	Phase         string   // e.g. "build", "tdd"
	Workspace     string   // absolute path; SBPL file lives here on darwin
	Worktree      string   // absolute path; the only write-allowed location
	RepoRoot      string   // absolute path; the read-only main repo root
	// WriteSubpaths are the phase profile's declared sandbox.write_subpaths
	// (repo-relative, absolute, or {worktree_path}-prefixed), granted on top
	// of the write floor every sandboxed phase already gets; a narrower
	// declaration documents intent without narrowing the floor.
	WriteSubpaths []string
	// AllowNetwork is always forced true on the sandboxPrefixForLaunch path:
	// a sandboxed phase runs a cloud CLI that needs the model API.
	AllowNetwork bool
}

func defaultIfZero(val, def int) int {
	if val > 0 {
		return val
	}
	return def
}

// withDefaults returns a copy of d with each zero-value seam replaced by
// its production default.
func (d Deps) withDefaults() Deps {
	if d.CaptureBaseline == nil {
		d.CaptureBaseline = captureArtifactBaseline
	}
	if d.Runner == nil {
		d.Runner = execRunner
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.NewChallengeToken == nil {
		d.NewChallengeToken = defaultChallengeToken
	}
	if d.Stdout == nil {
		d.Stdout = os.Stdout
	}
	if d.Stderr == nil {
		d.Stderr = os.Stderr
	}
	if d.LookupEnv == nil {
		d.LookupEnv = os.LookupEnv
	}
	if d.Tmux == nil {
		d.Tmux = execTmux{}
	}
	if d.Sleep == nil {
		d.Sleep = time.Sleep
	}
	if d.LookPath == nil {
		d.LookPath = exec.LookPath
	}
	if d.Reviewer == nil {
		d.Reviewer = newDeterministicReviewer(defaultIfZero(d.ArtifactMaxExtends, defaultArtifactMaxExtends))
	}
	if d.MkScratchDir == nil {
		d.MkScratchDir = os.MkdirTemp
	}
	if d.SandboxWrap == nil {
		// defaultSandboxWrap captures d, so MkScratchDir must be defaulted first.
		d.SandboxWrap = defaultSandboxWrap(d)
	}
	if d.KeychainProbe == nil {
		d.KeychainProbe = defaultKeychainProbe(d)
	}
	return d
}

// Config is the fully-resolved launch configuration — flags, env and
// profile merged to concrete values — that the Engine populates once and
// hands to the selected Driver; its field set mirrors the bin/bridge launch
// flag surface.
type Config struct {
	CLI        string
	Profile    string
	Model      string // effective model — "auto" already resolved
	PromptFile string
	Workspace  string
	StdoutLog  string
	StderrLog  string
	Artifact   string
	// SecondaryArtifacts: extra contract deliverables (absolute paths)
	// required alongside the primary artifact.
	SecondaryArtifacts []string
	Completion         core.CompletionContract
	Cycle              int
	Worktree           string
	RunID              string
	ProjectRoot        string // absolute path; sandbox uses this as the read-only RepoRoot
	Agent              string
	PermissionMode     string // "" = driver default
	StreamOutput       bool
	SessionName        string
	AllowBypass        bool
	HumanInput         bool
	RequireFull        bool
	RequireSandbox     bool // fail closed when OS filesystem confinement is unavailable
	DenyPaths          []string
	DenyReadPaths      []string
	// SandboxWriteSubpaths is profile.sandbox.write_subpaths, carried verbatim
	// to the wrapper (SandboxWrapRequest.WriteSubpaths), which resolves them.
	SandboxWriteSubpaths []string
	AllowedTools         []string // from profile.allowed_tools
	ExtraFlags           []string // forwarded to the inner CLI after `--` (direct passthrough)
	// Realization is the per-CLI launch realization (ADR-0022): the model,
	// permission and raw flags this CLI understands, resolved from a
	// LaunchIntent against its manifest; drivers build launch commands from
	// Realization.LaunchFlags so one CLI's argv never leaks into another.
	Realization Realization
	// ArtifactTimeoutS overrides the *-tmux artifact-wait deadline (seconds);
	// 0 uses the built-in default (300s).
	ArtifactTimeoutS int
	// BootOnly turns a *-tmux launch into a boot smoke-test: it boots the
	// CLI and waits for the prompt marker, then exits without delivering a
	// prompt or waiting for an artifact.
	BootOnly bool
	// AnthropicBaseURL is the policy-sourced proxy URL override; non-empty
	// fires the claude-tmux proxy guard.
	AnthropicBaseURL string
	// AllowNetwork carries profile.sandbox.allow_network; on the OS-sandbox
	// path it is forced true regardless (a sandboxed phase's cloud CLI must
	// reach the model API), so this field only decides whether a misconfig
	// WARN fires.
	AllowNetwork bool
	// codexConfigPath overrides ~/.codex/config.toml for
	// pretrustCodexProjects; tests set it to avoid touching the real file.
	codexConfigPath string
}

// Engine is the core.Bridge implementation: Launch runs a fixed pipeline
// that only the dispatch step varies by Driver; a single instance is safe
// for sequential reuse.
type Engine struct {
	deps Deps
}

// NewEngine constructs an Engine, filling in default seams. Pass a Deps
// with only the fields you need to override (typically just Runner +
// Now + Env in tests).
func NewEngine(deps Deps) *Engine {
	d := deps.withDefaults()
	if d.TokenResolver == nil {
		// Fail-open stays loud: lifecycle/outcome records continue without
		// token counts.
		// See ADR-0101.
		d.Signals.Emit(signalcenter.Event{
			Module: signalcenter.ModuleBridge, Origin: "NewEngine", Kind: signalcenter.KindBridgeWarning,
			Severity: signalcenter.SeverityWarn, Code: CodeTokenResolverMissing,
			Reason: "Deps.TokenResolver is nil — token usage enrichment unavailable (fail-open: lifecycle/outcome records continue without token counts)",
		})
	}
	return &Engine{deps: d}
}

// SignalsWired reports whether a Signal Center reached this Engine.
func (e *Engine) SignalsWired() bool { return e.deps.Signals != nil }

// HasTokenResolver reports whether this Engine was wired with a non-nil
// TokenResolver.
func (e *Engine) HasTokenResolver() bool {
	return e.deps.TokenResolver != nil
}

// resolvedModel is the single source of the "auto" model default, shared by
// the arg construction and token-usage attribution.
func resolvedModel(m string) string {
	if m == "" {
		return "auto"
	}
	return m
}

// launchArgs is the pure construction of a Launch's argument list, testable
// without a real CLI. req.BudgetScale scales deps' per-agent artifact
// budget.
// See ADR-0076.
func launchArgs(req core.BridgeRequest, promptFile, stdoutLog, stderrLog string, deps Deps) []string {
	model := resolvedModel(req.Model)
	args := []string{
		"--cli=" + req.CLI,
		"--profile=" + req.Profile,
		"--model=" + model,
		"--prompt-file=" + promptFile,
		"--workspace=" + req.Workspace,
		"--stdout-log=" + stdoutLog,
		"--stderr-log=" + stderrLog,
		"--artifact=" + req.ArtifactPath,
	}
	if len(req.SecondaryArtifacts) > 0 {
		args = append(args, "--secondary-artifacts="+strings.Join(req.SecondaryArtifacts, secondaryArtifactSep))
	}
	if req.Cycle > 0 {
		args = append(args, "--cycle="+strconv.Itoa(req.Cycle))
	}
	if req.Agent != "" {
		args = append(args, "--agent="+req.Agent)
	}
	if budget := scaledArtifactBudget(deps.PhaseArtifactTimeoutS[req.Agent], req.BudgetScale); budget > 0 {
		args = append(args, "--artifact-timeout-s="+strconv.Itoa(budget))
	}
	if req.Worktree != "" {
		args = append(args, "--worktree="+req.Worktree)
	}
	if req.RunID != "" {
		args = append(args, "--run-id="+req.RunID)
	}
	if req.ProjectRoot != "" {
		// Threaded as a flag (parseLaunchArgs writes Config.ProjectRoot) so
		// args stays the single source of truth for Config construction;
		// SandboxWrap needs it as the read-only RepoRoot.
		args = append(args, "--project-root="+req.ProjectRoot)
	}
	if req.RequireSandbox {
		args = append(args, "--require-sandbox")
	}
	if req.Completion != "" {
		args = append(args, "--completion="+string(req.Completion))
	}
	// Permission mode flows as a top-level flag (→ Config.PermissionMode → the
	// LaunchIntent), NOT after `--`, so it is realized per-CLI and never pasted
	// into a non-claude launch command.
	if req.PermissionMode != "" {
		args = append(args, "--permission-mode="+req.PermissionMode)
	}
	// SessionName pins a deterministic tmux session — swarm orphan-on-cancel
	// hardening; resolveSession then uses the named-session path.
	if req.SessionName != "" {
		args = append(args, "--session-name="+req.SessionName)
	}
	// The in-process entry is the autonomous runner's trusted path, so it
	// always sets --allow-bypass for the tmux safety gates (the explicit
	// opt-in exists for ad-hoc human `evolve bridge launch` use, not the
	// orchestrator); harmless for headless drivers, which ignore it.
	args = append(args, "--allow-bypass")
	if len(req.ExtraFlags) > 0 {
		args = append(args, "--")
		args = append(args, req.ExtraFlags...)
	}
	return args
}

// Launch satisfies core.Bridge, mapping a BridgeRequest onto the LaunchArgs
// pipeline: materialize the prompt to a file, dispatch, then read the
// artifact into the response on success.
// See ADR-0103.
//
// Launch is concurrency-safe on Engine state: BootMS is captured via a
// call-local OnBoot hook on a per-call Deps copy (runScoped), so a Launch
// never mutates the shared e.deps.
func (e *Engine) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	if err := ValidateRequest(req); err != nil {
		return core.BridgeResponse{}, err
	}
	in, err := materializeInputs(req)
	if err != nil {
		return core.BridgeResponse{}, err
	}
	model := resolvedModel(req.Model)
	args := launchArgs(req, in.promptFile, in.stdoutLog, in.stderrLog, e.deps)
	run := e.freshSessionRetry(ctx, req, model, func() launchRun { return e.runAttempt(ctx, req, args) })
	resp := core.BridgeResponse{ExitCode: run.code, Stderr: run.stderr, BootMS: run.bootMS}
	// The terminal time is frozen before optional token resolution begins, so
	// one dispatch yields one attempt record even when token enrichment is
	// unavailable or errors; a Launch that ran a fresh session holds two: the
	// dead one (fresh_session_retry) and this one.
	c := e.recordModelAttempt(req, model, run.code, run.start, run.end, run.dispatched, run.stderr, &resp)
	e.clearBootStrike(c, req.CLI, run.code)
	if run.code == ExitOK {
		e.readResult(c, req, in.stdoutLog, &resp)
		return resp, nil
	}
	launchError := e.persistLaunchError(c, req.Workspace, in.agent, run.stderr)
	// What the exit MEANS — the error chain (wire shape: "bridge: launch
	// exit=<code>[: <cause>]" + the port sentinel), the ledger cause and the
	// signal code — is the classifier's ONE table. ctx.Err() is sampled after
	// the persist: only the signal-death row reads it.
	out := launchoutcome.Classify(run.code, ctx.Err(), run.stderr)
	e.recordBootStrike(c, req.CLI, run.code)
	c.launchWarn("Engine.Launch", "classify", out.Signal, out.Err.Error(), launchFields(out, launchError))
	return resp, out.Err
}

// launchInputs is what materializeInputs derives from the request: the agent
// label and the three files Launch names in the workspace.
type launchInputs struct {
	agent, promptFile, stdoutLog, stderrLog string
}

// materializeInputs ensures the workspace, writes the prompt to
// <workspace>/<agent>-prompt.txt and defaults the two log paths.
func materializeInputs(req core.BridgeRequest) (launchInputs, error) {
	if err := os.MkdirAll(req.Workspace, 0o755); err != nil {
		return launchInputs{}, fmt.Errorf("bridge: ensure workspace: %w", err)
	}
	in := launchInputs{agent: req.Agent, stdoutLog: req.StdoutLog, stderrLog: req.StderrLog}
	if in.agent == "" {
		in.agent = "agent"
	}
	in.promptFile = filepath.Join(req.Workspace, phasecontract.PromptArtifactFilename(in.agent))
	if err := os.WriteFile(in.promptFile, []byte(req.Prompt), 0o644); err != nil {
		return launchInputs{}, fmt.Errorf("bridge: write prompt: %w", err)
	}
	if in.stdoutLog == "" {
		in.stdoutLog = filepath.Join(req.Workspace, in.agent+"-stdout.log")
	}
	if in.stderrLog == "" {
		in.stderrLog = filepath.Join(req.Workspace, in.agent+"-stderr.log")
	}
	return in, nil
}

// launchRun is what one scoped run of the pipeline yields: the exit,
// captured bridge stderr, the wall-clock window and the driver's
// observations, including a fatal-pane fast-fail's observation.
type launchRun struct {
	code       int
	stderr     string
	start, end time.Time
	bootMS     int64
	dispatched modelDispatch
	fatal      fatalPaneObservation
}

// freshSessionRetry runs the launch and, when it ended on a session-
// recoverable terminal cause and freshSessionAllowed holds, records the dead
// dispatch (marked fresh_session_retry), clears its boot strike, and runs
// one fresh session of the same CLI before the caller's chain walks on. At
// most one retry — the second run's cause is never read.
func (e *Engine) freshSessionRetry(ctx context.Context, req core.BridgeRequest, model string, run func() launchRun) launchRun {
	first := run()
	if !e.freshSessionAllowed(ctx, first) {
		return first
	}
	dead := core.BridgeResponse{ExitCode: first.code, Stderr: first.stderr, BootMS: first.bootMS}
	c := e.recordModelAttempt(req, model, first.code, first.start, first.end, first.dispatched, first.stderr, &dead, markFreshSessionRetry)
	e.clearBootStrike(c, req.CLI, first.code)
	c.launchWarn("Engine.freshSessionRetry", "", CodeFreshSessionRetry,
		fmt.Sprintf("%s pane died (%s): the REPL process is gone, not the CLI — one fresh %s session before the chain moves on", req.Agent, first.fatal.cause, req.CLI), nil)
	return run()
}

// freshSessionAllowed is the retry's one gate:
//   - the cause is session-recoverable on the exact exit the fast-fail
//     closes with (a run that delivered is never retried);
//   - the session is ephemeral — a named session's lifecycle belongs to its
//     owner (resume, the swarm reaper), never to this retry;
//   - the caller's deadline leaves room for one more of the driver's actual
//     waits, else the chain gets a clean exit 81 in time to walk.
func (e *Engine) freshSessionAllowed(ctx context.Context, first launchRun) bool {
	if !first.fatal.cause.SessionRecoverable() || first.fatal.named || first.code != ExitArtifactTimeout || ctx.Err() != nil {
		return false
	}
	if deadline, ok := ctx.Deadline(); ok && deadline.Sub(e.deps.Now()) < time.Duration(first.fatal.intervalS)*time.Second {
		return false
	}
	return true
}

// markFreshSessionRetry tags the dead dispatch's ledger row.
func markFreshSessionRetry(rec *llmcalls.Record) { rec.FreshSessionRetry = true }

// runScoped runs the LaunchArgs pipeline against a scoped Engine holding a
// per-call Deps copy: the call-local OnBoot and onModelDispatch hooks chain
// any pre-wired callback, so every pipeline method reads the
// call-local hooks and the shared e.deps is never mutated — concurrent
// Launch on one Engine is race-free with no defer-restore needed.
func (e *Engine) runScoped(ctx context.Context, args []string, env map[string]string) launchRun {
	var run launchRun
	callDeps := e.deps
	prevOnBoot := callDeps.OnBoot
	callDeps.OnBoot = func(ms int64) {
		run.bootMS = ms
		if prevOnBoot != nil {
			prevOnBoot(ms)
		}
	}
	previousDispatchObserver := callDeps.onModelDispatch
	callDeps.onModelDispatch = func(observation modelDispatch) {
		run.dispatched = observation
		if previousDispatchObserver != nil {
			previousDispatchObserver(observation)
		}
	}
	previousFatalObserver := callDeps.onFatalPane
	callDeps.onFatalPane = func(observation fatalPaneObservation) {
		run.fatal = observation
		if previousFatalObserver != nil {
			previousFatalObserver(observation)
		}
	}
	callEngine := &Engine{deps: callDeps}
	var stderrBuf bytes.Buffer
	run.start = e.deps.Now()
	run.code = callEngine.LaunchArgs(ctx, args, env, io.Discard, &stderrBuf)
	run.end = e.deps.Now()
	run.stderr = stderrBuf.String()
	return run
}

// clearBootStrike resets the driver's consecutive-strike counter on any exit
// other than ExitREPLBootTimeout (the REPL booted), so non-adjacent failures
// never bench. A store fault is BRIDGE_BOOT_STRIKE_CLEAR_FAILED.
func (e *Engine) clearBootStrike(c attemptLogContext, cli string, code int) {
	if e.deps.BootTimeoutStore == nil || clihealth.IsBootTimeoutExitCode(code) {
		return
	}
	if err := e.deps.BootTimeoutStore.ClearBootStrike(cli); err != nil {
		c.launchWarn("Engine.clearBootStrike", "clear_boot_strike", CodeBootStrikeClearFailed,
			fmt.Sprintf("boot-strike clear failed for %s: %v", cli, err), nil)
	}
}

// readResult is the strategy-aware result read (ADR-0027): the stdout
// contract writes no artifact file — its answer is the captured scrollback
// (stdoutLog), so reading req.ArtifactPath would always miss. Every other
// contract reads the artifact file. An unreadable result is
// BRIDGE_RESULT_READ_FAILED, never an error: the on-disk report is the
// verdict source and Launch still returns nil with an empty Stdout.
func (e *Engine) readResult(c attemptLogContext, req core.BridgeRequest, stdoutLog string, resp *core.BridgeResponse) {
	readPath, completion := req.ArtifactPath, string(completionContractName(req.Completion))
	if req.Completion == completionStdout {
		readPath = stdoutLog
	}
	b, err := os.ReadFile(readPath)
	if err != nil {
		c.launchWarn("Engine.readResult", "read_result", CodeResultReadFailed,
			fmt.Sprintf("result read failed: path=%s completion=%s error=%v", readPath, completion, err),
			map[string]string{"path": readPath, "completion": completion})
		return
	}
	resp.Stdout = string(b)
}

// persistLaunchError keeps the captured stderr as <workspace>/<agent>-launch-
// error.txt and returns the path, or "" when there was nothing to persist or
// the write failed. A launch dying in the validate gauntlet fails before the
// per-agent stderr-log exists, so without this file the diagnostic would
// evaporate. The classifier threads the first line into the error chain;
// bridgeExitCode's digit scan stops at ':', so appending the cause never
// breaks exit-code parsing.
func (e *Engine) persistLaunchError(c attemptLogContext, workspace, agent, stderr string) string {
	if stderr == "" {
		return ""
	}
	path := filepath.Join(workspace, agent+"-launch-error.txt")
	if err := os.WriteFile(path, []byte(stderr), 0o644); err != nil {
		c.launchWarn("Engine.persistLaunchError", "persist_launch_error", CodeLaunchErrorPersistFailed,
			fmt.Sprintf("launch-error persist failed: path=%s error=%v", path, err), map[string]string{"path": path})
		return ""
	}
	return path
}

// recordBootStrike counts one boot-timeout strike for the driver on
// ExitREPLBootTimeout (reaching the bench threshold benches it for
// llmroute.ApplyDriverBench). A store fault is BRIDGE_BOOT_STRIKE_RECORD_FAILED;
// the launch error still wraps the transient sentinel.
func (e *Engine) recordBootStrike(c attemptLogContext, cli string, code int) {
	if code != ExitREPLBootTimeout || e.deps.BootTimeoutStore == nil {
		return
	}
	if _, err := e.deps.BootTimeoutStore.RecordBootStrike(cli); err != nil {
		c.launchWarn("Engine.recordBootStrike", "record_boot_strike", CodeBootStrikeRecordFailed,
			fmt.Sprintf("boot-timeout bench record failed for %s: %v", cli, err), nil)
	}
}

// launchFields is the BRIDGE_EXIT_* event's payload: the classification's
// facts plus the persisted launch-error path when there is one.
func launchFields(out launchoutcome.Outcome, launchError string) map[string]string {
	fields := map[string]string{
		"exit_code":     strconv.Itoa(out.ExitCode),
		"cause_code":    out.CauseCode,
		"transient":     strconv.FormatBool(out.Transient),
		"ctx_cancelled": strconv.FormatBool(out.CtxCancelled),
	}
	if launchError != "" {
		fields["launch_error"] = launchError
	}
	return fields
}

// defaultContextFillWarnPct is the bridge-side built-in context-fill WARN
// threshold for an unconfigured Deps.ContextFillWarnPct. It intentionally
// matches policy's own built-in: internal/bridge cannot import internal/policy
// (the adapter is what wires them), so the zero-value path here must resolve to
// the same number the operator would get from an absent policy block.
const defaultContextFillWarnPct = 60

// tripwireSuccessThreshold is the wall-clock floor separating a genuine
// unmeasured success from a quiet quota-abort: only launches that ran
// longer than this are real work worth a collector.
const tripwireSuccessThreshold = 60 * time.Second

// artifactTimeoutSummary is a Strangler Fig facade kept for the
// driver-output tests: it delegates to launchoutcome.ArtifactTimeoutSummary.
func artifactTimeoutSummary(stderr string) string {
	return launchoutcome.ArtifactTimeoutSummary(stderr)
}

// randRead is the entropy source for defaultChallengeToken — a package
// var so tests can exercise the (otherwise unreachable) read-error path.
var randRead = rand.Read

// defaultChallengeToken mints 8 random bytes as hex (16 chars), matching
// `openssl rand -hex 8` from the bash dry-run path.
func defaultChallengeToken() (string, error) {
	var b [8]byte
	if _, err := randRead(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// execRunner is the production CmdRunner: it wraps exec.CommandContext and
// maps a process exit code to (code, nil), reserving err for unrecoverable
// failures.
func execRunner(ctx context.Context, name, dir string, args, env []string,
	stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	// Empty dir → leave cmd.Dir unset → inherit caller cwd (unchanged).
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	// cmd.Run() = Start() + Wait(); split so the agent PID can be published
	// between them (behavior-identical otherwise).
	if err := cmd.Start(); err != nil {
		return -1, err
	}
	// Best-effort: publish the agent PID so the auto-spawn observer's CPU
	// liveness probe can tell a silently-thinking HEADLESS agent from a hung
	// one. Gated by bridgePidfileEnv (set only by the headless driver, so
	// tmux drivers — which use the pane probe — are unaffected). Removed on exit.
	if pidFile := envValue(env, bridgePidfileEnv); pidFile != "" {
		// cmd.Process is guaranteed non-nil after a successful Start.
		_ = os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644)
		defer func() { _ = os.Remove(pidFile) }()
	}
	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}
	return 0, nil
}

// envValue returns the value of key in a KEY=VALUE env slice, or "".
func envValue(env []string, key string) string {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return e[len(prefix):]
		}
	}
	return ""
}
