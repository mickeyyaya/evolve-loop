package core

import (
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

type FloorResidualClass string

const (
	FloorResidualCoverChanged FloorResidualClass = "cover_changed"
	FloorResidualChangedAreas FloorResidualClass = "changed_areas"

	floorResidualClass = "build-floor-residual"
)

type FloorResidual struct {
	Class    FloorResidualClass
	Failure  string
	Source   string
	Line     int
	Func     string
	Document string
	Path     string
}

type floorPrescription struct {
	evidence string
	target   string
	defect   string
	text     string
}

func (r FloorResidual) prescribe() (floorPrescription, bool) {
	switch r.Class {
	case FloorResidualCoverChanged:
		test := strings.TrimSuffix(r.Source, ".go") + "_test.go"
		at := fmt.Sprintf("%s:%d", r.Source, r.Line)
		return floorPrescription{
			evidence: r.Source, target: test, defect: at + " " + r.Func + " is below 100% statement coverage",
			text: "add tests in " + test + " that cover " + r.Func + " (" + at + ") to 100% statement coverage",
		}, true
	case FloorResidualChangedAreas:
		return floorPrescription{
			evidence: r.Document, target: r.Document, defect: r.Document + ": Changed Areas does not explain material path " + r.Path,
			text: "add a Changed Areas line for " + r.Path + " in " + r.Document,
		}, true
	}
	return floorPrescription{}, false
}

type floorResidualLog struct{ items []FloorResidual }

func (l *floorResidualLog) add(rs ...FloorResidual) {
	if l != nil {
		l.items = append(l.items, rs...)
	}
}

func (l *floorResidualLog) list() []FloorResidual {
	if l == nil {
		return nil
	}
	return l.items
}

func (l *floorResidualLog) noteOmission(o explanationdocs.Omission) {
	l.add(FloorResidual{Class: FloorResidualChangedAreas, Failure: o.Failure, Document: o.Document, Path: o.Path})
}

func coverResiduals(failures []string, refused []covchanged.Func, prefix string) []FloorResidual {
	var out []FloorResidual
	for _, failure := range failures {
		for _, f := range refused {
			out = append(out, FloorResidual{Class: FloorResidualCoverChanged, Failure: failure, Source: path.Join(prefix, f.File), Line: f.Line, Func: f.Key})
		}
	}
	return out
}

func moduleRepoPrefix(worktree, moduleDir string) string {
	rel, err := filepath.Rel(worktree, moduleDir)
	if err != nil {
		return "."
	}
	return filepath.ToSlash(rel)
}

func floorResidualBlock(rr ReviewResult) (phasecontract.FailureBlock, bool) {
	fb := phasecontract.FailureBlock{Class: floorResidualClass}
	var kept []FloorResidual
	for _, r := range rr.Residuals {
		if slices.Contains(rr.Failures, r.Failure) && !slices.Contains(kept, r) {
			kept = append(kept, r)
		}
	}
	for _, f := range rr.Failures {
		if !slices.ContainsFunc(kept, func(r FloorResidual) bool { return r.Failure == f }) {
			return fb, false
		}
	}
	for _, r := range kept {
		p, ok := r.prescribe()
		if !ok {
			return fb, false
		}
		fb.Defects = append(fb.Defects, p.defect)
		fb.Prescription = append(fb.Prescription, p.text)
		fb.EvidencePaths = append(fb.EvidencePaths, p.evidence)
		fb.Targets = append(fb.Targets, p.target)
	}
	fb.EvidencePaths = slices.Compact(slices.Sorted(slices.Values(fb.EvidencePaths)))
	fb.Targets = slices.Compact(slices.Sorted(slices.Values(fb.Targets)))
	return fb, len(kept) > 0
}

func (cr *cycleRun) floorResidualAuditFix(dr *dispatchResult, rr ReviewResult, phaseErr error, refused blockerStatement) (loopAction, bool) {
	fb, ok := floorResidualBlock(rr)
	if !ok {
		return loopAbort, false
	}
	res, ran := cr.runAuditFix(PhaseBuild, &fb, auditFixSourceBuildFloor)
	switch {
	case !ran:
		return loopAbort, false
	case res.Stop:
		cr.sealFloorResidualHandoff(dr, repairHandoffReasonPrefix+res.Reason+": the build floor residuals stay after the audit_fix escalation ("+phaseErr.Error()+"); salvage "+cr.cs.ActiveWorktree)
		return loopBreak, true
	case res.Handled && !res.Escalate:
		fmt.Fprintf(cr.o.errOut(), "[orchestrator] cycle %d: audit_fix %s on the build floor residuals; the build is delivered\n", cr.cycle, res.Reason)
		cr.sendLastRefusal(PhaseBuild, refused, blockerStatement.delivered)
		return loopNext, true
	}
	return loopAbort, false
}

func (cr *cycleRun) sealFloorResidualHandoff(dr *dispatchResult, reason string) {
	cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, rejectedOutcome(phaseOutcomeFrom(PhaseBuild, dr.resp, dr.attemptCount, reason, cr.cs.PhaseStartedAt)))
	cr.result.FinalVerdict = VerdictFAIL
	cr.sealHandedOff(&cr.result, cr.phaseTimings, reason)
}
