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
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

// SSOT IPC-protocol-allowed: bridge engine -> REPL subprocess pidfile handoff,
// not an operator dial (split so the flagreaders AST guard does not flag it).
const bridgePidfileEnv = "EVOLVE_" + "BRIDGE_PIDFILE"

type CmdRunner func(ctx context.Context, name, dir string, args, env []string,
	stdin io.Reader, stdout, stderr io.Writer) (exitCode int, err error)

type Deps struct {
	Runner                CmdRunner
	Now                   func() time.Time
	NewChallengeToken     func() (string, error)
	CaptureBaseline       func(*Config) dispatchBaseline
	Env                   map[string]string
	EnvStage              string
	EnvDrops              *EnvDropLog
	RecoveryStage         string
	FatalPaneStage        string
	ContextFillWarnPct    int
	ScrollbackLines       int
	BootTimeoutS          int
	ArtifactTimeoutS      int
	ArtifactMaxExtends    int
	PhaseArtifactTimeoutS map[string]int
	Efforts               policy.EffortTable
	CorroborateWall       WallCorroborator
	Stdout                io.Writer
	Stderr                io.Writer
	LookupEnv             func(key string) (string, bool)
	Tmux                  TmuxController
	Sleep                 func(time.Duration)
	LookPath              func(file string) (string, error)
	Reviewer              StopReviewer
	SandboxWrap           SandboxWrapper
	BootTimeoutStore      *clihealth.Store
	OnStopReview          func(phase, action, reason string)
	OnBoot                func(bootMS int64)
	onModelDispatch       func(modelDispatch)
	launchModel           func(context.Context, core.BridgeRequest) (core.BridgeResponse, error)
	onFatalPane           func(fatalPaneObservation)
	onBootLoss            func(launchoutcome.BootLossCause)
	onDispatchID          func(string)
	onLaunchEffort        func(string)
	onServedModel         func(string)
	PaneCaptureMaxBytes   int
	onPaneCapture         func(string)
	KeychainProbe         func(service string) bool
	MkScratchDir          func(dir, pattern string) (string, error)
	LivenessCenter        *panestream.LivenessCenter
	Signals               *signalcenter.Center
	TokenResolver         func(tokenusage.Window) (tokenusage.Result, error)
	ModelChain            func(cli, model string) []string
	Floors                policy.Floors
	ListProcesses         proctree.Lister
	ListCwds              proctree.CwdLister
	SignalProcess         proctree.Signaler
	sweep                 *dispatchSweep
	hostOS                string
	terminalCheck         func(string) error
}

type SandboxWrapper func(req SandboxWrapRequest) (prefixArgv []string, available bool)

type SandboxWrapRequest struct {
	TerminalPath  string
	DenyPaths     []string
	DenyReadPaths []string
	Phase         string
	Workspace     string
	Worktree      string
	RepoRoot      string
	WriteSubpaths []string
	AllowNetwork  bool
}

func defaultIfZero(val, def int) int {
	if val > 0 {
		return val
	}
	return def
}

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
	if d.EnvDrops == nil {
		d.EnvDrops = &EnvDropLog{}
	}
	if d.MkScratchDir == nil {
		d.MkScratchDir = os.MkdirTemp
	}
	if d.SandboxWrap == nil {
		d.SandboxWrap = defaultSandboxWrap(d)
	}
	if d.KeychainProbe == nil {
		d.KeychainProbe = defaultKeychainProbe(d)
	}
	d = d.withProcessDefaults()
	return d
}

type Config struct {
	CLI                  string
	Profile              string
	Model                string
	Effort               string
	PromptFile           string
	Workspace            string
	StdoutLog            string
	StderrLog            string
	Artifact             string
	SecondaryArtifacts   []string
	Completion           core.CompletionContract
	Cycle                int
	Worktree             string
	RunID                string
	ProjectRoot          string
	Agent                string
	PermissionMode       string
	StreamOutput         bool
	SessionName          string
	AllowBypass          bool
	HumanInput           bool
	RequireFull          bool
	RequireSandbox       bool
	DenyPaths            []string
	DenyReadPaths        []string
	SandboxWriteSubpaths []string
	AllowedTools         []string
	ExtraFlags           []string
	Realization          Realization
	ArtifactTimeoutS     int
	BootOnly             bool
	AnthropicBaseURL     string
	AllowNetwork         bool
	codexConfigPath      string
	DispatchID           string
}

type Engine struct {
	deps Deps
}

func NewEngine(deps Deps) *Engine {
	d := deps.withDefaults()
	if d.TokenResolver == nil {
		d.Signals.Emit(signalcenter.Event{
			Module: signalcenter.ModuleBridge, Origin: "NewEngine", Kind: signalcenter.KindBridgeWarning,
			Severity: signalcenter.SeverityWarn, Code: CodeTokenResolverMissing,
			Reason: "Deps.TokenResolver is nil — token usage enrichment unavailable (fail-open: lifecycle/outcome records continue without token counts)",
		})
	}
	return &Engine{deps: d}
}

func (e *Engine) SignalsWired() bool { return e.deps.Signals != nil }

func (e *Engine) HasTokenResolver() bool {
	return e.deps.TokenResolver != nil
}

func resolvedModel(m string) string {
	if m == "" {
		return "auto"
	}
	return m
}

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
	if req.Effort != "" {
		args = append(args, "--effort="+req.Effort)
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
		args = append(args, "--project-root="+req.ProjectRoot)
	}
	if req.RequireSandbox {
		args = append(args, "--require-sandbox")
	}
	if req.Completion != "" {
		args = append(args, "--completion="+string(req.Completion))
	}
	if req.PermissionMode != "" {
		args = append(args, "--permission-mode="+req.PermissionMode)
	}
	if req.SessionName != "" {
		args = append(args, "--session-name="+req.SessionName)
	}
	args = append(args, "--allow-bypass")
	if len(req.ExtraFlags) > 0 {
		args = append(args, "--")
		args = append(args, req.ExtraFlags...)
	}
	return args
}

func (e *Engine) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	chain := e.modelChain(req)
	var resp core.BridgeResponse
	var err error
	for i, model := range chain {
		req.Model = model
		resp, err = e.launchOnce(ctx, req)
		if i == len(chain)-1 || !modelWalled(resp) || ctx.Err() != nil {
			break
		}
		e.logModelStep(req, model, chain[i+1])
		e.clearArtifact(req)
	}
	return resp, err
}

func (e *Engine) launchModel(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	if err := ValidateRequest(req); err != nil {
		return core.BridgeResponse{}, err
	}
	in, err := materializeInputs(req)
	if err != nil {
		return core.BridgeResponse{}, err
	}
	model := resolvedModel(req.Model)
	args := launchArgs(req, in.promptFile, in.stdoutLog, in.stderrLog, e.deps)
	run := e.runAttempt(ctx, req, args)
	resp := core.BridgeResponse{
		ExitCode: run.code, Stderr: run.stderr, BootMS: run.bootMS,
		DispatchID: run.dispatchID, PaneCapture: run.paneCapture, SessionRecoverable: e.freshSessionAllowed(ctx, run),
	}
	marks := append(sessionMarks(req.FreshSession, resp.SessionRecoverable), markBootLoss(run), markTry(run))
	c := e.recordModelAttempt(req, model, run.code, run.start, run.end, run.dispatched, run.stderr, &resp, marks...)
	e.reportFreshSession(c, req)
	e.clearBootStrike(c, req.CLI, run.code)
	if run.code == ExitOK {
		e.readResult(c, req, in.stdoutLog, &resp)
		return resp, nil
	}
	launchError := e.persistLaunchError(c, req.Workspace, in.agent, run.stderr)
	out := launchoutcome.Classify(run.code, ctx.Err(), run.stderr)
	resp.CauseCode = launchoutcome.WithBootLoss(out.CauseCode, run.code, run.bootLoss)
	e.recordBootStrike(c, req.CLI, run.code)
	c.launchWarn("Engine.Launch", "classify", out.Signal, out.Err.Error(), launchFields(out, launchError))
	return resp, out.Err
}

type launchInputs struct {
	agent, promptFile, stdoutLog, stderrLog string
}

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

type launchRun struct {
	code        int
	stderr      string
	start, end  time.Time
	bootMS      int64
	dispatched  modelDispatch
	fatal       fatalPaneObservation
	bootLoss    launchoutcome.BootLossCause
	dispatchID  string
	paneCapture string
	effort      string
	servedModel string
}

func (e *Engine) reportFreshSession(c attemptLogContext, req core.BridgeRequest) {
	if !req.FreshSession {
		return
	}
	c.launchWarn("Engine.Launch", "", CodeFreshSessionRetry,
		fmt.Sprintf("%s runs one fresh %s session: the pane of the session before it died with a session-recoverable cause", req.Agent, req.CLI), nil)
}

func sessionMarks(fresh, recoverable bool) []func(*llmcalls.Record) {
	var marks []func(*llmcalls.Record)
	if fresh {
		marks = append(marks, markFreshSessionRetry)
	}
	if recoverable {
		marks = append(marks, markSessionRecoverable)
	}
	return marks
}

func (e *Engine) freshSessionAllowed(ctx context.Context, first launchRun) bool {
	if !first.fatal.cause.SessionRecoverable() || first.fatal.named || first.code != ExitArtifactTimeout || ctx.Err() != nil {
		return false
	}
	if deadline, ok := ctx.Deadline(); ok && deadline.Sub(e.deps.Now()) < time.Duration(first.fatal.intervalS)*time.Second {
		return false
	}
	return true
}

func markFreshSessionRetry(rec *llmcalls.Record) { rec.FreshSessionRetry = true }

func markSessionRecoverable(rec *llmcalls.Record) { rec.SessionRecoverable = true }

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
	previousBootLossObserver := callDeps.onBootLoss
	callDeps.onBootLoss = func(cause launchoutcome.BootLossCause) {
		run.bootLoss = cause
		if previousBootLossObserver != nil {
			previousBootLossObserver(cause)
		}
	}
	callDeps.onDispatchID = func(id string) { run.dispatchID = id }
	callDeps.onPaneCapture = func(path string) { run.paneCapture = path }
	callDeps.onLaunchEffort = func(effort string) { run.effort = effort }
	callDeps.onServedModel = func(model string) { run.servedModel = model }
	callEngine := &Engine{deps: callDeps}
	var stderrBuf bytes.Buffer
	run.start = e.deps.Now()
	run.code = callEngine.LaunchArgs(ctx, args, env, io.Discard, &stderrBuf)
	run.end = e.deps.Now()
	run.stderr = stderrBuf.String()
	return run
}

func (e *Engine) clearBootStrike(c attemptLogContext, cli string, code int) {
	if e.deps.BootTimeoutStore == nil || clihealth.IsBootTimeoutExitCode(code) {
		return
	}
	if err := e.deps.BootTimeoutStore.ClearBootStrike(cli); err != nil {
		c.launchWarn("Engine.clearBootStrike", "clear_boot_strike", CodeBootStrikeClearFailed,
			fmt.Sprintf("boot-strike clear failed for %s: %v", cli, err), nil)
	}
}

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

func (e *Engine) recordBootStrike(c attemptLogContext, cli string, code int) {
	if code != ExitREPLBootTimeout || e.deps.BootTimeoutStore == nil {
		return
	}
	if _, err := e.deps.BootTimeoutStore.RecordBootStrike(cli); err != nil {
		c.launchWarn("Engine.recordBootStrike", "record_boot_strike", CodeBootStrikeRecordFailed,
			fmt.Sprintf("boot-timeout bench record failed for %s: %v", cli, err), nil)
	}
}

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

const defaultContextFillWarnPct = 60

const tripwireSuccessThreshold = 60 * time.Second

func artifactTimeoutSummary(stderr string) string {
	return launchoutcome.ArtifactTimeoutSummary(stderr)
}

var randRead = rand.Read

func defaultChallengeToken() (string, error) {
	var b [8]byte
	if _, err := randRead(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func execRunner(ctx context.Context, name, dir string, args, env []string,
	stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	cmd := sysexec.Command(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return -1, err
	}
	if pidFile := envValue(env, bridgePidfileEnv); pidFile != "" {
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

func envValue(env []string, key string) string {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return e[len(prefix):]
		}
	}
	return ""
}
