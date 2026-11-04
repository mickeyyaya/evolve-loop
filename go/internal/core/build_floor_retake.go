package core

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeBuildFloorRetakeGreen signalcenter.Code = "BUILD_FLOOR_RETAKE_GREEN"
	CodeBuildFloorRetakeRed   signalcenter.Code = "BUILD_FLOOR_RETAKE_RED"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleBuild, CodeBuildFloorRetakeGreen, "a build floor test run was RED once and GREEN on its one serialized retake under the integration-tier lock, so the build goes on; fields.pkg (the package or the run), tests (the failed top-level test names of the first run; evolve ci classify also keys a failure on its package and top-level test), retake=green")
	signalcenter.RegisterCode(signalcenter.ModuleBuild, CodeBuildFloorRetakeRed, "a build floor test run was RED and its one serialized retake under the integration-tier lock was RED again, so the floor fails; fields.pkg (the package or the run), tests (the failed top-level test names of the retake; evolve ci classify also keys a failure on its package and top-level test), retake=red")
}

func floorScopedPackages(pkgs []string, warn io.Writer) []string {
	scoped, envExclusive, _ := ciparity.TierScope(pkgs)
	if len(envExclusive) > 0 {
		fmt.Fprintf(warn, "[build-floor] NOTE: env-exclusive package(s) %s are not run at the floor under a live loop, as at the audit tier. Backstop: %s\n",
			strings.Join(envExclusive, ", "), ciparity.EnvExclusiveBackstopNote(envExclusive))
	}
	return scoped
}

func (in ReviewInput) retakeFloorRun(label, first string, rerun func() (string, bool)) (string, bool) {
	release := in.acquireFloorRetakeLock()
	out, ok := rerun()
	release()
	failed := out
	if ok {
		failed = first
	}
	_, tests := goTestFailureLines(failed)
	in.emitFloorRetake(label, tests, ok)
	return out, ok
}

func (in ReviewInput) retakeSelfCheckFailures(fails []selfCheckFailure, rerun func(pkg string) (string, bool)) []selfCheckFailure {
	var still []selfCheckFailure
	for _, f := range fails {
		if out, ok := in.retakeFloorRun(f.Pkg, f.Output, func() (string, bool) { return rerun(f.Pkg) }); !ok {
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

func (in ReviewInput) emitFloorRetake(label string, tests []string, green bool) {
	code, verdict, kind := CodeBuildFloorRetakeRed, "red", signalcenter.KindGateRejected
	if green {
		code, verdict, kind = CodeBuildFloorRetakeGreen, "green", signalcenter.KindGatePassed
	}
	names := strings.Join(tests, ",")
	reason := fmt.Sprintf("build floor: %s was RED, and its serialized retake is %s; failed test(s): %s", label, strings.ToUpper(verdict), cmp.Or(names, "none named in the output"))
	fmt.Fprintf(in.errOut(), "[build-floor] %s %s\n", code, reason)
	in.Signals.Emit(signalcenter.Event{
		Cycle: in.Cycle, RunID: in.RunID, Phase: string(PhaseBuild),
		Module: signalcenter.ModuleBuild, Origin: "ReviewInput.emitFloorRetake", Kind: kind,
		Severity: signalcenter.SeverityWarn, Code: code, Reason: reason,
		Fields: map[string]string{"pkg": label, "tests": names, "retake": verdict},
	})
}

func goTestFailureLines(output string) (lines, tests []string) {
	for _, ln := range strings.Split(output, "\n") {
		ln = strings.TrimSpace(ln)
		if name, ok := strings.CutPrefix(ln, "--- FAIL: "); ok {
			lines = append(lines, ln)
			top, _, _ := strings.Cut(name, " ")
			top, _, _ = strings.Cut(top, "/")
			if top != "" && !slices.Contains(tests, top) {
				tests = append(tests, top)
			}
			continue
		}
		if f := strings.Fields(ln); len(f) >= 2 && f[0] == "FAIL" {
			lines = append(lines, ln)
		}
	}
	return lines, tests
}

func coverRetakeResult(first, out string, status int) (string, int) {
	if status == coverStatusPlumbingError {
		return first + "\nretake: " + out, coverStatusTestsFailed
	}
	return out, status
}
