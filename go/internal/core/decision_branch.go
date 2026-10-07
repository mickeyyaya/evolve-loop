package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/failureadapter"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func (o *Orchestrator) decideAfterRetroRouted(ctx context.Context, cycle int, cs CycleState, seq int, retroVerdict string, history []FailedRecord, in router.RouteInput) (Phase, map[string]string, string, *SystemFailureSignal) {
	// The reason string's prefix ("proceed:"/"retry-with-fallback:"/…) is
	// grepped by dashboards and scenario pins, so it must stay stable.
	detNext, extraEnv, detReason, sig := o.decideAfterRetro(cs, retroVerdict, history)
	if sig != nil {
		return PhaseEnd, nil, detReason, sig
	}
	if strings.HasPrefix(detReason, BookkeepingRegradeReasonPrefix) {
		return detNext, extraEnv, detReason, nil
	}

	in.Current = string(PhaseRetro)
	in.Verdict = retroVerdict
	in.History = entriesFromRecords(history)
	in.Now = o.now()
	rdec := o.strategy.Decide(in)

	branch := PhaseEnd
	if rdec.NextPhase != "" && rdec.NextPhase != router.PhaseEnd {
		branch = Phase(rdec.NextPhase)
	}
	if branch != PhaseEnd && !o.sm.CanTransition(PhaseRetro, branch) {
		forced := detNext
		if router.IsFailureInsert(string(branch)) {
			forced = PhaseTDD
		}
		rdec.Clamps = append(rdec.Clamps, router.Clamp{
			Rule:     "retro-branch-sm-clamped",
			Proposed: string(branch),
			Forced:   string(forced),
		})
		branch = forced
	}
	o.recordRoutingDecision(ctx, cycle, cs, seq, rdec)
	if branch == detNext {
		return branch, extraEnv, detReason, nil // advisor agrees; keep the contract string
	}
	return branch, extraEnv, "retro-routed: " + rdec.Reason, nil
}

func (o *Orchestrator) applyFailureDecisionFloor(cs CycleState, retroVerdict string) *SystemFailureSignal {
	d := buildFailureDossier(cs, retroVerdict, o.failurePolicy)
	_ = writeFailureDossier(cs.WorkspacePath, d) // per-cycle forensics; best-effort

	if d.FloorCandidate != "" && o.failurePolicy.IsFloor(d.FloorCandidate) {
		return &SystemFailureSignal{
			Category: d.FloorCandidate,
			Level:    policy.LevelSystem,
			Evidence: d.Evidence,
			Halt:     true,
		}
	}

	dec, _ := readFailureDecision(cs.WorkspacePath)
	if dec == nil || !o.failurePolicy.IsFloor(dec.Category) {
		return nil
	}
	contradicted := readDispositionLegitimacy(cs.WorkspacePath, cs.CycleID) == legitRejection
	if contradicted {
		return nil
	}
	if refutation := floorClaimRefutation(dec.Category, cs); refutation != "" {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN floor: prose %s claim overruled — %s; treating as task-level FAIL\n", dec.Category, refutation)
		return nil
	}
	ev := dec.Evidence
	if ev == "" {
		ev = dec.Justification
	}
	return &SystemFailureSignal{
		Category: dec.Category,
		Level:    policy.LevelSystem,
		Evidence: "orchestrator-classified " + dec.Category + ": " + ev,
		Halt:     true,
	}
}

func floorClaimRefutation(category string, cs CycleState) string {
	if category == policy.CategoryVerdictIncoherence && hasSubstantiveFailReasons(cs) {
		return "the recorded FAIL carries persisted substantive fail reasons (diagnosed downgrade, not forgery)"
	}
	if staticGateReadingsForcedTheFail(cs.AuditFailReasons) && len(cs.ShipFailReasons) == 0 {
		return "every persisted fail reason is a static reading of the tree by a deterministic audit gate, forced over a PASS or WARN narrative"
	}
	return ""
}

func (o *Orchestrator) decideAfterRetro(cs CycleState, retroVerdict string, history []FailedRecord) (next Phase, extraEnv map[string]string, reason string, sig *SystemFailureSignal) {
	cycleVerdict := retroVerdict
	if cycleVerdict == VerdictPASS {
		cycleVerdict = VerdictFAIL
	}
	s := o.applyFailureDecisionFloor(cs, cycleVerdict)
	if s != nil {
		return PhaseEnd, nil, "system-failure-floor: " + s.Category, s
	}
	if !cs.BookkeepingRegradeAttempted && BookkeepingRegradeEligible(cs.AuditFailReasons) {
		return o.recoveryTarget(PhaseRetro, recoveryKeyBookkeepingRegrade, PhaseAudit), nil,
			BookkeepingRegradeReasonPrefix + "meta-only audit FAIL with non-FAIL narrative; re-dispatching audit once in-cycle", nil
	}
	entries := entriesFromRecords(history)
	dec := failureadapter.Decide(entries, failureadapter.Options{Now: o.now()})
	switch dec.Action {
	case failureadapter.ActionRetryWithFallback:
		return o.recoveryTarget(PhaseRetro, string(dec.Action), PhaseTDD), dec.SetEnv, "retry-with-fallback: " + dec.Reason, nil
	case failureadapter.ActionBlockCode, failureadapter.ActionBlockOperatorAction:
		return o.recoveryTarget(PhaseRetro, string(dec.Action), PhaseEnd), nil, string(dec.Action) + ": " + dec.Reason, nil
	default: // ActionProceed
		return o.recoveryTarget(PhaseRetro, string(dec.Action), PhaseEnd), dec.SetEnv, "proceed: " + dec.Reason, nil
	}
}

func (o *Orchestrator) recoveryTarget(p Phase, key string, fallback Phase) Phase {
	if spec, ok := o.specFor(p); ok && spec.Recovery != nil {
		if t, ok := spec.Recovery.Targets[key]; ok {
			if target := phaseFromRouter(t); target != "" {
				return target
			}
		}
	}
	return fallback
}

func (o *Orchestrator) decideAfterDebugger(resp PhaseResponse) Phase {
	action, _ := resp.Signals["debugger.action"].(string)
	switch action {
	case "RESHIP":
		return o.recoveryTarget(PhaseDebugger, "RESHIP", PhaseShip)
	case "RERUN_PHASE":
		// Rerun targets clamp to upstream phases (audit/build/tdd): re-shipping
		// is the dedicated RESHIP action, so "rerun_phase: ship" must not become
		// a reship that skips re-establishing the precondition.
		rerun, _ := resp.Signals["debugger.rerun_phase"].(string)
		switch o.candidatePhase(rerun) {
		case PhaseAudit:
			return PhaseAudit
		case PhaseBuild:
			return PhaseBuild
		case PhaseTDD:
			return PhaseTDD
		default:
			return o.recoveryTarget(PhaseDebugger, "RERUN_PHASE", PhaseAudit)
		}
	default: // BLOCK, empty, unknown
		return o.recoveryTarget(PhaseDebugger, "BLOCK", PhaseEnd)
	}
}

func (o *Orchestrator) recordShipError(ctx context.Context, cycle int, cs CycleState, se *ShipError) {
	ts := o.now().UTC().Format(time.RFC3339)
	artifactPath := filepath.Join(cs.WorkspacePath, "ship-error.json")
	sha := ""
	payload := map[string]string{
		"code":    string(se.Code),
		"class":   string(se.Class),
		"stage":   string(se.Stage),
		"message": se.Message,
		"debug":   se.DebugString(),
	}
	if buf, err := json.MarshalIndent(payload, "", "  "); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN ship-error marshal: %v\n", err)
		artifactPath = ""
	} else if err := os.MkdirAll(cs.WorkspacePath, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN ship-error mkdir: %v\n", err)
		artifactPath = ""
	} else if err := os.WriteFile(artifactPath, buf, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN ship-error write: %v\n", err)
		artifactPath = ""
	} else {
		sum := sha256.Sum256(buf)
		sha = hex.EncodeToString(sum[:])
	}
	if err := o.ledger.Append(ctx, LedgerEntry{
		TS: ts, Cycle: cycle, Role: "ship", Kind: "ship_error",
		ExitCode: 1, ArtifactPath: artifactPath, ArtifactSHA256: sha,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN ship_error ledger append: %v\n", err)
	}
	o.emitShipError(cycle, cs, se, artifactPath)
}

func (o *Orchestrator) recordDebuggerDecision(ctx context.Context, cycle int, cs CycleState, _ PhaseResponse) {
	artifactPath := filepath.Join(cs.WorkspacePath, "debug-decision.json")
	sha := ""
	if buf, err := os.ReadFile(artifactPath); err == nil {
		sum := sha256.Sum256(buf)
		sha = hex.EncodeToString(sum[:])
	} else {
		artifactPath = ""
	}
	if err := o.ledger.Append(ctx, LedgerEntry{
		TS: o.now().UTC().Format(time.RFC3339), Cycle: cycle, Role: "debugger",
		Kind: "debugger_decision", ExitCode: 0, ArtifactPath: artifactPath, ArtifactSHA256: sha,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN debugger_decision ledger append: %v\n", err)
	}
}

func (o *Orchestrator) recordRoutingDecision(ctx context.Context, cycle int, cs CycleState, seq int, dec router.RouterDecision) {
	ts := o.now().UTC().Format(time.RFC3339)
	artifactPath := filepath.Join(cs.WorkspacePath, fmt.Sprintf("routing-decision-%d.json", seq))
	sha := ""
	if buf, err := json.MarshalIndent(dec, "", "  "); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN routing-decision marshal: %v\n", err)
		artifactPath = ""
	} else if err := os.MkdirAll(cs.WorkspacePath, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN routing-decision mkdir: %v\n", err)
		artifactPath = ""
	} else if err := os.WriteFile(artifactPath, buf, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN routing-decision write: %v\n", err)
		artifactPath = ""
	} else {
		sum := sha256.Sum256(buf)
		sha = hex.EncodeToString(sum[:])
	}

	if err := o.ledger.Append(ctx, LedgerEntry{
		TS: ts, Cycle: cycle, Role: "orchestrator", Kind: "routing_decision",
		ExitCode: 0, ArtifactPath: artifactPath, ArtifactSHA256: sha,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN routing_decision ledger append: %v\n", err)
	}
	for _, sp := range dec.SkipPhases {
		if err := o.ledger.Append(ctx, LedgerEntry{
			TS: ts, Cycle: cycle, Role: sp, Kind: "phase_skipped", ExitCode: 0,
			Source: "router",
		}); err != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase_skipped ledger append: %v\n", err)
		}
	}
	o.emitRegistryGatedPhases(cycle, cs, dec)
}

func (o *Orchestrator) recordPlanRejections(ctx context.Context, cycle int, cs CycleState, rejections []router.PlanRejection) {
	o.recordPlanRejectionsKind(ctx, cycle, cs, rejections, "plan")
}

func (o *Orchestrator) recordPlanRejectionsKind(ctx context.Context, cycle int, cs CycleState, rejections []router.PlanRejection, kind string) {
	if cs.WorkspacePath == "" {
		return
	}
	name := "advisor-rejections.json"
	if kind != "" && kind != "plan" {
		name = "advisor-rejections-" + kind + ".json"
	}
	// nil marshals identically to an empty slice, but writing "[]" distinguishes
	// a validated-clean plan from validation never having run.
	if rejections == nil {
		rejections = []router.PlanRejection{}
	}
	buf, err := json.MarshalIndent(rejections, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN advisor-rejections marshal (cycle %d): %v\n", cycle, err)
		return
	}
	artifactPath := filepath.Join(cs.WorkspacePath, name)
	sha := ""
	if err := os.MkdirAll(cs.WorkspacePath, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN advisor-rejections mkdir: %v\n", err)
		artifactPath = ""
	} else if err := os.WriteFile(artifactPath, buf, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN advisor-rejections write: %v\n", err)
		artifactPath = ""
	} else {
		sum := sha256.Sum256(buf)
		sha = hex.EncodeToString(sum[:])
	}
	if err := o.ledger.Append(ctx, LedgerEntry{
		TS: o.now().UTC().Format(time.RFC3339), Cycle: cycle, Role: "orchestrator",
		Kind: "plan_rejections", ExitCode: 0, ArtifactPath: artifactPath, ArtifactSHA256: sha,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN plan_rejections ledger append: %v\n", err)
	}
}

func (o *Orchestrator) recordPhasePlan(ctx context.Context, cycle int, cs CycleState, plan *router.PhasePlan, clamps []router.Clamp) {
	o.recordPhasePlanKind(ctx, cycle, cs, plan, clamps, "plan")
}

func (o *Orchestrator) recordPhasePlanKind(ctx context.Context, cycle int, cs CycleState, plan *router.PhasePlan, clamps []router.Clamp, kind string) {
	ts := o.now().UTC().Format(time.RFC3339)
	artifactPath := filepath.Join(cs.WorkspacePath, "phase-"+kind+".json")
	sha := ""
	if buf, err := json.MarshalIndent(plan.Entries, "", "  "); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase-%s marshal: %v\n", kind, err)
		artifactPath = ""
	} else if err := os.MkdirAll(cs.WorkspacePath, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase-%s mkdir: %v\n", kind, err)
		artifactPath = ""
	} else if err := os.WriteFile(artifactPath, buf, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase-%s write: %v\n", kind, err)
		artifactPath = ""
	} else {
		sum := sha256.Sum256(buf)
		sha = hex.EncodeToString(sum[:])
	}
	for _, c := range clamps {
		fmt.Fprintf(os.Stderr, "[orchestrator] integrity-floor clamp: %s (%s → %s)\n", c.Rule, c.Proposed, c.Forced)
	}
	if err := o.ledger.Append(ctx, LedgerEntry{
		TS: ts, Cycle: cycle, Role: "orchestrator", Kind: "phase_" + kind,
		ExitCode: 0, ArtifactPath: artifactPath, ArtifactSHA256: sha,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase_%s ledger append: %v\n", kind, err)
	}

	for _, cap := range []struct{ kind, file string }{
		{"advisor_prompt", "advisor-prompt-" + kind + ".txt"},
		{"advisor_response", "advisor-response-" + kind + ".txt"},
	} {
		path := filepath.Join(cs.WorkspacePath, cap.file)
		capSHA := bindArtifactSHA(path)
		if capSHA == "" {
			continue
		}
		if err := o.ledger.Append(ctx, LedgerEntry{
			TS: ts, Cycle: cycle, Role: "orchestrator", Kind: cap.kind,
			ExitCode: 0, ArtifactPath: path, ArtifactSHA256: capSHA,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN %s ledger append: %v\n", cap.kind, err)
		}
	}
}

func bindArtifactSHA(path string) string {
	buf, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}

func escalateRetroReason(reason, pattern string, led *recurrence.Ledger) string {
	if led == nil || !strings.HasPrefix(reason, "proceed:") {
		return reason
	}
	if led.IsGenericPattern(pattern) {
		return reason
	}
	if n := led.Count(pattern); n >= 2 {
		return fmt.Sprintf("adapt: escalated %s to Nth-occurrence (count=%d); %s",
			pattern, n, strings.TrimPrefix(reason, "proceed: "))
	}
	return reason
}

func (o *Orchestrator) escalateRetroReasonForHistory(projectRoot, reason string, history []FailedRecord) string {
	if projectRoot == "" || len(history) == 0 || !strings.HasPrefix(reason, "proceed:") {
		return reason
	}
	pattern := history[len(history)-1].Classification
	if pattern == "" {
		return reason
	}
	led, err := recurrence.Load(filepath.Join(projectRoot, ".evolve", "recurrence-ledger.json"))
	if err != nil {
		return reason
	}
	return escalateRetroReason(reason, pattern, led)
}
