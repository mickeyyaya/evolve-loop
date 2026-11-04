package core

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeBuildFloorConverging signalcenter.Code = "BUILD_FLOOR_CONVERGING"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleBuild, CodeBuildFloorConverging, "a build floor rejection after a correction held fewer findings than the one before, or the same number of other findings, so the build got one further correction inside the policy bound retry.floor_convergence_rounds; fields.before and after (the finding counts), round (the further correction in use), bound (the policy limit)")
}

type floorConvergence struct {
	o          *Orchestrator
	cycle      int
	phase      Phase
	bound      int
	used       int
	prev, cur  []string
	converging bool
}

func (o *Orchestrator) newFloorConvergence(cycle int, phase Phase, first []string) *floorConvergence {
	fc := &floorConvergence{o: o, cycle: cycle, phase: phase, cur: first}
	if phase == PhaseBuild {
		fc.bound = o.retryConfig.FloorConvergenceRounds
	}
	return fc
}

func (fc *floorConvergence) observe(cur []string) {
	fc.prev, fc.cur = fc.cur, cur
	fc.converging = floorFindingsConverge(fc.prev, fc.cur)
}

func (fc *floorConvergence) grant() bool {
	if !fc.converging || fc.used >= fc.bound {
		return false
	}
	fc.used++
	fc.emit()
	return true
}

func (fc *floorConvergence) emit() {
	reason := fmt.Sprintf("build floor: %d finding(s) after the correction, %d before; one further correction (%d of %d)", len(fc.cur), len(fc.prev), fc.used, fc.bound)
	fmt.Fprintf(fc.o.errOut(), "[orchestrator] %s %s\n", CodeBuildFloorConverging, reason)
	fc.o.signals.Emit(signalcenter.Event{
		Cycle: fc.cycle, RunID: fc.o.signalRunID(), Phase: string(fc.phase),
		Module: signalcenter.ModuleBuild, Origin: "floorConvergence.grant", Kind: signalcenter.KindGateCorrected,
		Severity: signalcenter.SeverityInfo, Code: CodeBuildFloorConverging, Reason: reason,
		Fields: map[string]string{
			"before": strconv.Itoa(len(fc.prev)), "after": strconv.Itoa(len(fc.cur)),
			"round": strconv.Itoa(fc.used), "bound": strconv.Itoa(fc.bound),
		},
	})
}

func floorFindingsConverge(prev, cur []string) bool {
	if len(prev) == 0 || len(cur) == 0 || len(cur) > len(prev) {
		return false
	}
	if len(cur) < len(prev) {
		return true
	}
	known := normalizedFindings(prev)
	return slices.ContainsFunc(normalizedFindings(cur), func(f string) bool { return !slices.Contains(known, f) })
}

func normalizedFindings(findings []string) []string {
	out := make([]string, len(findings))
	for i, f := range findings {
		out[i] = findingIdentity(f)
	}
	return out
}

func findingIdentity(finding string) string {
	lines, tests := goTestFailureLines(finding)
	if len(tests) == 0 {
		return normalizeReasonForFingerprint(finding)
	}
	var pkgs []string
	for _, ln := range lines {
		if f := strings.Fields(ln); f[0] == "FAIL" && !slices.Contains(pkgs, f[1]) {
			pkgs = append(pkgs, f[1])
		}
	}
	if label, _, ok := strings.Cut(finding, floorTestsFailMarker); ok && len(pkgs) == 0 {
		pkgs = append(pkgs, label)
	}
	slices.Sort(tests)
	slices.Sort(pkgs)
	return "go test\x00" + strings.Join(pkgs, "\x00") + "\x00" + strings.Join(tests, "\x00")
}
