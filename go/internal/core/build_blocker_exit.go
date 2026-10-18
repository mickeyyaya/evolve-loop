package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeBlockerRouted  signalcenter.Code = "ORCHESTRATOR_BLOCKER_ROUTED"
	CodeBlockerRefused signalcenter.Code = "ORCHESTRATOR_BLOCKER_REFUSED"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeBlockerRouted, "a phase's gate failures are all explained by a blocker the host proved (a protected path the fix needs, or two of the cycle's own predicates that conflict); the correction rounds stop, the cycle seals FAIL with termination reason blocked-needs-console and the BLOCKER_ROUTE diagnostic routes the item to the console; the reason is the proof and the lane's diagnosis")
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeBlockerRefused, "a phase's verdict declared a BLOCKER defect that the host could not prove, or that does not explain every gate failure; the correction rounds continue as for any rejection; the reason lists each refusal")
}

var acsPredicateFailure = regexp.MustCompile(`^\./acs/cycle[0-9]+ \(-tags [^)]*\): unit tests FAIL`)

func buildFloorExplainable(failure string) bool {
	return strings.HasPrefix(failure, coverStrictHeadline) || acsPredicateFailure.MatchString(failure)
}

func (cr *cycleRun) buildBlockerCase(next Phase, rin ReviewInput, rr ReviewResult) (BlockerCase, bool) {
	if next != PhaseBuild || len(rr.Failures) == 0 {
		return BlockerCase{}, false
	}
	defects := phaseVerdictDefects(cr.cs.WorkspacePath, next)
	if len(parseBlockerClaims(defects)) == 0 {
		return BlockerCase{}, false
	}
	diff, fault := worktreeDiffSince(cr.ctx, rin.Worktree, floorBase(rin))
	return BlockerCase{
		Phase:               string(next),
		Worktree:            rin.Worktree,
		PredicatesAtRequest: cr.predicatesAtBuild,
		PredicatesNow:       cyclePredicateSources(rin.Worktree, cr.cycle),
		Changed:             diff.changed,
		ShrunkTests:         diff.shrunkTests,
		Protected:           cr.o.protectedSurface,
		Failures:            rr.Failures,
		Defects:             defects,
		Explainable:         buildFloorExplainable,
		EvidenceFault:       fault,
	}, true
}

func cyclePredicateSources(worktree string, cycle int) map[string]string {
	sources := map[string]string{}
	if worktree == "" {
		return sources
	}
	files, _ := filepath.Glob(filepath.Join(worktree, "go", "acs", fmt.Sprintf("cycle%d", cycle), "*.go"))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			sources[filepath.Base(f)] = "unreadable: " + err.Error()
			continue
		}
		sources[filepath.Base(f)] = string(data)
	}
	return sources
}

type worktreeDiff struct {
	changed     []string
	shrunkTests []string
}

func worktreeDiffSince(ctx context.Context, worktree, base string) (worktreeDiff, string) {
	numstat, code, err := gitCapture(ctx, worktree, "diff", "--numstat", "--no-renames", base)
	if err != nil || code != 0 {
		return worktreeDiff{}, fmt.Sprintf("git diff --numstat %s failed (rc=%d, %v)", base, code, err)
	}
	others, code, err := gitCapture(ctx, worktree, "ls-files", "--others", "--exclude-standard")
	if err != nil || code != 0 {
		return worktreeDiff{}, fmt.Sprintf("git ls-files --others failed (rc=%d, %v)", code, err)
	}
	var d worktreeDiff
	for _, line := range strings.Split(strings.TrimSpace(numstat), "\n") {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) != 3 {
			continue
		}
		d.changed = append(d.changed, fields[2])
		if strings.HasSuffix(fields[2], "_test.go") && fields[1] != "0" {
			d.shrunkTests = append(d.shrunkTests, fields[2])
		}
	}
	d.changed = append(d.changed, strings.Fields(others)...)
	return d, ""
}

func phaseVerdictDefects(workspace string, phase Phase) []string {
	data, err := os.ReadFile(filepath.Join(workspace, string(phase)+"-report.md"))
	if err != nil {
		return nil
	}
	s, ok := phasecontract.ParseVerdictSentinelFull(string(data))
	if !ok || s.Failure == nil {
		return nil
	}
	return s.Failure.Defects
}

func (cr *cycleRun) provenBlocker(next Phase, rin ReviewInput, rr ReviewResult) (string, BlockerProof, bool) {
	c, ok := cr.buildBlockerCase(next, rin, rr)
	if !ok {
		return "", BlockerProof{}, false
	}
	proof := NewBlockerProver().Prove(c)
	if !proof.Blocked() {
		cr.refuseBlocker(next, "the declared blocker is not proven: "+strings.Join(proof.Refusals, "; "))
		return "", proof, false
	}
	committed, decided := committedset.Committed(cr.cs.WorkspacePath)
	if !decided || len(committed) != 1 {
		cr.refuseBlocker(next, fmt.Sprintf("the blocker is proven, but the console route names one item and the cycle committed %v", committed))
		return "", proof, false
	}
	return committed[0], proof, true
}

func (cr *cycleRun) endOnProvenBlocker(next Phase, dr *dispatchResult, item string, proof BlockerProof) (loopAction, error) {
	diag := BlockerRouteDiagnostic(string(next), item, proof)
	resp := dr.resp
	resp.Verdict = VerdictFAIL
	resp.Diagnostics = append([]Diagnostic{diag}, resp.Diagnostics...)
	fmt.Fprintf(cr.o.errOut(), "[orchestrator] phase %s: %s\n", next, diag.Message)
	cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath,
		phaseOutcomeFrom(next, resp, dr.attemptCount, CycleTerminationBlockedNeedsConsole, cr.cs.PhaseStartedAt))
	cr.result.FinalVerdict = VerdictFAIL
	cr.result.TerminationReason = CycleTerminationBlockedNeedsConsole
	cr.emitBlockerSignal(next, CodeBlockerRouted, signalcenter.SeverityWarn, diag.Message)
	dr.phaseReq.CorrectionDirective = ""
	return loopBreak, nil
}

func (cr *cycleRun) refuseBlocker(next Phase, reason string) {
	if reason == cr.lastBlockerRefusal {
		return
	}
	cr.lastBlockerRefusal = reason
	fmt.Fprintf(cr.o.errOut(), "[orchestrator] phase %s: %s\n", next, reason)
	cr.emitBlockerSignal(next, CodeBlockerRefused, signalcenter.SeverityWarn, reason)
}

func (cr *cycleRun) emitBlockerSignal(next Phase, code signalcenter.Code, severity signalcenter.Severity, reason string) {
	cr.o.signals.Emit(signalcenter.Event{
		Cycle: cr.cycle, RunID: cr.o.signalRunID(), Phase: string(next),
		Module: signalcenter.ModuleOrchestrator, Origin: "cycleRun.provenBlocker", Kind: signalcenter.KindGateRejected,
		Severity: severity, Code: code, Reason: reason,
	})
}
