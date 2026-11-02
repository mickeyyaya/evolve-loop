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

	selfCheckWouldReturn    = "would_return_in_session"
	selfCheckDelivered      = "delivered"
	selfCheckDeliveryFailed = "delivery_failed"
	selfCheckSourceBridge   = "bridge"
	selfCheckSourceGate     = "gate"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleBuild, CodeBuilderSelfCheck, "a build closed with no GREEN self-check receipt for its tree and the handoff floor found findings (ADR-0158 decision 18); fields.stage is shadow or enforce; fields.source is gate (shadow: the findings of the review gate, no floor run in the wait) or bridge (enforce: the floor run before the close); fields.decision is would_return_in_session (shadow), delivered (enforce, the paste into the live session succeeded) or delivery_failed (enforce, fields.error names why); fields.receipt is absent, red or stale; fields.findings is the count")
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
	Source   string   `json:"source"`
	Decision string   `json:"decision"`
	Receipt  string   `json:"receipt"`
	Error    string   `json:"error,omitempty"`
	Findings []string `json:"findings"`
}

func (cr *cycleRun) selfCheckFor(in ReviewInput, floor BuildHandoffFloor) builderSelfCheck {
	return builderSelfCheck{
		stage: cr.o.workflowConfig.BuilderSelfCheck.Stage, floor: floor, in: in,
		signals: cr.o.signals, runID: cr.o.signalRunID(), errOut: cr.o.errOut(),
	}
}

func (cr *cycleRun) builderSelfCheckBeforeClose(next Phase) CloseCheck {
	if next != PhaseBuild || cr.o.workflowConfig.BuilderSelfCheck.Stage != config.StageEnforce {
		return nil
	}
	floor := floorChecks(cr.o.reviewer)
	if len(floor) == 0 {
		return nil
	}
	return cr.selfCheckFor(ReviewInputFor(cr.cs, PhaseBuild, cr.req.ProjectRoot), floor).beforeClose
}

func (cr *cycleRun) recordShadowSelfCheck(next Phase, in ReviewInput, rr ReviewResult) {
	if next != PhaseBuild || cr.o.workflowConfig.BuilderSelfCheck.Stage != config.StageShadow || rr.Approve || !strings.HasPrefix(rr.Reason, floorReasonHead) {
		return
	}
	receipt := selfCheckReceiptState(cr.ctx, in.Workspace, in.Worktree)
	if receipt == receiptGreen {
		return
	}
	cr.selfCheckFor(in, nil).record(selfCheckFindings{Source: selfCheckSourceGate, Decision: selfCheckWouldReturn, Receipt: receipt, Findings: rr.Failures})
}

func (c builderSelfCheck) beforeClose(ctx context.Context) (string, func(error)) {
	receipt := selfCheckReceiptState(ctx, c.in.Workspace, c.in.Worktree)
	if receipt == receiptGreen {
		return "", nil
	}
	findings := c.floor.Failures(ctx, c.in)
	if len(findings) == 0 || ctx.Err() != nil {
		return "", nil
	}
	report := func(delivery error) {
		rec := selfCheckFindings{Source: selfCheckSourceBridge, Decision: selfCheckDelivered, Receipt: receipt, Findings: findings}
		if delivery != nil {
			rec.Decision, rec.Error = selfCheckDeliveryFailed, delivery.Error()
		}
		c.record(rec)
	}
	return selfCheckFindingsMessage(findings), report
}

func (c builderSelfCheck) record(rec selfCheckFindings) {
	rec.Cycle, rec.Stage = c.in.Cycle, c.stage.String()
	if err := atomicwrite.JSON(filepath.Join(c.in.Workspace, BuilderSelfCheckFindingsFile), rec); err != nil {
		fmt.Fprintf(c.errOut, "[builder-selfcheck] WARN findings record not written: %v\n", err)
	}
	c.signals.Emit(signalcenter.Event{
		Cycle: c.in.Cycle, RunID: c.runID, Phase: string(PhaseBuild),
		Module: signalcenter.ModuleBuild, Origin: "builderSelfCheck.record", Kind: signalcenter.KindPhaseOutcome,
		Severity: signalcenter.SeverityWarn, Code: CodeBuilderSelfCheck,
		Reason: fmt.Sprintf("build handoff floor before close (%s, %s): %s, %d finding(s)", rec.Stage, rec.Source, rec.Decision, len(rec.Findings)),
		Fields: map[string]string{
			"stage": rec.Stage, "source": rec.Source, "decision": rec.Decision, "receipt": rec.Receipt,
			"error": rec.Error, "findings": strconv.Itoa(len(rec.Findings)),
		},
	})
}

func selfCheckFindingsMessage(findings []string) string {
	return fmt.Sprintf("The build handoff floor found %d finding(s) on your tree, and you have no GREEN self-check for this tree. The session stays open for one fix. Fix each finding exactly, run `evolve selfcheck build` until it prints GREEN, then write your deliverable again. The phase completes only after that new write.\n  - %s",
		len(findings), strings.Join(findings, "\n  - "))
}
