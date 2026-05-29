// Package ship implements the script-driven commit-and-push phase as a
// core.PhaseRunner. Unlike the LLM phases, ship does not call a Bridge;
// it shells to legacy/scripts/lifecycle/ship.sh — the canonical atomic shipper
// the v8.13.0 ship-gate hook allowlists for git commit / git push /
// gh release create operations.
//
// EVOLVE_SHIP_SCRIPT env override points at an alternate script (used
// by tests and by Phase 4 cutover when the Go-native ship lands).
package ship

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const (
	phaseName         = string(core.PhaseShip)
	defaultShipScript = "legacy/scripts/lifecycle/ship.sh"
)

// CmdRunner is the subprocess injection seam. Tests override it; the
// production wiring uses execRunner.
type CmdRunner func(ctx context.Context, name string, args, env []string, cwd string,
	stdin io.Reader, stdout, stderr io.Writer) (exitCode int, err error)

type Config struct {
	Runner CmdRunner
	NowFn  func() time.Time
}

// Phase implements core.PhaseRunner for the ship stage.
type Phase struct {
	runner CmdRunner
	nowFn  func() time.Time
}

func New(c Config) *Phase {
	nowFn := c.NowFn
	if nowFn == nil {
		nowFn = time.Now
	}
	return &Phase{runner: c.Runner, nowFn: nowFn}
}

func (p *Phase) Name() string { return phaseName }

// defaultCommitMessage synthesizes a deterministic cycle commit message when
// the caller didn't supply Context["commit_message"]. Mirrors the shape
// `evolve cycle run` uses (cmd_cycle.go), plus the cycle number for traceable
// git history.
func defaultCommitMessage(req core.PhaseRequest) string {
	if req.GoalHash != "" {
		return fmt.Sprintf("evolve-cycle %d: goal=%s", req.Cycle, req.GoalHash)
	}
	return fmt.Sprintf("evolve-cycle %d", req.Cycle)
}

func (p *Phase) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	start := p.nowFn()
	if p.runner == nil {
		return core.PhaseResponse{}, fmt.Errorf("ship: runner required")
	}
	// An explicit Context["commit_message"] always wins. When absent, synthesize
	// a deterministic message from the cycle identity rather than failing the
	// ship (single seam, cycle-147 lesson): `evolve cycle run` populates this
	// Context key but the autonomous-loop construction path (cmd_loop.go
	// buildCycleContext) does not, so erroring here silently sank EVERY loop
	// cycle at the ship step once the audit gate finally let cycles reach ship
	// (cycle-150). Defaulting here covers all current and future callers; the
	// manual `evolve ship` CLI always passes an explicit message and is
	// unaffected.
	msg := req.Context["commit_message"]
	if msg == "" {
		msg = defaultCommitMessage(req)
	}

	// v11.3.0: dispatch to native Go ship by default. EVOLVE_NATIVE_SHIP=0
	// reverts to the legacy bash shell-out (rollback path).
	if useNativeShip(req.Env) {
		return p.runNative(ctx, req, msg, start)
	}

	script := req.Env["EVOLVE_SHIP_SCRIPT"]
	if script == "" {
		script = filepath.Join(req.ProjectRoot, defaultShipScript)
	}

	args := []string{"--class", "cycle", msg}
	env := os.Environ()
	for k, v := range req.Env {
		env = append(env, k+"="+v)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	exitCode, runErr := p.runner(ctx, script, args, env, req.ProjectRoot, nil, &stdoutBuf, &stderrBuf)
	durationMS := p.nowFn().Sub(start).Milliseconds()

	if runErr != nil {
		return core.PhaseResponse{
			Phase:        phaseName,
			Verdict:      core.VerdictFAIL,
			ArtifactsDir: req.Workspace,
			DurationMS:   durationMS,
			Diagnostics: []core.Diagnostic{
				{Severity: "error", Message: fmt.Sprintf("ship.sh run error: %s", runErr.Error())},
			},
		}, fmt.Errorf("ship: run: %w", runErr)
	}

	if exitCode != 0 {
		return core.PhaseResponse{
			Phase:        phaseName,
			Verdict:      core.VerdictFAIL,
			ArtifactsDir: req.Workspace,
			DurationMS:   durationMS,
			Diagnostics: []core.Diagnostic{
				{Severity: "error", Message: stderrBuf.String()},
			},
		}, fmt.Errorf("ship: exit=%d", exitCode)
	}

	return core.PhaseResponse{
		Phase:        phaseName,
		Verdict:      core.VerdictPASS,
		ArtifactsDir: req.Workspace,
		NextPhase:    string(core.PhaseRetro),
		DurationMS:   durationMS,
	}, nil
}

// execRunner is the production CmdRunner.
func execRunner(ctx context.Context, name string, args, env []string, cwd string,
	stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	cmd.Dir = cwd
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

// NewWithDefaultRunner is a convenience constructor for production
// wiring that uses exec.CommandContext.
func NewWithDefaultRunner() *Phase {
	return New(Config{Runner: execRunner})
}

// useNativeShip returns true when the native Go path should be used.
// Default in v11.3.0: native. Override with EVOLVE_NATIVE_SHIP=0 to fall
// back to the legacy bash shell-out (rollback path through v11.x).
// EVOLVE_NATIVE_SHIP=1 (or unset/anything-else) → native.
func useNativeShip(env map[string]string) bool {
	if v, ok := env["EVOLVE_NATIVE_SHIP"]; ok {
		return v != "0"
	}
	if v := os.Getenv("EVOLVE_NATIVE_SHIP"); v == "0" {
		return false
	}
	return true
}

// runNative dispatches to the native Go ship implementation. Translates
// PhaseRequest → Options, then RunResult → PhaseResponse.
func (p *Phase) runNative(ctx context.Context, req core.PhaseRequest, msg string, start time.Time) (core.PhaseResponse, error) {
	opts := Options{
		Class:         ClassCycle, // PhaseRunner only invokes for cycle commits
		CommitMessage: msg,
		ProjectRoot:   req.ProjectRoot,
		PluginRoot:    req.Env["EVOLVE_PLUGIN_ROOT"],
		Env:           req.Env,
		Runner:        execRunner,
	}
	res, err := Run(ctx, opts)
	durationMS := p.nowFn().Sub(start).Milliseconds()

	if err != nil {
		verdict := core.VerdictFAIL
		return core.PhaseResponse{
			Phase:        phaseName,
			Verdict:      verdict,
			ArtifactsDir: req.Workspace,
			DurationMS:   durationMS,
			Diagnostics: []core.Diagnostic{
				{Severity: "error", Message: err.Error()},
			},
		}, fmt.Errorf("ship: native: %w", err)
	}
	if res.ExitCode != ExitOK {
		return core.PhaseResponse{
			Phase:        phaseName,
			Verdict:      core.VerdictFAIL,
			ArtifactsDir: req.Workspace,
			DurationMS:   durationMS,
			Diagnostics: []core.Diagnostic{
				{Severity: "error", Message: strings.Join(res.Logs, "\n")},
			},
		}, fmt.Errorf("ship: native exit=%d", res.ExitCode)
	}
	return core.PhaseResponse{
		Phase:        phaseName,
		Verdict:      core.VerdictPASS,
		ArtifactsDir: req.Workspace,
		NextPhase:    string(core.PhaseRetro),
		DurationMS:   durationMS,
	}, nil
}
