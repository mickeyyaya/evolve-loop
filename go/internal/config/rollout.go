package config

// rollout.go — RolloutStages: the subsystem-migration dials embedded in
// RoutingConfig, each documenting its own ladder and provenance. Data only.

// RolloutStages groups the three independent rollout-axis dials, each gating a
// subsystem's Off→Shadow→Enforce migration. They share no logic, but all three
// are composition-root *views* of an env-driven signal the subprocess reads
// directly. Embedded anonymously in RoutingConfig, so every cfg.CommitEvidence
// / cfg.ReviewGate / cfg.SandboxMode access is unchanged via field promotion.
type RolloutStages struct {
	// CommitEvidence is the ADR-0027 commit-as-evidence rollout stage:
	// StageOff (legacy path-poll, byte-identical), StageShadow (git-evidence
	// computed + logged, artifact authoritative), StageEnforce (git-evidence
	// authoritative, phases commit, kernel relaxed). StageAdvisory is not used
	// for this axis. The bridge driver reads EVOLVE_COMMIT_EVIDENCE from env
	// directly (it is a subprocess); this field is the orchestrator's view.
	CommitEvidence Stage
	// ReviewGate is Workstream E2's per-phase review-gate rollout stage:
	//   StageOff      — orchestrator uses noopReviewer (every non-SKIPPED
	//                   verdict approved). Byte-identical to pre-E2.
	//   StageShadow   — deterministic reviewer runs but is log-only.
	//   StageEnforce  — deterministic + (future) LLM reviewer authoritative;
	//                   reject aborts the cycle.
	// StageAdvisory is not used for this axis (no advisory-intermediate). The
	// orchestrator owns the stage interpretation; this field is the
	// composition-root view, exactly like CommitEvidence.
	ReviewGate Stage
	// EvalGate is the structural inter-phase eval-gate rollout stage
	// (internal/evalgate): StageOff — no eval gates (orchestrator keeps the
	// noopReviewer; byte-identical); StageShadow — Gate A (scout eval-file
	// materialization) + Gate B (tdd predicate-quality) run + log but always
	// approve; StageEnforce — a CERTAIN violation (a stat'd-missing eval file
	// or a definite tautology) aborts the cycle. The gates fail OPEN on any
	// ambiguity, so enforce-default never false-blocks a healthy cycle.
	// Configured through policy.GatesConfig; default StageEnforce.
	EvalGate Stage
	// ContractGate is the deliverable-contract gate rollout stage
	// (internal/deliverable, ADR-0034): StageOff — no contract gate
	// (orchestrator keeps the noopReviewer; byte-identical); StageShadow —
	// the verifier runs but every violation is log-only; StageEnforce — a
	// confirmed well-formedness violation (missing/misplaced/malformed
	// deliverable) rejects the phase. The gate fails OPEN on ambiguity, and a
	// runtime circuit breaker demotes enforce→advisory after N consecutive
	// blocks so a miscalibrated gate cannot brick the loop. Configured through
	// policy.GatesConfig; default StageEnforce.
	ContractGate Stage
	// TriageCapGate is the R9.2 triage capacity clamp rollout stage
	// (internal/triagecap): StageOff — no clamp; StageShadow — overpacked
	// triage logs a would-block but is approved; StageEnforce — committed
	// coverage floors above ceil(1.25·K) (K = observed throughput window)
	// reject the triage deliverable through the correction ladder with a
	// cap directive (inbox coverage-floor-overpacking: three consecutive
	// coverage cycles burned on the same overpacked shape). Fails OPEN on
	// any ambiguity. Configured through policy.GatesConfig; default StageEnforce.
	TriageCapGate Stage
	// TopNGate is the build->audit top_n task-binding gate rollout stage
	// (internal/topngate): StageOff — no gate (noopReviewer; byte-identical);
	// StageShadow — an out-of-lane build (a build-report ## Task: slug outside
	// triage ## top_n) is logged but approved; StageEnforce — a CERTAIN
	// out-of-lane build aborts the cycle at the build->audit transition before
	// audit/ship spend (inbox builder-task-binding-topn-gate, 8th recurrence).
	// Fails OPEN on ambiguity (missing report, empty top_n). Configured through
	// policy.GatesConfig; default StageEnforce.
	TopNGate Stage
	// SandboxMode controls OS-level sandbox wrapping for source-writing phases
	// (Workstream B — cycle-119 cross-CLI trust bypass). Values:
	//   "auto" (default) — wrap when nested-claude is NOT detected and the
	//                       host's sandbox binary (sandbox-exec / bwrap) is
	//                       present; degrade unwrapped otherwise.
	//   "on"             — always wrap when the binary is available; WARN
	//                       loudly (no fallback) when it isn't.
	//   "off"            — never wrap. Operator-only emergency hatch; the
	//                       trust kernel is then Claude-PreToolUse-only.
	//
	// PRECEDENCE NOTE: the bridge subprocess reads EVOLVE_SANDBOX from its
	// own env chain (deps.Env / os.Getenv), which is the actual signal. This
	// field is the COMPOSITION-ROOT view — set from the same env var by
	// applyEnv so operators auditing the loaded config can see the effective
	// mode. Mirrors the CommitEvidence pattern (also env-direct on the
	// subprocess hot path). Setting this field in code without also propagating
	// EVOLVE_SANDBOX into the bridge's env map has no effect.
	SandboxMode string
	// PhaseRecovery is the ADR-0044 Unified Phase Recovery rollout stage —
	// the ONE dial for the whole program (fatal-pane fast-fail, the
	// observer's chain-backed StallPolicy, the orchestrator's failure-advisor
	// hook):
	//   StageOff     — recovery components inert; byte-identical legacy.
	//   StageShadow  — classify + log would-be actions only (DEFAULT).
	//   StageEnforce — corrective actions execute (fast-fail, stall policy,
	//                  advise+promote). Classification is always-on above off;
	//                  only ACTING is staged.
	// PRECEDENCE NOTE: the bridge and observer subprocesses read
	// EVOLVE_PHASE_RECOVERY from their own env (the actual hot-path signal,
	// same pattern as CommitEvidence/SandboxMode); this field is the
	// composition-root/orchestrator view, set from the same env var by
	// applyEnv. Default StageShadow per ADR-0044 (behavior-neutral first ship).
	PhaseRecovery Stage

	// SpineFloor is the artifact-backed spine floor's OWN rollout dial (the
	// R8.5 flip, 2026-07-16). Split out of PhaseRecovery because that dial is
	// overloaded — ADR-0045 I6 folded the bidirectional channel into it
	// (channel.Enabled: enforce → live producer + per-tick capture) and the
	// failure-adviser promotion hook also keys on it — so arming the floor
	// through PhaseRecovery would have armed two unsoaked subsystems along
	// with it. This dial gates EXACTLY ONE behavior: whether a clean-absence
	// mandatory-predecessor handoff gap ABORTS the cycle (StageEnforce, the
	// default) or WARN-and-proceeds (StageShadow). Degraded reads fail open at
	// every stage. Policy override: `recovery.spine_floor` (no env var).
	SpineFloor Stage

	// PhaseIO is the ADR-0050 Phase-3 unified-phase-I/O rollout dial. Unlike the
	// gate dials above (off/shadow/enforce trichotomy) it uses the FULL
	// off→shadow→advisory→enforce ladder, like the dynamic-routing Stage:
	//   StageOff      — the unified phaseio envelope is dormant; byte-identical
	//                   legacy dispatch (the rollback escape hatch).
	//   StageShadow   — the envelope is assembled and compared against the
	//                   legacy disk reads; mismatches log + ledger only.
	//   StageAdvisory — the envelope is populated and read alongside the legacy
	//                   path (legacy still wins; the two are compared).
	//   StageEnforce  — the typed envelope is authoritative.
	// Default StageEnforce as of the 3.10 cutover (set in defaults()); set
	// EVOLVE_PHASE_IO=off to roll back. A typo falls back to off via parseStage
	// (fail-safe — never leaves the dial in an unintended state).
	PhaseIO Stage

	// RouterReplan is the ADR-0052 advisor-maximization post-scout re-plan
	// rollout dial (WS2). It uses the off→shadow→advisory subset of the Stage
	// ladder (enforce is unused for this axis):
	//   StageOff      — no post-scout re-plan; the upfront plan stands.
	//   StageShadow   — the re-plan is computed + logged (replan-plan.json), but
	//                   the upfront clamped plan still drives (DEFAULT).
	//   StageAdvisory — the re-plan replaces the clamped plan after the floor
	//                   re-clamps it (opt-in, post-soak).
	// PRECEDENCE NOTE: like the other rollout dials this is the composition-root
	// view, loaded from policy; the re-plan call site (WS2-S3) reads it.
	// Default StageShadow (set in defaults()).
	RouterReplan Stage

	// MergeGate is the merge-to-main gate rollout dial. Like RouterReplan/PhaseIO
	// it uses the FULL off→shadow→advisory→enforce ladder:
	//   StageOff      — the gate is never inserted; byte-identical legacy (the
	//                   per-cycle ship path to main is unaffected).
	//   StageShadow   — the gate runs and records its would-be promotion verdict
	//                   to the ledger/dossier, but promotes nothing (DEFAULT).
	//   StageAdvisory — the gate's verdict surfaces as a recommendation; still no
	//                   automatic promotion.
	//   StageEnforce  — on a PASS verdict the kernel auto-promotes the completed
	//                   milestone's integration branch to main through the
	//                   hardened ship/merge-train path (armed auto-rollback).
	// This is the composition-root view, loaded from policy.MergeGateConfig; the
	// promoter call site reads it. Default StageShadow (set in defaults()) — the
	// auto-merge in enforce activates only after a human-watched shadow soak.
	MergeGate Stage
	// ParallelEvaluate is the post-build checking-phase parallelization dial. It
	// uses the off→shadow→enforce trichotomy:
	//   StageOff      — the dispatcher is dormant; phases run sequentially
	//                   (DEFAULT, byte-identical legacy).
	//   StageShadow   — the would-be batch + projected saving is recorded only.
	//   StageEnforce  — the independent post-build evaluate phases (archetype
	//                   "evaluate", excluding the audit verdict-brancher) run
	//                   CONCURRENTLY; a single serialized merge records all
	//                   outcomes (weakest-link verdict). The enforce flip activates
	//                   only after a shadow soak (the ~11% projected fleet saving).
	// Concurrency is ParallelEvaluateConcurrency (RoutingConfig). Composition-root
	// view: cmd_cycle.go calls policy.ParallelEvaluateConfig() and writes both
	// ParallelEvaluate and ParallelEvaluateConcurrency before the runners are built.
	ParallelEvaluate Stage
	// ScoutDecompose is the scout map-reduce dial (off->shadow->enforce, default off):
	// enforce runs N scout-scan workers over codebase slices concurrently and the
	// scout agent synthesizes from their merged digests instead of scanning itself.
	// Concurrency = ScoutDecomposeConcurrency. Flip to enforce only after a shadow soak.
	ScoutDecompose Stage
}
