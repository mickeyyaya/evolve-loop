// Package ship implements the script-driven commit-and-push phase as a
// core.PhaseRunner. Unlike the LLM phases, ship does not call a Bridge;
// it shells to scripts/lifecycle/ship.sh — the canonical atomic shipper
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
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const (
	phaseName         = string(core.PhaseShip)
	defaultShipScript = "scripts/lifecycle/ship.sh"
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

func (p *Phase) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	start := p.nowFn()
	if p.runner == nil {
		return core.PhaseResponse{}, fmt.Errorf("ship: runner required")
	}
	msg := req.Context["commit_message"]
	if msg == "" {
		err := errors.New("ship: commit_message required in Context")
		return core.PhaseResponse{
			Phase:        phaseName,
			Verdict:      core.VerdictFAIL,
			ArtifactsDir: req.Workspace,
			DurationMS:   p.nowFn().Sub(start).Milliseconds(),
			Diagnostics:  []core.Diagnostic{{Severity: "error", Message: err.Error()}},
		}, err
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
