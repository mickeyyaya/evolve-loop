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

// The two apicover gates' hard-FAIL sentences for an underivable change set
// on a cycle WITH an enforce list: git failed (no repo, bad baseRef, fleet
// .git/index.lock race), so neither gate can prove its set is empty — FAIL
// loud (err == nil) instead of the silent (nil, nil) no-op that shipped an
// uncovered export (cycle-581 D1 / D2). Deliberately NOT the three whole-repo
// gates' WARN (the severity asymmetry is the cycle-581 decision).
const (
	underivableEnforce    = "changed-package set is underivable this cycle (git diff failed) — apicover -enforce gate cannot verify coverage; treat as FAIL, do not ship"
	underivableGraduation = "changed-package set is underivable this cycle (git diff failed) — apicover graduation gate cannot verify new packages; treat as FAIL, do not ship"
)

// enforceInputs is what both apicover gates resolve before deciding.
type enforceInputs struct {
	dir     string
	changed []string
	enforce []byte
}

// inputs is the ONE prologue the two apicover gates share (it was copy-pasted
// once per gate): the module dir (no module → done), the change set, the
// enforce list (absent → done, read BEFORE the derivable check), and the
// underivable hard-FAIL — the caller's own sentence, one GATE_FAILED
// cause=underivable (never a CHANGESET_UNDERIVABLE: one fact, one event).
func (g *Gates) inputs(at gateOrigin, req Request, underivable string) (in enforceInputs, offenders []string, done bool) {
	dir := moduleDir(req.root())
	if dir == "" {
		return enforceInputs{}, nil, true
	}
	changed, derivable := g.changedSet(req.root(), req.Cycle)
	enforceBytes, err := os.ReadFile(filepath.Join(dir, ".apicover-enforce"))
	if err != nil {
		return enforceInputs{}, nil, true // no enforce list → nothing to enforce / graduate against
	}
	if !derivable {
		return enforceInputs{}, g.failed(at, req, causeUnderivable, []string{underivable}), true
	}
	return enforceInputs{dir: dir, changed: changed, enforce: enforceBytes}, nil, false
}

// ApicoverEnforce runs `apicover -enforce` (CI go.yml "api-coverage enforce"
// step) over the enforced packages this cycle actually touched — the
// AST-level UNCOVERED (unnamed-export) check that repeatedly broke main.
// Scoped to the touched∩enforced set (O(change)); a no-op when the cycle
// touched no enforced package. FALSE-GREEN (coverage-dependent) is left to
// CI, matching the acs/regression/apicover completeness/correctness split.
// The scoped coverage profile is forked (coverageProfile), the enforce gate
// runs IN-PROCESS over just those dirs — the same pipeline as go.yml, scoped,
// folded into the evolve binary (one-binary S1): no runtime `go build -o
// bin/apicover`. The gate ctx bounds the measurement itself.
func (g *Gates) ApicoverEnforce(req Request) ([]string, error) {
	in, offenders, done := g.inputs(gateEnforce, req, underivableEnforce)
	if done {
		return offenders, nil
	}
	touched := ciparity.IntersectEnforced(in.changed, in.enforce)
	if len(touched) == 0 {
		return nil, nil // cycle touched no enforced package
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
		return nil, nil // clean
	}
	return g.failed(gateEnforce, req, causeApicover, offenders), nil // offenders or measurement error → FAIL
}

// coverageProfile forks the three toolchain pre-steps: the scoped coverage
// run (tag-parity through the ciparity SSOT — an untagged run under-reports a
// tag-gated package by up to 43 points), `go tool cover -func`, and the
// package-dir listing. Each failure is fail-open with GATE_STEP_FAILED naming
// the step. The scratch files live under the worktree's bin/ (ensured to
// exist — the deleted build used to create it as a side effect) and cleanup
// removes them best-effort.
func (g *Gates) coverageProfile(ctx context.Context, req Request, dir string, touched []string) (funcPath string, dirs []string, cleanup func(), err error) {
	var scratch []string
	cleanup = func() {
		for _, p := range scratch {
			_ = os.Remove(p) // scratch profile — don't accumulate on a persistent worktree
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
	if _, err := sysexec.Output(ctx, g.run, dir, "go", testArgs...); err != nil {
		return fail(stepCoverRun, fmt.Errorf("apicover gate: scoped coverage run: %w", err), "go "+strings.Join(testArgs, " "))
	}
	funcPath = covPath + ".func.txt"
	scratch = append(scratch, funcPath)
	funcOut, err := sysexec.Output(ctx, g.run, dir, "go", "tool", "cover", "-func="+covPath)
	if err != nil {
		return fail(stepCoverFunc, fmt.Errorf("apicover gate: cover -func: %w", err), "go tool cover -func="+covPath)
	}
	if werr := os.WriteFile(funcPath, []byte(funcOut+"\n"), 0o644); werr != nil {
		return fail(stepWriteFuncCover, fmt.Errorf("apicover gate: write func cover: %w", werr), "")
	}
	listArgs := append([]string{"list", "-e", "-f", "{{.Dir}}"}, touched...)
	dirsOut, err := sysexec.Output(ctx, g.run, dir, "go", listArgs...)
	if err != nil {
		return fail(stepPkgDirs, fmt.Errorf("apicover gate: go list: %w", err), "go "+strings.Join(listArgs, " "))
	}
	return funcPath, strings.Fields(dirsOut), cleanup, nil
}

// enforceVerdict is the PURE exit-code contract of the in-process
// apicover.Run: 0 clean; 1 offenders → FAIL; 2 (with a non-nil error) a
// measurement failure → also FAIL. In-process there is NO exec-start failure
// mode, so a measurement error is a real finding about the touched code — an
// unparseable enforced package — folded into the offender report (cf. the
// underivable hard-FAIL). A ctx-deadline/cancel interruption is INFRA weather,
// not a finding: it surfaces as an error so the gate fails OPEN (WARN).
func enforceVerdict(code int, runErr error, report string) ([]string, error) {
	if code == 0 && runErr == nil {
		return nil, nil // clean
	}
	if runErr != nil && (errors.Is(runErr, context.DeadlineExceeded) || errors.Is(runErr, context.Canceled)) {
		return nil, fmt.Errorf("apicover gate: measurement interrupted: %w", runErr)
	}
	detail := strings.TrimSpace(report)
	if runErr != nil {
		detail = strings.TrimSpace(detail + "\napicover -enforce measurement error: " + runErr.Error())
	}
	return offenderLines(detail), nil // offenders or measurement error → FAIL
}
