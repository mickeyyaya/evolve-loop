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

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// CmdRunner is the subprocess seam. The production impl (execRunner)
// wraps exec.CommandContext; tests inject a fake to drive driver
// behavior without exec()ing a real CLI. Signature matches the adapter's
// CmdRunner verbatim so the two are interchangeable during the M7
// cutover.
//
// Return value is the exit code; err is non-nil only on truly
// unrecoverable failures (binary not found, context cancellation). A
// non-zero exit code with err == nil is the normal "process ran but
// failed" path.
//
// dir is the subprocess working directory. Source-writing phase drivers
// (claude-p/codex/agy) pass cfg.Worktree so the inner CLI writes into the
// per-cycle worktree rather than the parent cwd (= main repo root) — parity
// with the tmux driver's `cd <worktree>`. An empty dir leaves cmd.Dir unset,
// so the subprocess inherits the caller cwd (UNCHANGED behavior for the
// git/probe utility callers that pass "").
type CmdRunner func(ctx context.Context, name, dir string, args, env []string,
	stdin io.Reader, stdout, stderr io.Writer) (exitCode int, err error)

// Deps carries the injectable seams shared by the Engine and its
// Drivers. All fields default to real implementations in NewEngine; a
// zero-value field is replaced with its default so callers only set what
// they want to override. Later milestones extend this with the tmux
// controller and filesystem boundary.
type Deps struct {
	// Runner executes inner-CLI subprocesses. Defaults to execRunner.
	Runner CmdRunner
	// Now supplies timestamps. Defaults to time.Now (UTC formatting is
	// the caller's responsibility). Injected for deterministic tests.
	Now func() time.Time
	// NewChallengeToken mints the dry-run / artifact challenge token
	// (bash used `openssl rand -hex 8`). Defaults to 8 random bytes hex.
	NewChallengeToken func() (string, error)
	// Env is the request-local environment overlay consulted ahead of
	// os.Getenv (via envchain). nil is treated as empty.
	Env map[string]string
	// Stdout/Stderr are the bridge's own diagnostic streams (NOT the
	// inner CLI's stdout/stderr — a driver redirects those to the log
	// files named in Config). Drivers write their `[driver] ...` notes
	// here. LaunchArgs overrides these per-call with the caller's
	// writers. Default os.Stdout / os.Stderr.
	Stdout io.Writer
	Stderr io.Writer
	// LookupEnv resolves an environment variable, like os.LookupEnv. The
	// credential-isolation guards consult it to detect a key (e.g.
	// ANTHROPIC_API_KEY) that the in-process inner CLI would inherit via
	// driverEnv. Injected in tests for a controlled env without touching
	// the global process env. Default os.LookupEnv.
	LookupEnv func(key string) (string, bool)
	// Tmux drives interactive REPLs for the *-tmux drivers. Default
	// execTmux (shells to tmux); tests inject a scriptable fake.
	Tmux TmuxController
	// Sleep paces the *-tmux REPL-boot and artifact-wait poll loops.
	// Default time.Sleep; tests inject a no-op so the loops iterate
	// instantly (the loop bound is an iteration counter, not wall clock).
	Sleep func(time.Duration)
	// LookPath resolves a binary on PATH, like exec.LookPath. Probe uses
	// it to detect available CLIs + tier dependencies. Default
	// exec.LookPath; tests inject a controlled set.
	LookPath func(file string) (string, error)
	// Reviewer adjudicates a pipeline StopEvent (e.g. the artifact wait
	// elapsing a review interval) into extend/pause/stop. Default
	// deterministicReviewer (output-progress heuristic); tests and the
	// future LLM/orchestrator reviewer inject their own. See stopreview.go.
	Reviewer StopReviewer
	// SandboxWrap computes the OS-sandbox prefix argv for a source-writing
	// phase (Workstream B — CLI-agnostic confinement). Returns
	// (prefixArgv, true) when the host can sandbox AND the policy allows it;
	// (nil, false) when sandboxing is unavailable or disabled — drivers then
	// run unwrapped (degraded). cfg.Worktree=="" callers can skip this seam
	// entirely (only source-writing phases need confinement).
	//
	// On macOS the prefix is ["sandbox-exec","-p","<sbpl-file>"]; the SBPL is
	// written to a file (not inlined) so SendKeys doesn't have to shell-quote
	// a multi-line profile. On Linux it is the bwrap prefix slice.
	//
	// Default reads cfg.SandboxMode from deps.Env + nested-claude / Probe.
	SandboxWrap SandboxWrapper
	// OnStopReview is called when a stop-review decision is made.
	// Nil-safe: drivers must check if it is non-nil before invoking.
	OnStopReview func(phase, action, reason string)
	// OnBoot is called once by a tmux-REPL driver when the REPL prompt marker
	// first appears, reporting the cold-boot latency in milliseconds (ADR-0043
	// A0 instrumentation). Not called on a warm/resumed named session (no boot)
	// or by headless drivers. Nil-safe: drivers check before invoking. The
	// Engine wires this per-Launch to populate BridgeResponse.BootMS.
	OnBoot func(bootMS int64)
	// KeychainProbe reports whether a macOS login-Keychain generic-password
	// item exists for the given service. doctorAuth consults it for claude,
	// whose OAuth token Claude Code stores in the Keychain (service
	// "Claude Code-credentials") rather than a file — so a file-only check
	// false-negatives on a Keychain-authenticated host. Default
	// (defaultKeychainProbe): on darwin shells to `security find-generic-password`
	// via Runner; on other OSes always false (no Keychain). Tests inject a
	// deterministic stub.
	KeychainProbe func(service string) bool
}

// SandboxWrapper is the bridge's view of the sandbox decision — the bridge
// package depends on adapters/sandbox via its Config type. Kept as a named
// type so tests can substitute without naming the function type inline.
type SandboxWrapper func(req SandboxWrapRequest) (prefixArgv []string, available bool)

// SandboxWrapRequest carries everything the wrapper needs to decide + emit a
// prefix. Phase is the agent name (used as the SBPL file suffix). Workspace
// is the absolute path to write the per-phase SBPL into when needed.
type SandboxWrapRequest struct {
	Phase     string // e.g. "build", "tdd"
	Workspace string // absolute path; SBPL file lives here on darwin
	Worktree  string // absolute path; the only write-allowed location
	RepoRoot  string // absolute path; the read-only main repo root
}

// withDefaults returns a copy of d with any zero-value seam replaced by
// its production default. Keeps NewEngine and tests from each repeating
// the defaulting logic.
func (d Deps) withDefaults() Deps {
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
		// envInt reads via d.LookupEnv, defaulted just above.
		d.Reviewer = newDeterministicReviewer(envInt(d, "EVOLVE_ARTIFACT_MAX_EXTENDS", defaultArtifactMaxExtends))
	}
	if d.SandboxWrap == nil {
		d.SandboxWrap = defaultSandboxWrap(d)
	}
	if d.KeychainProbe == nil {
		d.KeychainProbe = defaultKeychainProbe(d)
	}
	return d
}

// Config is the fully-resolved launch configuration: flags, env, and
// profile merged down to concrete values (e.g. Model already has "auto"
// resolved against the profile). The Engine populates it once and hands
// it to the selected Driver, so drivers never re-parse flags or re-read
// the profile. Field set mirrors the bin/bridge launch flag surface.
type Config struct {
	CLI        string
	Profile    string
	Model      string // effective model — "auto" already resolved
	PromptFile string
	Workspace  string
	StdoutLog  string
	StderrLog  string
	Artifact   string
	// Completion selects the phase-completion contract (ADR-0027): "" /
	// "artifact" = poll for the artifact file (default, legacy); "stdout" =
	// complete on REPL-idle for agents that print their answer (router/advisor).
	Completion string
	Cycle      int
	Worktree   string
	// RunID is the CA.5 run identity (CB.5): non-empty → tmux session names
	// carry the r<runid8> run token and the per-run registry is stamped with it.
	RunID          string
	ProjectRoot    string // absolute path; sandbox uses this as the read-only RepoRoot (WS-B)
	Agent          string
	PermissionMode string // "" = driver default
	StreamOutput   bool
	SessionName    string
	AllowBypass    bool
	HumanInput     bool
	RequireFull    bool
	AllowedTools   []string // from profile.allowed_tools
	ExtraFlags     []string // forwarded to the inner CLI after `--` (direct passthrough)
	// Realization is the per-CLI launch realization (ADR-0022): the model,
	// permission, and raw flags this CLI actually understands, resolved from a
	// LaunchIntent against the CLI's manifest. The *-tmux drivers build their
	// launch command from Realization.LaunchFlags rather than constructing
	// model/permission flags inline, so one CLI's argv never leaks into another.
	Realization Realization
	// ArtifactTimeoutS overrides the *-tmux artifact-wait deadline (seconds);
	// 0 → tmuxArtifactTimeoutS (300). A per-launch control for callers that
	// want a tighter ceiling than the default — e.g. fast agents, or a probe
	// that should fail quickly rather than wait the full five minutes.
	ArtifactTimeoutS int
	// BootOnly turns a *-tmux launch into a boot smoke-test: the shared REPL
	// state machine boots the CLI and waits for the prompt marker, then exits
	// cleanly WITHOUT delivering a prompt or waiting for an artifact. Used by
	// BootSmokeTest / the loop readiness gate to verify the bridge can boot the
	// CLI before any real work (and LLM budget) is committed.
	BootOnly bool
}

// Engine is the core.Bridge implementation and the Template Method host:
// Launch() runs the fixed pipeline (validate → resolveConfig → preflight
// → dispatch(driver) → report) while Drivers vary only the dispatch
// step. A single Engine instance is safe for sequential reuse.
type Engine struct {
	deps Deps
}

// NewEngine constructs an Engine, filling in default seams. Pass a Deps
// with only the fields you need to override (typically just Runner +
// Now + Env in tests).
func NewEngine(deps Deps) *Engine {
	return &Engine{deps: deps.withDefaults()}
}

// Launch satisfies core.Bridge: the in-process entry the M7 adapter
// cutover routes to. It maps a BridgeRequest onto the LaunchArgs pipeline
// (materializing req.Prompt to a file, mirroring the bash bridge's
// --prompt-file contract), then reads the artifact into the response on
// success — matching the existing subprocess adapter's behavior so the
// cutover is a drop-in.
//
// NOT safe for concurrent calls on the same Engine: Launch installs a
// call-local OnBoot hook on e.deps (restored via defer) to capture BootMS.
// Production always builds a fresh Engine per Launch (adapters/bridge), so the
// "sequential reuse" contract holds; a future caller must not Launch the same
// Engine from multiple goroutines.
func (e *Engine) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	switch "" {
	case req.CLI:
		return core.BridgeResponse{}, errors.New("bridge: CLI required")
	case req.Profile:
		return core.BridgeResponse{}, errors.New("bridge: Profile required")
	case req.Workspace:
		return core.BridgeResponse{}, errors.New("bridge: Workspace required")
	case req.ArtifactPath:
		return core.BridgeResponse{}, errors.New("bridge: ArtifactPath required")
	}
	if err := os.MkdirAll(req.Workspace, 0o755); err != nil {
		return core.BridgeResponse{}, fmt.Errorf("bridge: ensure workspace: %w", err)
	}
	agent := req.Agent
	if agent == "" {
		agent = "agent"
	}
	promptFile := filepath.Join(req.Workspace, agent+"-prompt.txt")
	if err := os.WriteFile(promptFile, []byte(req.Prompt), 0o644); err != nil {
		return core.BridgeResponse{}, fmt.Errorf("bridge: write prompt: %w", err)
	}
	stdoutLog := req.StdoutLog
	if stdoutLog == "" {
		stdoutLog = filepath.Join(req.Workspace, agent+"-stdout.log")
	}
	stderrLog := req.StderrLog
	if stderrLog == "" {
		stderrLog = filepath.Join(req.Workspace, agent+"-stderr.log")
	}
	model := req.Model
	if model == "" {
		model = "auto"
	}
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
	if req.Cycle > 0 {
		args = append(args, "--cycle="+strconv.Itoa(req.Cycle))
	}
	if req.Agent != "" {
		args = append(args, "--agent="+req.Agent)
	}
	if req.Worktree != "" {
		args = append(args, "--worktree="+req.Worktree)
	}
	if req.RunID != "" {
		// CB.5: run identity → run-scoped session names + per-run registry.
		args = append(args, "--run-id="+req.RunID)
	}
	if req.ProjectRoot != "" {
		// Workstream B: SandboxWrap needs the read-only RepoRoot. Threaded as
		// a flag (parseLaunchArgs writes Config.ProjectRoot) so the args path
		// stays the single source of truth for Config construction.
		args = append(args, "--project-root="+req.ProjectRoot)
	}
	if req.Completion != "" {
		args = append(args, "--completion="+req.Completion)
	}
	// Permission mode flows as a top-level flag (→ Config.PermissionMode → the
	// LaunchIntent), NOT after `--`, so it is realized per-CLI and never pasted
	// into a non-claude launch command.
	if req.PermissionMode != "" {
		args = append(args, "--permission-mode="+req.PermissionMode)
	}
	// SessionName pins a deterministic tmux session (swarm orphan-on-cancel
	// hardening). parseLaunchArgs→LaunchArgs already validates + threads it into
	// Config.SessionName; resolveSession then uses the named-session path.
	if req.SessionName != "" {
		args = append(args, "--session-name="+req.SessionName)
	}
	// The in-process entry is the autonomous runner's trusted path: it is the
	// bypass authority, so it enables --allow-bypass for the tmux safety gates
	// (the explicit-opt-in gate exists for ad-hoc human `evolve bridge launch`
	// use, not for the programmatic orchestrator). Harmless for headless
	// drivers, which do not consult AllowBypass.
	args = append(args, "--allow-bypass")
	if len(req.ExtraFlags) > 0 {
		args = append(args, "--")
		args = append(args, req.ExtraFlags...)
	}

	// Capture the cold-boot latency the tmux-REPL driver reports via OnBoot
	// (ADR-0043 A0) into this call's BridgeResponse, chaining any pre-wired
	// callback. The production engine is built fresh per Launch, so mutating
	// e.deps here is call-local; the defer restores it for any sequential reuse.
	var bootMS int64
	prevOnBoot := e.deps.OnBoot
	e.deps.OnBoot = func(ms int64) {
		bootMS = ms
		if prevOnBoot != nil {
			prevOnBoot(ms)
		}
	}
	defer func() { e.deps.OnBoot = prevOnBoot }()

	var stderrBuf bytes.Buffer
	code := e.LaunchArgs(ctx, args, req.Env, io.Discard, &stderrBuf)
	resp := core.BridgeResponse{ExitCode: code, Stderr: stderrBuf.String(), BootMS: bootMS}
	if code == ExitOK {
		// Strategy-aware result read (ADR-0027): the stdout contract writes no
		// artifact file — its answer is the captured scrollback (stdoutLog), so
		// reading req.ArtifactPath would always miss. Every other contract reads
		// the artifact file as before.
		readPath := req.ArtifactPath
		if req.Completion == "stdout" {
			readPath = stdoutLog
		}
		if b, err := os.ReadFile(readPath); err == nil {
			resp.Stdout = string(b)
		}
		return resp, nil
	}
	// R3.6 (inbox bridge-launch-validation-stderr-lost): a launch dying in
	// the validate gauntlet fails BEFORE the per-agent stderr-log exists, so
	// without these two lines the diagnostic evaporates (cycle-270: a bare
	// "launch exit=10" cost a forensic session; the cause was one missing
	// profile file). Persist the captured stderr into the run dir and thread
	// its first line into the error chain so <phase>-failure-diag.json
	// carries the "[bridge] …" cause. bridgeExitCode's digit scan stops at
	// the ':', so appending the cause never breaks exit-code parsing.
	if stderrBuf.Len() > 0 {
		_ = os.WriteFile(filepath.Join(req.Workspace, agent+"-launch-error.txt"), stderrBuf.Bytes(), 0o644)
	}
	msg := fmt.Sprintf("bridge: launch exit=%d", code)
	if cause := firstDiagnosticLine(stderrBuf.String()); cause != "" {
		msg += ": " + cause
	}
	// Wrap the artifact-timeout exit with the port-level sentinel so the
	// generic phase runner can errors.Is-match it (Workstream D soft-fail)
	// without importing this adapter. Other non-zero codes stay plain.
	if code == ExitArtifactTimeout {
		return resp, fmt.Errorf("%s: %w", msg, core.ErrArtifactTimeout)
	}
	if code == ExitREPLBootTimeout || code == ExitUnknownPrompt || code == ExitRespondLoopGuard {
		return resp, fmt.Errorf("%s: %w", msg, core.ErrTransientBridgeFailure)
	}
	return resp, errors.New(msg)
}

// firstDiagnosticLine picks the one-line cause threaded into the launch
// error chain. Validate-gauntlet failures put the cause FIRST and prefix it
// "[bridge]"; driver failures accumulate launch chatter first and end with
// the causal line (cycle-286: a timeout's first line was a stream_output
// NOTE while "FAIL: completion never signalled" sat last). So: the first
// "[bridge]"-prefixed line wins; otherwise the LAST non-empty line.
func firstDiagnosticLine(stderr string) string {
	last := ""
	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[bridge]") {
			return boundCause(line)
		}
		last = line
	}
	return boundCause(last)
}

// boundCause caps the cause line rune-safely (never split UTF-8 mid-sequence).
func boundCause(line string) string {
	const maxCause = 300
	if runes := []rune(line); len(runes) > maxCause {
		return string(runes[:maxCause]) + "…"
	}
	return line
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

// execRunner is the production CmdRunner — wraps exec.CommandContext and
// maps a process exit code to (code, nil), reserving err for
// unrecoverable failures. Ported verbatim from the adapter so behavior
// is identical across the cutover.
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
	// one. Gated by EVOLVE_BRIDGE_PIDFILE (set only by the headless driver, so
	// tmux drivers — which use the pane probe — are unaffected). Removed on exit.
	if pidFile := envValue(env, "EVOLVE_BRIDGE_PIDFILE"); pidFile != "" {
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
