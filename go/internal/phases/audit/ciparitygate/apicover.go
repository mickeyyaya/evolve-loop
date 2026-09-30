package ciparitygate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	underivableEnforce    = "changed-package set is underivable this cycle (git diff failed) — apicover -enforce gate cannot verify coverage; treat as FAIL, do not ship"
	underivableGraduation = "changed-package set is underivable this cycle (git diff failed) — apicover graduation gate cannot verify new packages; treat as FAIL, do not ship"
)

type enforceInputs struct {
	dir     string
	changed []string
	enforce []byte
}

func (g *Gates) inputs(at gateOrigin, req Request, underivable string) (in enforceInputs, offenders []string, done bool) {
	dir := moduleDir(req.root())
	if dir == "" {
		return enforceInputs{}, nil, true
	}
	changed, derivable := g.changedSet(req.root(), req.Cycle)
	enforceBytes, err := os.ReadFile(filepath.Join(dir, ".apicover-enforce"))
	if err != nil {
		return enforceInputs{}, nil, true
	}
	if !derivable {
		return enforceInputs{}, g.failed(at, req, causeUnderivable, []string{underivable}), true
	}
	return enforceInputs{dir: dir, changed: changed, enforce: enforceBytes}, nil, false
}

func (g *Gates) ApicoverEnforce(req Request) ([]string, error) {
	in, offenders, done := g.inputs(gateEnforce, req, underivableEnforce)
	if done {
		return offenders, nil
	}
	touched := ciparity.IntersectEnforced(in.changed, in.enforce)
	if len(touched) == 0 {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), g.timeouts.Apicover)
	defer cancel()
	funcPath, dirs, cleanup, err := g.coverageProfile(ctx, req, in.dir, touched)
	defer cleanup()
	if err != nil {
		return nil, err
	}
	if len(dirs) == 0 {
		return nil, nil
	}
	var report bytes.Buffer
	code, runErr := apicover.Run(ctx, apicover.Config{Enforce: true, CoverPath: funcPath, Dirs: dirs}, &report)
	offenders, err = enforceVerdict(code, runErr, report.String())
	if err != nil {
		g.stepFailed(gateEnforce, req, stepMeasure, err.Error(), runErr)
		return nil, err
	}
	if len(offenders) == 0 {
		return nil, nil
	}
	return g.failed(gateEnforce, req, causeApicover, offenders), nil
}

func (g *Gates) coverageProfile(ctx context.Context, req Request, dir string, touched []string) (funcPath string, dirs []string, cleanup func(), err error) {
	var scratch []string
	cleanup = func() {
		for _, p := range scratch {
			_ = os.Remove(p)
		}
	}
	fail := func(step gateStep, err error, cmd string) (string, []string, func(), error) {
		g.stepFailed(gateEnforce, req, step, err.Error(), err, "cmd", cmd)
		return "", nil, cleanup, err
	}
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fail(stepBinDir, fmt.Errorf("apicover gate: ensure bin dir: %w", err), "")
	}
	covPath := filepath.Join(binDir, "ciparity-cover.txt")
	scratch = append(scratch, covPath)
	testArgs := ciparity.CoverageTestArgs(covPath, touched)
	run := scrubbedRun(g.run)
	if _, err := sysexec.Output(ctx, run, dir, "go", testArgs...); err != nil {
		return fail(stepCoverRun, fmt.Errorf("apicover gate: scoped coverage run: %w", err), "go "+strings.Join(testArgs, " "))
	}
	funcPath = covPath + ".func.txt"
	scratch = append(scratch, funcPath)
	funcOut, err := sysexec.Output(ctx, run, dir, "go", "tool", "cover", "-func="+covPath)
	if err != nil {
		return fail(stepCoverFunc, fmt.Errorf("apicover gate: cover -func: %w", err), "go tool cover -func="+covPath)
	}
	if werr := os.WriteFile(funcPath, []byte(funcOut+"\n"), 0o644); werr != nil {
		return fail(stepWriteFuncCover, fmt.Errorf("apicover gate: write func cover: %w", werr), "")
	}
	listArgs := append([]string{"list", "-e", "-f", "{{.Dir}}"}, touched...)
	dirsOut, err := sysexec.Output(ctx, run, dir, "go", listArgs...)
	if err != nil {
		return fail(stepPkgDirs, fmt.Errorf("apicover gate: go list: %w", err), "go "+strings.Join(listArgs, " "))
	}
	return funcPath, strings.Fields(dirsOut), cleanup, nil
}

func enforceVerdict(code int, runErr error, report string) ([]string, error) {
	if code == 0 && runErr == nil {
		return nil, nil
	}
	if runErr != nil && (errors.Is(runErr, context.DeadlineExceeded) || errors.Is(runErr, context.Canceled)) {
		return nil, fmt.Errorf("apicover gate: measurement interrupted: %w", runErr)
	}
	detail := strings.TrimSpace(report)
	if runErr != nil {
		detail = strings.TrimSpace(detail + "\napicover -enforce measurement error: " + runErr.Error())
	}
	return offenderLines(detail), nil
}
