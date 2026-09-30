package ciparitygate

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func (g *Gates) GoVet(req Request) ([]string, error) {
	if _, run, err := g.scope(gateGoVet, req); err != nil || !run {
		return nil, err
	}
	return g.runGate(gateGoVet, req, "go vet ./...", g.timeouts.GoVet, "go", "vet", "./...")
}

func (g *Gates) ACSDurable(req Request) ([]string, error) {
	if _, run, err := g.scope(gateACSDurable, req); err != nil || !run {
		return nil, err
	}
	return g.runGate(gateACSDurable, req, "acs-durable (-tags acs)", g.timeouts.ACSDurable,
		"go", "test", "-count=1", "-tags", "acs", "./acs/regression/...")
}

func (g *Gates) runGate(at gateOrigin, req Request, label string, timeout time.Duration, name string, args ...string) ([]string, error) {
	dir := moduleDir(req.root())
	if dir == "" {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, errOut, code, err := sysexec.Capture(ctx, g.run, dir, name, args...)
	if err != nil {
		wrapped := fmt.Errorf("%s gate could not run: %w", label, err)
		g.stepFailed(at, req, stepExec, wrapped.Error(), err, "cmd", name+" "+strings.Join(args, " "))
		return nil, wrapped
	}
	if code == 0 {
		return nil, nil
	}
	combined := strings.TrimSpace(out + "\n" + errOut)
	if combined == "" {
		combined = fmt.Sprintf("%s exited %d (no output)", name, code)
	}
	return g.failed(at, req, causeExit, offenderLines(combined), "exit", strconv.Itoa(code)), nil
}
