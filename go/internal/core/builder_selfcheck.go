package core

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeBuilderSelfCheck signalcenter.Code = "BUILD_SELFCHECK_FINDINGS"

const (
	BuilderSelfCheckFindingsFile = "builder-selfcheck-findings.json"

	selfCheckWouldReturn = "would_return_in_session"
	selfCheckReturn      = "return_in_session"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleBuild, CodeBuilderSelfCheck, "the build session closed with no GREEN self-check receipt for its tree and the handoff floor found findings (ADR-0158 decision 18); fields.decision is would_return_in_session (shadow: recorded in builder-selfcheck-findings.json, the flow of today runs) or return_in_session (enforce: the bridge returns the findings into the live session once), fields.stage is shadow or enforce and fields.findings is the count")
}

type builderSelfCheck struct {
	stage   config.Stage
	floor   BuildHandoffFloor
	in      ReviewInput
	signals *signalcenter.Center
	runID   string
	errOut  io.Writer
}

type selfCheckFindings struct {
	Cycle    int      `json:"cycle"`
	Stage    string   `json:"stage"`
	Decision string   `json:"decision"`
	Findings []string `json:"findings"`
}

func (cr *cycleRun) builderSelfCheckBeforeClose(next Phase) CloseCheck {
	stage := cr.o.workflowConfig.BuilderSelfCheck.Stage
	if next != PhaseBuild || stage == config.StageOff {
		return nil
	}
	floor := floorChecks(cr.o.reviewer)
	if len(floor) == 0 {
		return nil
	}
	return builderSelfCheck{
		stage: stage, floor: floor, in: ReviewInputFor(cr.cs, PhaseBuild, cr.req.ProjectRoot),
		signals: cr.o.signals, runID: cr.o.signalRunID(), errOut: cr.o.errOut(),
	}.beforeClose
}

func (c builderSelfCheck) beforeClose(ctx context.Context) string {
	if selfCheckReceiptGreen(ctx, c.in.Workspace, c.in.Worktree) {
		return ""
	}
	findings := c.floor.Failures(ctx, c.in)
	if len(findings) == 0 {
		return ""
	}
	decision := selfCheckWouldReturn
	if c.stage == config.StageEnforce {
		decision = selfCheckReturn
	}
	c.record(decision, findings)
	if decision != selfCheckReturn {
		return ""
	}
	return selfCheckFindingsMessage(findings)
}

func (c builderSelfCheck) record(decision string, findings []string) {
	rec := selfCheckFindings{Cycle: c.in.Cycle, Stage: c.stage.String(), Decision: decision, Findings: findings}
	if err := atomicwrite.JSON(filepath.Join(c.in.Workspace, BuilderSelfCheckFindingsFile), rec); err != nil {
		fmt.Fprintf(c.errOut, "[builder-selfcheck] WARN findings record not written: %v\n", err)
	}
	c.signals.Emit(signalcenter.Event{
		Cycle: c.in.Cycle, RunID: c.runID, Phase: string(PhaseBuild),
		Module: signalcenter.ModuleBuild, Origin: "builderSelfCheck.beforeClose", Kind: signalcenter.KindPhaseOutcome,
		Severity: signalcenter.SeverityWarn, Code: CodeBuilderSelfCheck,
		Reason: fmt.Sprintf("build handoff floor before close (%s): %s, %d finding(s)", rec.Stage, decision, len(findings)),
		Fields: map[string]string{"stage": rec.Stage, "decision": decision, "findings": strconv.Itoa(len(findings))},
	})
}

func selfCheckFindingsMessage(findings []string) string {
	return fmt.Sprintf("The build handoff floor found %d finding(s) on your tree, and you have no GREEN self-check for this tree. The session stays open for one fix. Fix each finding exactly, run `evolve selfcheck build` until it prints GREEN, then write your deliverable again. The phase completes only after that new write.\n  - %s",
		len(findings), strings.Join(findings, "\n  - "))
}
