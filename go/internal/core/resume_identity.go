package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleidentity"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	LedgerKindCycleIdentity         = "cycle_identity"
	LedgerKindCycleIdentityOverride = "cycle_identity_override"

	CodeResumeIdentityShadow signalcenter.Code = "CYCLE_RESUME_IDENTITY_SHADOW"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleCycle, CodeResumeIdentityShadow, "at resume.identity.stage shadow, a cycle resume found that the phase graph, the phase contracts or the policy changed since the cycle recorded its identity (W2.2). The resume went on. At enforce the resume refuses with the same parts. Fields: stage, parts (graph, contract, policy), recorded, current")
}

type ResumeIdentityConfig struct {
	Stage   config.Stage
	Compute func(projectRoot string) (cycleidentity.Identity, error)
}

func WithResumeIdentity(cfg ResumeIdentityConfig) Option {
	return func(o *Orchestrator) { o.resumeIdentity = cfg }
}

func (c ResumeIdentityConfig) active() bool {
	return c.Compute != nil && c.Stage != config.StageOff
}

func (o *Orchestrator) sealResumeBinding(ctx context.Context, projectRoot string, cs CycleState) {
	o.persistResumeWitness(projectRoot, cs)
	o.recordCycleIdentity(ctx, projectRoot, cs.CycleID)
}

func (o *Orchestrator) recordCycleIdentity(ctx context.Context, projectRoot string, cycle int) {
	cfg := o.resumeIdentity
	if !cfg.active() {
		return
	}
	id, err := cfg.Compute(projectRoot)
	if err != nil {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN cycle identity: %v; the cycle records none\n", err)
		return
	}
	entry := LedgerEntry{TS: o.now().UTC().Format(time.RFC3339), Cycle: cycle, Role: "orchestrator", Kind: LedgerKindCycleIdentity, Message: cycleidentity.Encode(id)}
	if err := o.ledger.Append(ctx, entry); err != nil {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN cycle identity: ledger append: %v\n", err)
	}
}

func (o *Orchestrator) recordedIdentity(ctx context.Context, cycle int) (cycleidentity.Identity, error) {
	it, err := o.ledger.Iter(ctx)
	if err != nil {
		return cycleidentity.Identity{}, err
	}
	defer func() { _ = it.Close() }()
	for {
		e, ok, err := it.Next()
		if err != nil || !ok {
			return cycleidentity.Identity{}, firstErr(err, cycleidentity.ErrNoIdentity)
		}
		if e.Kind == LedgerKindCycleIdentity && e.Cycle == cycle {
			return cycleidentity.Decode(e.Message)
		}
	}
}

func firstErr(err, fallback error) error {
	if err != nil {
		return err
	}
	return fallback
}

type resumeCheck struct {
	req   CycleRequest
	point *ResumePoint
	state CycleState
}

func (o *Orchestrator) verifyResume(ctx context.Context, c resumeCheck) error {
	if err := o.verifyResumeIdentityAndHealCarrier(ctx, c.req.ProjectRoot, c.point, c.state); err != nil {
		return err
	}
	cycle := c.state.CycleID
	if cycle == 0 {
		cycle = c.point.CycleID
	}
	return o.verifyResumeIdentity(ctx, c.req, cycle)
}

func (o *Orchestrator) verifyResumeIdentity(ctx context.Context, req CycleRequest, cycle int) error {
	cfg := o.resumeIdentity
	if !cfg.active() {
		return nil
	}
	recorded, err := o.recordedIdentity(ctx, cycle)
	if errors.Is(err, cycleidentity.ErrNoIdentity) {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN resume: cycle %d recorded no identity; it resumes unchecked\n", cycle)
		return nil
	}
	if err != nil {
		return o.identityUnverified(cfg, cycle, err)
	}
	current, err := cfg.Compute(req.ProjectRoot)
	if err != nil {
		return o.identityUnverified(cfg, cycle, err)
	}
	var mismatch *cycleidentity.MismatchError
	if !errors.As(cycleidentity.Check(recorded, current), &mismatch) {
		return nil
	}
	return o.settleIdentityMismatch(ctx, resumeIdentityCase{req: req, cycle: cycle, recorded: recorded, current: current}, mismatch)
}

func (o *Orchestrator) identityUnverified(cfg ResumeIdentityConfig, cycle int, err error) error {
	if cfg.Stage == config.StageEnforce {
		return fmt.Errorf("resume: cycle %d identity not verified: %w", cycle, err)
	}
	fmt.Fprintf(o.errOut(), "[orchestrator] WARN resume: cycle %d identity not verified: %v; the resume goes on\n", cycle, err)
	return nil
}

type resumeIdentityCase struct {
	req      CycleRequest
	cycle    int
	recorded cycleidentity.Identity
	current  cycleidentity.Identity
}

func (o *Orchestrator) settleIdentityMismatch(ctx context.Context, c resumeIdentityCase, mismatch *cycleidentity.MismatchError) error {
	parts := cycleidentity.JoinParts(mismatch.Parts)
	switch {
	case c.req.AcceptIdentityChange:
		return o.recordIdentityOverride(ctx, c, parts)
	case o.resumeIdentity.Stage == config.StageEnforce:
		return fmt.Errorf("resume: cycle %d refused: %w", c.cycle, mismatch)
	}
	o.emitResumeIdentityShadow(c, parts)
	return nil
}

func (o *Orchestrator) recordIdentityOverride(ctx context.Context, c resumeIdentityCase, parts string) error {
	entry := LedgerEntry{
		TS: o.now().UTC().Format(time.RFC3339), Cycle: c.cycle, Role: "orchestrator", Kind: LedgerKindCycleIdentityOverride,
		Action: "accept_identity_change", Message: "changed: " + parts + "; recorded " + cycleidentity.Encode(c.recorded) + "; current " + cycleidentity.Encode(c.current),
	}
	if err := o.ledger.Append(ctx, entry); err != nil {
		return fmt.Errorf("resume: cycle %d record identity override: %w", c.cycle, err)
	}
	return nil
}

func (o *Orchestrator) emitResumeIdentityShadow(c resumeIdentityCase, parts string) {
	o.signals.Emit(signalcenter.Event{
		Cycle: c.cycle, RunID: o.signalRunID(),
		Module: signalcenter.ModuleCycle, Origin: "Orchestrator.verifyResumeIdentity", Kind: signalcenter.KindPhaseOutcome,
		Severity: signalcenter.SeverityWarn, Code: CodeResumeIdentityShadow,
		Reason: "resume identity (shadow): changed " + parts,
		Fields: map[string]string{
			"stage": config.StageShadow.String(), "parts": parts,
			"recorded": cycleidentity.Encode(c.recorded), "current": cycleidentity.Encode(c.current),
		},
	})
}
