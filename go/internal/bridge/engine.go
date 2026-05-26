package bridge

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
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
type CmdRunner func(ctx context.Context, name string, args, env []string,
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
	return d
}

// Config is the fully-resolved launch configuration: flags, env, and
// profile merged down to concrete values (e.g. Model already has "auto"
// resolved against the profile). The Engine populates it once and hands
// it to the selected Driver, so drivers never re-parse flags or re-read
// the profile. Field set mirrors the bin/bridge launch flag surface.
type Config struct {
	CLI            string
	Profile        string
	Model          string // effective model — "auto" already resolved
	PromptFile     string
	Workspace      string
	StdoutLog      string
	StderrLog      string
	Artifact       string
	Cycle          int
	Worktree       string
	Agent          string
	PermissionMode string // "" = driver default
	StreamOutput   bool
	SessionName    string
	AllowBypass    bool
	HumanInput     bool
	RequireFull    bool
	AllowedTools   []string // from profile.allowed_tools
	ExtraFlags     []string // forwarded to the inner CLI after `--`
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

// errNotImplemented marks the M0 scaffold stubs. M2–M6 replace each stub
// with the ported logic; the error never reaches production because the
// EVOLVE_BRIDGE_GO cutover (M7) only flips on once the stubs are gone.
var errNotImplemented = errors.New("bridge: not implemented (M0 scaffold)")

// Launch satisfies core.Bridge. M0 stub — the real pipeline lands in
// M2 (engine flow) + M3–M5 (drivers).
func (e *Engine) Launch(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	_ = ctx
	_ = req
	return core.BridgeResponse{}, errNotImplemented
}

// Probe satisfies core.Bridge. M0 stub — real CLI/tier detection lands
// in M6 (probe/ subpackage).
func (e *Engine) Probe(ctx context.Context) (core.BridgeProbe, error) {
	_ = ctx
	return core.BridgeProbe{}, errNotImplemented
}

// EnabledFromEnv reports whether the in-process Go bridge should be used
// instead of the bash subprocess. Reads EVOLVE_BRIDGE_GO from the
// request-local overlay first, then the process env. Default off — the
// adapter keeps shelling to bash until M7 flips the default.
func EnabledFromEnv(env map[string]string) bool {
	v, ok := env["EVOLVE_BRIDGE_GO"]
	if !ok || v == "" {
		v = os.Getenv("EVOLVE_BRIDGE_GO")
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// defaultChallengeToken mints 8 random bytes as hex (16 chars), matching
// `openssl rand -hex 8` from the bash dry-run path.
func defaultChallengeToken() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// execRunner is the production CmdRunner — wraps exec.CommandContext and
// maps a process exit code to (code, nil), reserving err for
// unrecoverable failures. Ported verbatim from the adapter so behavior
// is identical across the cutover.
func execRunner(ctx context.Context, name string, args, env []string,
	stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}
	return 0, nil
}
