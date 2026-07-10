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
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// decideAfterRetro consults the failure-adapter over cycle history
// (state.failedApproaches) to pick the post-retro branch.
//
// Mapping (retro verdict × failureadapter action → next phase):
//   - retro PASS               → ship   (retrospective recovered the cycle)
//   - retro FAIL/WARN + BLOCK-* → end    (cycle history forbids further work)
//   - retro FAIL/WARN + RETRY  → tdd    (retry from earlier phase w/ fallback env)
//   - retro FAIL/WARN + PROCEED → end   (no recovery, no block — exit cleanly)
//
// Returned reason is "<action>: <failureadapter reason>" for the
// CycleResult.RetroDecision audit field.
// decideAfterRetroRouted is decideAfterRetro for Stage>=Advisory (failure
// floor Phase 3): the routing strategy decides the failure branch — which
// applies the advisor's failure vocabulary (RecoveryAction, failure-scoped
// inserts) above the failure-adapter floor, BLOCK non-overridable — and the
// decision is recorded as a routing-decision artifact, giving failure
// branches the same forensic trail as happy-path transitions. extraEnv
// still comes from the deterministic adapter (SetEnv is kernel-owned).
//
// The routed branch is adopted only where the state machine allows
// (retro→{ship,tdd,end} today); a routed failure-insert (fault-localization
// / bug-reproduction) is clamped to the legal retry target until the SM
// opens that edge — kernel disposes, and the clamp is visible in the
// artifact.
func (o *Orchestrator) decideAfterRetroRouted(ctx context.Context, cycle int, cs CycleState, seq int, retroVerdict string, history []FailedRecord, in router.RouteInput) (Phase, map[string]string, string) {
	// Deterministic baseline: branch, kernel-owned SetEnv, and the
	// operator-facing reason contract ("proceed:"/"retry-with-fallback:"/…)
	// that dashboards and scenario pins grep for.
	detNext, extraEnv, detReason := o.decideAfterRetro(retroVerdict, history)
	if retroVerdict == VerdictPASS {
		return detNext, extraEnv, detReason // PASS recovers; not a failure branch
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
		// A failure-scoped insert (fault-localization/bug-reproduction)
		// carries retry intent — clamp to the legal retry target. Any
		// other illegal phase falls back to the deterministic branch:
		// the SM clamp must never UPGRADE a proceed-to-end into a retry.
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
		return branch, extraEnv, detReason // advisor agrees; keep the contract string
	}
	return branch, extraEnv, "retro-routed: " + rdec.Reason
}

func (o *Orchestrator) decideAfterRetro(retroVerdict string, history []FailedRecord) (next Phase, extraEnv map[string]string, reason string) {
	// retro PASS → ship; no failureadapter consultation.
	if retroVerdict == VerdictPASS {
		return o.recoveryTarget(PhaseRetro, VerdictPASS, PhaseShip), nil, "retro-recovered: ship"
	}
	entries := entriesFromRecords(history)
	dec := failureadapter.Decide(entries, failureadapter.Options{Now: o.now()})
	switch dec.Action {
	case failureadapter.ActionRetryWithFallback:
		return o.recoveryTarget(PhaseRetro, string(dec.Action), PhaseTDD), dec.SetEnv, "retry-with-fallback: " + dec.Reason
	case failureadapter.ActionBlockCode, failureadapter.ActionBlockOperatorAction:
		return o.recoveryTarget(PhaseRetro, string(dec.Action), PhaseEnd), nil, string(dec.Action) + ": " + dec.Reason
	default: // ActionProceed
		return o.recoveryTarget(PhaseRetro, string(dec.Action), PhaseEnd), dec.SetEnv, "proceed: " + dec.Reason
	}
}

// recoveryTarget resolves a control phase's recovery successor from its
// RecoveryMap (PA-DDK DDK-6): spec.Recovery.Targets[key], denormalized through
// phaseFromRouter. An unmapped key (or no map / catalog) returns the literal
// fallback. The chosen edge is still gated by CanTransition at the call site —
// config selects the target, the legality graph constrains it.
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

// recoverFromShipError resolves a ship-phase ShipError via the advisor's
// recovery chain (Strategy + Chain-of-Responsibility, Component #6/#7). Ship is
// a pure executor: it never rejects a cycle, it returns a structured error and
// the orchestrator decides what to do. This records the error for forensics,
// then asks the strategy's Recover() for the recovery phase. Returns
// (phase, true) to proceed with recovery, or ("", false) to abort the cycle:
//   - depth >= maxRecoveryDepth  → exhausted, abort loud
//   - recovery routes to end     → integrity breach / unmapped, abort loud
//   - illegal ship→cand edge     → defensive abort
//
// Recovery is structural (always available via StaticPreset.Recover) and so runs
// regardless of the dynamic-routing Stage — it is error handling, not routing.

func (o *Orchestrator) decideAfterDebugger(resp PhaseResponse) Phase {
	action, _ := resp.Signals["debugger.action"].(string)
	switch action {
	case "RESHIP":
		return o.recoveryTarget(PhaseDebugger, "RESHIP", PhaseShip)
	case "RERUN_PHASE":
		// Clamp rerun targets to UPSTREAM phases (audit/build/tdd) — re-shipping
		// is the dedicated RESHIP action, so a "rerun_phase: ship" must not become
		// a reship that skips re-establishing the precondition. An unrecognized or
		// non-upstream target falls to the config-declared RERUN_PHASE default.
		// (Defense-in-depth: the loop's CanTransition gate independently rejects
		// illegal edges.)
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

// recordShipError persists a ShipError to <workspace>/ship-error.json and
// appends a hash-bound ship_error ledger entry (Component #6 forensics). The
// tamper-evident trail lets the failure-adapter and operators see every
// auto-recovery. Best-effort: a marshal/write/append failure WARNs and is
// swallowed — forensics must never compound a ship failure into a cycle abort.
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
}

// recordDebuggerDecision appends a hash-bound debugger_decision ledger entry
// pointing at the debugger's debug-decision.json artifact (Component #6
// forensics). Best-effort: failures WARN and are swallowed.
func (o *Orchestrator) recordDebuggerDecision(ctx context.Context, cycle int, cs CycleState, _ PhaseResponse) {
	// The action + root_cause live in the debug-decision.json artifact; the
	// ledger entry binds its SHA so the decision is tamper-evident without
	// duplicating the payload into a field LedgerEntry does not have.
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

// recordRoutingDecision marshals the RouterDecision to
// <workspace>/routing-decision-<seq>.json and appends a hash-bound
// routing_decision ledger entry, plus one phase_skipped entry per declined
// optional phase (preserving the PSMAS resume/audit-binding contract).
//
// Best-effort: a marshal/write/append failure WARNs and is swallowed —
// routing forensics must never abort a cycle. Called only when Stage != Off,
// so the legacy path appends nothing new.
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
}

// recordPlanRejections persists the WS2-S1 ValidatePlan findings to
// advisor-rejections.json (ADR-0052 WS2-S2). STANDALONE telemetry — decoupled
// from the WS3-S3 decision span and from phase-plan.json — and best-effort /
// fail-open: a capture failure WARNs but never affects the cycle. It NEVER
// mutates the plan; the integrity floor (ClampPlanToFloorWith) remains the sole
// disposer. An empty finding set still writes ("[]" = validated-clean, distinct
// from "validation never ran"); nil ⇒ [] so the artifact is always well-formed.
// The artifact is hash-bound into the ledger like every sibling decision
// artifact (recordPhasePlan / recordRoutingDecision), so a post-hoc mutation is
// tamper-evident — "standalone" means a separate file, not outside the chain.
func (o *Orchestrator) recordPlanRejections(ctx context.Context, cycle int, cs CycleState, rejections []router.PlanRejection) {
	if cs.WorkspacePath == "" {
		return
	}
	if rejections == nil {
		rejections = []router.PlanRejection{}
	}
	buf, err := json.MarshalIndent(rejections, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN advisor-rejections marshal (cycle %d): %v\n", cycle, err)
		return
	}
	artifactPath := filepath.Join(cs.WorkspacePath, "advisor-rejections.json")
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

// recordPhasePlan persists the advisor's CLAMPED INITIAL whole-cycle plan. It is
// the back-compat entry point (kind "plan" → phase-plan.json); the post-scout
// re-plan records via recordPhasePlanKind with kind "replan".
func (o *Orchestrator) recordPhasePlan(ctx context.Context, cycle int, cs CycleState, plan *router.PhasePlan, clamps []router.Clamp) {
	o.recordPhasePlanKind(ctx, cycle, cs, plan, clamps, "plan")
}

// recordPhasePlanKind persists a CLAMPED whole-cycle plan to
// <workspace>/phase-<kind>.json (a bare PhasePlanEntry array, symmetric with the
// advisor's wire format) and appends a hash-bound phase_<kind> ledger entry, then
// hash-binds the WS3-S1 capture artifacts for that kind (advisor-{prompt,response}
// -<kind>.txt). kind is "plan" (the initial Plan) or "replan" (the WS2 post-scout
// re-plan), so the two decisions land in distinct, separately-diffable artifacts.
// Any integrity-floor clamps that fired are logged for operator visibility.
// Best-effort: a marshal/write/append failure WARNs and is swallowed — plan
// forensics must never abort a cycle.
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

	// WS3-S2: hash-bind the WS3-S1 capture artifacts so a post-hoc mutation of
	// the persisted routing prompt/response is detectable (the ledger's hash
	// chain carries the tamper-evidence). One bound entry per artifact, reusing
	// the ArtifactPath+ArtifactSHA256 shape. Fail-open: a capture that never
	// landed (WS3-S1 is best-effort, or a pre-WS3 cycle) binds nothing.
	for _, cap := range []struct{ kind, file string }{
		{"advisor_prompt", "advisor-prompt-" + kind + ".txt"},
		{"advisor_response", "advisor-response-" + kind + ".txt"},
	} {
		path := filepath.Join(cs.WorkspacePath, cap.file)
		capSHA := bindArtifactSHA(path)
		if capSHA == "" {
			continue // capture absent — nothing to bind
		}
		if err := o.ledger.Append(ctx, LedgerEntry{
			TS: ts, Cycle: cycle, Role: "orchestrator", Kind: cap.kind,
			ExitCode: 0, ArtifactPath: path, ArtifactSHA256: capSHA,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN %s ledger append: %v\n", cap.kind, err)
		}
	}
}

// bindArtifactSHA returns the hex sha256 of the file at path, or "" if it is
// absent/unreadable. WS3-S1 capture is best-effort, so a missing artifact is
// expected and binds nothing — never an error that could abort a cycle.
func bindArtifactSHA(path string) string {
	buf, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}

// enforceNext maps the router's proposed NextPhase back to a core.Phase and
// returns it ONLY if it differs from the static successor AND survives both
// kernel gates: a legal edge (CanTransition) and the artifact-backed spine
// gate (SpineSatisfiedUpTo). Otherwise the static successor stands. This is
// the non-bypassable "kernel disposes" floor for Enforce mode — neither
// Strategy can reach Ship without a real PASS/WARN audit artifact.

// escalateRetroReason upgrades a deterministic "proceed:" retro reason to an
// "adapt:"-with-escalation reason when the failing lesson pattern has recurred
// (recurrence ledger count>=2). A repeat pattern must never emit a bare
// "proceed" — the ledger consult is what gives noticing write access to the
// decision. A nil ledger or non-proceed reason passes through unchanged.
func escalateRetroReason(reason, pattern string, led *recurrence.Ledger) string {
	if led == nil || !strings.HasPrefix(reason, "proceed:") {
		return reason
	}
	// Generic classification noise (operator-reset/loop-fatal echo) must never
	// force-escalate — its count is a corpus artifact, not a recurring defect.
	if led.IsGenericPattern(pattern) {
		return reason
	}
	if n := led.Count(pattern); n >= 2 {
		return fmt.Sprintf("adapt: escalated %s to Nth-occurrence (count=%d); %s",
			pattern, n, strings.TrimPrefix(reason, "proceed: "))
	}
	return reason
}

// escalateRetroReasonForHistory loads the recurrence ledger at projectRoot and
// applies escalateRetroReason to the most-recent failing lesson pattern. It is
// the live-loop consult site: RetroDecision routes its reason through here so a
// count>=2 pattern cannot be recorded as a bare "proceed". Best-effort — an
// empty root, non-proceed reason, empty history, or unreadable ledger returns
// reason unchanged (the deterministic branch still stands).
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
