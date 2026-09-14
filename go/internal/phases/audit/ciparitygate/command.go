package ciparitygate

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// GoVet runs `go vet ./...` (CI go.yml "vet + fmt" step / `make lint`) over
// the whole worktree module — catches import cycles and other vet-level
// defects a scoped build misses. No-op unless the cycle built Go; WARN (not a
// silent skip) when the change set is underivable.
func (g *Gates) GoVet(req Request) ([]string, error) {
	if _, run, err := g.scope(gateGoVet, req); err != nil || !run {
		return nil, err
	}
	return g.runGate(gateGoVet, req, "go vet ./...", g.timeouts.GoVet, "go", "vet", "./...")
}

// ACSDurable runs the durable ACS regression suite with -tags acs (CI ci.yml
// acs-durable gate / `make test-acs-durable`) — catches flagregistry /
// flag-ceiling / skills-drift regressions invisible without the acs build
// tag. No-op unless the cycle built Go; WARN when the change set is underivable.
func (g *Gates) ACSDurable(req Request) ([]string, error) {
	if _, run, err := g.scope(gateACSDurable, req); err != nil || !run {
		return nil, err
	}
	return g.runGate(gateACSDurable, req, "acs-durable (-tags acs)", g.timeouts.ACSDurable,
		"go", "test", "-count=1", "-tags", "acs", "./acs/regression/...")
}

// runGate runs one CI command in the cycle's go/ dir and maps the result to
// the hook contract via the EXIT CODE: an exec-start failure (binary not
// found, context cancelled) → error → fail-open WARN + GATE_STEP_FAILED
// step=exec; ANY non-zero exit → offenders → FAIL + GATE_FAILED cause=exit (a
// synthesized line covers the rare no-output case); exit 0 → clean. Capture
// (NOT CombinedOutput): the runner maps a non-zero process EXIT to (code,
// nil), reserving err for unrecoverable start failures, so only the exit code
// distinguishes "the tool ran and found problems" from "the gate could not
// run"; stdout AND stderr are captured — go vet writes its diagnostics to stderr.
func (g *Gates) runGate(at gateOrigin, req Request, label string, timeout time.Duration, name string, args ...string) ([]string, error) {
	dir := moduleDir(req.root())
	if dir == "" {
		return nil, nil // no go module in the worktree → nothing to check
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, errOut, code, err := sysexec.Capture(ctx, g.run, dir, name, args...)
	if err != nil {
		wrapped := fmt.Errorf("%s gate could not run: %w", label, err) // fail-open → WARN
		g.stepFailed(at, req, stepExec, wrapped.Error(), err, "cmd", name+" "+strings.Join(args, " "))
		return nil, wrapped
	}
	if code == 0 {
		return nil, nil // clean
	}
	combined := strings.TrimSpace(out + "\n" + errOut)
	if combined == "" {
		combined = fmt.Sprintf("%s exited %d (no output)", name, code)
	}
	return g.failed(at, req, causeExit, offenderLines(combined), "exit", strconv.Itoa(code)), nil // ran + non-zero exit → FAIL
}
