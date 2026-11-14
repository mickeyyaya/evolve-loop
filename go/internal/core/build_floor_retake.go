package core

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeBuildFloorRetakeGreen  signalcenter.Code = "BUILD_FLOOR_RETAKE_GREEN"
	CodeBuildFloorRetakeRed    signalcenter.Code = "BUILD_FLOOR_RETAKE_RED"
	CodeBuildFloorFlakyNewTest signalcenter.Code = "BUILD_FLOOR_FLAKY_NEW_TEST"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleBuild, CodeBuildFloorRetakeGreen, "a build floor test run was RED once and GREEN on its one serialized retake under the integration-tier lock, so the build goes on; fields.pkg (the package or the run), tests (the failed top-level test names of the first run; evolve ci classify also keys a failure on its package and top-level test), retake=green")
	signalcenter.RegisterCode(signalcenter.ModuleBuild, CodeBuildFloorRetakeRed, "a build floor test run was RED and its one serialized retake under the integration-tier lock was RED again, so the floor fails; fields.pkg (the package or the run), tests (the failed top-level test names of the retake; evolve ci classify also keys a failure on its package and top-level test), retake=red")
	signalcenter.RegisterCode(signalcenter.ModuleBuild, CodeBuildFloorFlakyNewTest, "a build floor test run was RED on a package where the cycle added a test file, and its serialized retake was GREEN; the floor fails the run as the ship added-test backstop does; fields.pkg (the new-test packages that were red), tests (the failed top-level test names of the first run), retake=green, class=flaky_new_test")
}

func floorScopedPackages(pkgs []string, warn io.Writer) []string {
	scoped, envExclusive, _ := ciparity.TierScope(pkgs)
	noteEnvExclusive(envExclusive, warn)
	return scoped
}

func withoutEnvExclusive(pkgs []string, warn io.Writer) []string {
	var kept, skipped []string
	for _, p := range pkgs {
		if ciparity.EnvExclusivePkg(p) {
			skipped = append(skipped, p)
			continue
		}
		kept = append(kept, p)
	}
	noteEnvExclusive(skipped, warn)
	return kept
}

func noteEnvExclusive(skipped []string, warn io.Writer) {
	if len(skipped) > 0 {
		fmt.Fprintf(warn, "[build-floor] NOTE: env-exclusive package(s) %s are not run at the floor under a live loop, as at the audit tier. Backstop: %s\n",
			strings.Join(skipped, ", "), ciparity.EnvExclusiveBackstopNote(skipped))
	}
}

type floorRetakeOutcome int

const (
	floorRetakeGreen floorRetakeOutcome = iota
	floorRetakeRed
	floorRetakeUnrunnable
	floorRetakeNewTestRed
)

func floorTestPackages(paths []string, warn io.Writer) []string {
	return floorScopedPackages(changedGoTestPackages(paths), warn)
}

func (in ReviewInput) retakeFloorRun(label, first string, covered []string, rerun func() (string, bool)) (string, bool) {
	out, outcome := in.retakeFloorOutcome(label, first, covered, func() (string, floorRetakeOutcome) {
		out, ok := rerun()
		if ok {
			return out, floorRetakeGreen
		}
		return out, floorRetakeRed
	})
	return out, outcome == floorRetakeGreen
}

func (in ReviewInput) retakeFloorOutcome(label, first string, covered []string, rerun func() (string, floorRetakeOutcome)) (string, floorRetakeOutcome) {
	release := in.acquireFloorRetakeLock()
	out, outcome := rerun()
	release()
	switch outcome {
	case floorRetakeGreen:
		if reds := in.added.newTestReds(first, covered); len(reds) > 0 {
			in.emitFloorRetake(redPackages(reds), redTests(reds), floorRetakeNewTestShape)
			return attachCorrection(floorRetakeNewTestShape.correction(reds), first), floorRetakeNewTestRed
		}
		_, tests := goTestFailureLines(first)
		in.emitFloorRetakeGreen(label, tests)
	case floorRetakeRed:
		_, tests := goTestFailureLines(out)
		in.emitFloorRetakeRed(label, tests)
	default:
		fmt.Fprintf(in.errOut(), "[build-floor] WARN: the retake of %s could not run (%s), so the first red stands\n", label, strings.TrimSpace(out))
	}
	return out, outcome
}

func (in ReviewInput) retakeSelfCheckFailures(fails []selfCheckFailure, rerun func(pkg string) (string, bool)) []selfCheckFailure {
	var still []selfCheckFailure
	for _, f := range fails {
		if out, ok := in.retakeFloorRun(f.Pkg, f.Output, []string{f.Pkg}, func() (string, bool) { return rerun(f.Pkg) }); !ok {
			still = append(still, selfCheckFailure{Pkg: f.Pkg, Output: out})
		}
	}
	return still
}

func (in ReviewInput) acquireFloorRetakeLock() func() {
	release, err := floorRetakeLock(cmp.Or(in.ProjectRoot, in.Worktree))
	if err == nil {
		return release
	}
	fmt.Fprintf(in.errOut(), "[build-floor] WARN: the integration-tier lock is unavailable (%v), so the retake runs unserialized\n", err)
	return func() {}
}

var floorRetakeLockWait = ciparity.TierLockWait

func floorRetakeLock(root string) (func(), error) {
	if root == "" {
		return nil, errors.New("no project root")
	}
	return flock.LockWithin(ciparity.TierLockPath(root), floorRetakeLockWait, func() {})
}

type floorRetakeShape struct {
	code    signalcenter.Code
	verdict string
	kind    signalcenter.Kind
	class   string
	advice  string
}

var (
	floorRetakeGreenShape = floorRetakeShape{CodeBuildFloorRetakeGreen, "green", signalcenter.KindGatePassed, "", ""}
	floorRetakeRedShape   = floorRetakeShape{CodeBuildFloorRetakeRed, "red", signalcenter.KindGateRejected, "", ""}
)

var floorRetakeNewTestShape = floorRetakeShape{
	CodeBuildFloorFlakyNewTest, "green", signalcenter.KindGateRejected, "flaky_new_test",
	"A test that this cycle added gets no retake: the ship added-test backstop (phases/ship/repocontract.go) runs its package once and refuses a red. Make the test deterministic.",
}

func (s floorRetakeShape) correction(reds []baseattr.Key) string {
	return fmt.Sprintf("%s was red on the first run and green on the retake, but the cycle added a test to its package. %s", baseattr.Names(reds), s.advice)
}

func (in ReviewInput) emitFloorRetakeGreen(label string, tests []string) {
	in.emitFloorRetake(label, tests, floorRetakeGreenShape)
}

func (in ReviewInput) emitFloorRetakeRed(label string, tests []string) {
	in.emitFloorRetake(label, tests, floorRetakeRedShape)
}

func (in ReviewInput) emitFloorRetake(label string, tests []string, shape floorRetakeShape) {
	names := strings.Join(tests, ",")
	reason := fmt.Sprintf("build floor: %s was RED, and its serialized retake is %s; failed test(s): %s", label, strings.ToUpper(shape.verdict), cmp.Or(names, "none named in the output"))
	fmt.Fprintf(in.errOut(), "[build-floor] %s %s\n", shape.code, reason)
	fields := map[string]string{"pkg": label, "tests": names, "retake": shape.verdict}
	if shape.class != "" {
		fields["class"] = shape.class
	}
	in.Signals.Emit(signalcenter.Event{
		Cycle: in.Cycle, RunID: in.RunID, Phase: string(PhaseBuild),
		Module: signalcenter.ModuleBuild, Origin: "ReviewInput.emitFloorRetake", Kind: shape.kind,
		Severity: signalcenter.SeverityWarn, Code: shape.code, Reason: reason, Fields: fields,
	})
}

func goTestFailureLines(output string) (lines, tests []string) {
	for _, ln := range strings.Split(output, "\n") {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "--- FAIL: ") {
			lines = append(lines, ln)
		} else if f := strings.Fields(ln); len(f) >= 2 && f[0] == "FAIL" {
			lines = append(lines, ln)
		}
	}
	return lines, baseattr.FailedTests(output)
}

func coverRetakeResult(first, out string, status int) (string, int) {
	if status == coverStatusPlumbingError {
		return first + "\nretake: " + out, coverStatusTestsFailed
	}
	return out, status
}
