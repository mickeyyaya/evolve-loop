// Package policy is the user-controlled rule layer that bounds the autonomous
// pipeline: which phases the routing advisor may NOT drop, and the hard
// per-phase CLI/model pins the dispatch resolver MUST honor.
//
// It is loaded from a single user-owned file (.evolve/policy.json) — distinct
// from the per-agent profiles (which are defaults the advisor/operator may
// vary). Policy is the TOP authority: a pin overrides even an operator's
// EVOLVE_<AGENT>_CLI/_MODEL env override (escape hatch: --bypass-policy flag),
// and a pin is validated to stay WITHIN the phase profile's guardrails
// (allowed_clis + model_tier_envelope) so policy cannot silently breach the
// trust-kernel constraints.
//
// Layering: imports profiles + the stdlib-only gc leaf (for the gc schema),
// so the dispatch resolver (llmroute) and the advisor can consult it without
// a heavy dependency. The tier/CLI
// vocabulary helpers below mirror setup.go's canonical versions (the same
// accepted "mirror of" pattern llmroute uses for bridge exit codes); a future
// refactor could extract a shared modeltier vocab package to de-duplicate.
package policy

import "github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"

// Pin is a user-authored hard dispatch pin for a phase: an exact CLI and/or
// model the resolver MUST use. An empty field means "no pin for that
// dimension" (the resolver's normal precedence applies to it).
type Pin struct {
	CLI   string `json:"cli,omitempty"`
	Model string `json:"model,omitempty"`
}

// FloorGate is one entry in the policy `floor` array (ADR-0055 D3): a named
// closeout gate every completed cycle must satisfy before a batch is considered
// clean. The canonical entry is "dossier-closeout" — every cycle must write a
// dossier to knowledge-base/cycles/cycle-N.json, enforced by `evolve dossier
// verify`. NOTE: this is the closeout-gate array (`floor`), distinct from
// ShipFloor (`ship_floor`, the per-plan integrity floor of PHASES). Before the
// 2026-06-22 doc↔impl audit the `floor` key was present in the checked-in
// policy.json but had NO struct field, so json.Unmarshal silently dropped it and
// the gate it declared enforced nothing.
type FloorGate struct {
	ID                 string `json:"id"`
	Description        string `json:"description,omitempty"`
	EnforcedSinceCycle int    `json:"enforced_since_cycle,omitempty"`
}

// Policy is the user-controlled rule set from .evolve/policy.json.
type Policy struct {
	// MandatoryPhases are phases the routing advisor may never drop from a
	// cycle. Merged into the orchestrator's mandatory set (the non-configurable
	// integrity floor — ship ⇒ build ∧ audit — still applies on top).
	MandatoryPhases []string `json:"mandatory_phases,omitempty"`
	// Pins maps a phase name (e.g. "audit") to its hard CLI/model pin.
	Pins map[string]Pin `json:"pins,omitempty"`
	// ShipFloor is the user-configurable integrity floor: the phases a plan
	// reaching ship MUST run. ABSENT/empty ⇒ the orchestrator uses the router's
	// safe structural default ({tdd, build, audit}); a present list is an
	// override (e.g. ["audit"] for an audit-only posture). "audit" is the one
	// non-removable gate — FloorPhases re-appends it if a supplied floor omits
	// it, so policy can never (even by typo) produce a floor without an
	// evaluator. This is the ONLY hard product invariant in this layer.
	ShipFloor []string `json:"ship_floor,omitempty"`
	// Floor is the closeout-gate array (ADR-0055 D3): named gates every
	// completed cycle must satisfy (e.g. "dossier-closeout"). Distinct from
	// ShipFloor above (which lists PHASES). Absent ⇒ no closeout gates. Read by
	// `evolve dossier verify` to decide whether a missing dossier fails the
	// batch. (See FloorGate for the Potemkin-enforcement bug this field fixes.)
	Floor []FloorGate `json:"floor,omitempty"`
	// FailureFloor is the ONE user surface for failure-learning policy
	// (failure floor Phase 4a). It tunes the LLM-learning layer only —
	// the deterministic floor (FailedRecord + retrospective/lesson
	// artifacts on every abnormal termination) is NON-configurable, like
	// the integrity floor.
	FailureFloor *FailureFloor `json:"failure_floor,omitempty"`
	// FailurePolicy is the declarative system-failure DECISION policy
	// (ADR-0072): the category→action mapping the orchestrator classifies
	// against, the non-negotiable Go-enforced floor categories
	// (verdict-incoherence, infra-systemic — always halt), and the
	// non-progress/retry thresholds. Distinct from FailureFloor (which tunes
	// the LLM-learning layer) and Classify (the hang reclassifier). Absent ⇒
	// compiled defaults (DefaultFailurePolicy). See policy_failure.go.
	SystemFailurePolicy *SystemFailurePolicy `json:"failure_policy,omitempty"`
	// GC is the declarative retention policy for the .evolve data tree
	// (L3.1). The schema lives in internal/gc (a stdlib-only leaf this
	// package may import without weight); absent ⇒ gc defaults. The hard
	// rules — quarantine manual-only, ledger never touched, live runs never
	// touched — are NOT configurable here, by design.
	GC *gcpolicy.Policy `json:"gc,omitempty"`
	// Fanout configures the fan-out dispatch subsystem. Absent ⇒ built-in
	// defaults apply (concurrency=2, track_workers=true, cache_prefix=true).
	Fanout *FanoutPolicy `json:"fanout,omitempty"`
	// Observer configures phase liveness observation and watchdog behavior.
	// Absent ⇒ built-in defaults apply.
	Observer *ObserverPolicy `json:"observer,omitempty"`

	// Boot configures boot-time self-heals (binary staleness refresh).
	// Absent ⇒ compiled defaults (binary_refresh=auto).
	Boot *BootPolicy `json:"boot,omitempty"`
	// CIWatch configures the post-push GitHub CI watch and the release
	// preflight CI hard-gate. Absent ⇒ built-in defaults apply (enabled,
	// 900s timeout, 30s poll). See policy_ciwatch.go.
	CIWatch *CIWatchPolicy `json:"ci_watch,omitempty"`
	// Bridge configures operator-writable bridge override directories.
	// Absent ⇒ each bridge subsystem uses its built-in .evolve directory.
	Bridge *BridgePolicy `json:"bridge,omitempty"`
	// QuotaReset configures the quota-reset wake-time estimator. Absent ⇒
	// built-in defaults apply (DefaultHours=5.4167, no ResetAt override).
	QuotaReset *QuotaResetConfig `json:"quota_reset,omitempty"`
	// Dispatch configures the loop dispatch verification policy. Absent ⇒
	// built-in defaults apply (Policy="verify", RepeatThreshold=5).
	Dispatch *DispatchConfig `json:"dispatch,omitempty"`
	// CLIHealth configures the CLI-health subsystem (proactive per-cycle usage
	// probe). Absent ⇒ ProactiveProbe=false (opt-in: the probe is dormant until
	// an operator enables it).
	CLIHealth *CLIHealthConfig `json:"cli_health,omitempty"`
	// Workflow configures loop and subagent workflow defaults. Absent ⇒
	// built-in defaults apply.
	Workflow *WorkflowPolicy `json:"workflow,omitempty"`
	// Retry configures phase retry, backoff, correction, and latency defaults.
	// Absent ⇒ built-in defaults apply.
	Retry *RetryPolicy `json:"retry,omitempty"`
	// Swarm configures swarm dispatch stage and port allocation. Absent ⇒
	// built-in defaults apply (Stage="shadow", PortBase=0).
	Swarm *SwarmPolicy `json:"swarm,omitempty"`
	// FailureDisposition configures the failure-disposition boundary applier
	// (stage + escalation formula). Absent ⇒ built-in defaults apply
	// (stage = chronicle.escalation, threshold=2, step=0.03, cap=0.99).
	FailureDisposition *FailureDispositionPolicy `json:"failure_disposition,omitempty"`
	// Gates configures persistent rollout stages for the contract, eval,
	// triage-cap, and review gates. Absent ⇒ built-in defaults apply.
	Gates *GatesPolicy `json:"gates,omitempty"`
	// Chronicle configures the recurrence-chronicle stages (digest, escalation,
	// historian) and digest budgets. Absent ⇒ built-in defaults apply
	// (digest=shadow, digest_tokens=1200, digest_cycles=10, escalation=shadow,
	// historian=off). See policy_chronicle.go.
	Chronicle *ChroniclePolicy `json:"chronicle,omitempty"`
	// Router configures advisor routing behavior and per-decision model
	// overrides. Absent ⇒ built-in defaults apply.
	Router *RouterPolicy `json:"router,omitempty"`
	// ReportBudget configures the report-size gate's per-artifact token budgets
	// (cycle-565 Slice S1). Absent ⇒ built-in defaults apply (HandoffTokens=2000).
	ReportBudget *ReportBudgetPolicy `json:"report_budget,omitempty"`
	// RetroAutofile configures the retro→inbox preventive-actions autofiler
	// (internal/retrofile, cycle-657). Absent ⇒ built-in safe default applies
	// (DefaultWeight=0.75).
	RetroAutofile *RetroAutofilePolicy `json:"retro_autofile,omitempty"`
	// MergeGate configures the merge-to-main gate: its rollout stage and the
	// cadence-scaling thresholds the advisor reads to decide when a completed
	// milestone is promoted to main. Absent ⇒ built-in defaults apply
	// (stage="shadow" — byte-neutral; the gate records its would-be verdict
	// but promotes nothing).
	MergeGate *MergeGatePolicy `json:"merge_gate,omitempty"`
	// ParallelEvaluate configures post-build evaluate-phase parallelization.
	// Absent ⇒ built-in defaults apply (stage="off" — dispatcher dormant,
	// byte-identical to pre-T1 baseline; concurrency=3, the soak sweet spot).
	ParallelEvaluate *ParallelEvaluatePolicy `json:"parallel_evaluate,omitempty"`
	// ContextFill configures the context-fill WARN threshold. Absent ⇒
	// built-in default applies (warn_threshold_pct=60).
	ContextFill *ContextFillPolicy `json:"context_fill,omitempty"`
	// Research configures the KB recall bound and the failure-lesson novelty
	// gate. Absent ⇒ built-in defaults apply (recall_k=5, the value
	// research.maxResults has always carried; novelty_threshold=0.9).
	Research *ResearchPolicy `json:"research,omitempty"`
	// RegressionTIA configures test-impact selection over the EGPS Go
	// regression corpus. Absent ⇒ built-in default applies (stage="off" —
	// nothing computed, no artifact, byte-identical audit path).
	RegressionTIA *RegressionTIAPolicy `json:"regression_tia,omitempty"`
	// Classify configures the cycle-failure classifier. Absent ⇒ built-in
	// defaults apply (HangClassifier=false — the exit-transport-hang
	// reclassifier is opt-in).
	Classify *ClassifyPolicy `json:"classify,omitempty"`
	// Catalog configures the model catalog subsystem. Absent ⇒ built-in
	// defaults apply (AutoRefresh=true — the cycle-start live refresh is on).
	Catalog *CatalogPolicy `json:"catalog,omitempty"`
	// Recovery configures the ADR-0044 Unified Phase Recovery rollout stage.
	// Absent ⇒ built-in default applies (PhaseRecovery="shadow" — behavior-neutral).
	Recovery *RecoveryPolicy `json:"recovery,omitempty"`
	// DocsFloor configures the ADR-0077 documentation floor for
	// architecture-labeled changes. Absent ⇒ built-in default applies
	// (Stage="enforce" — the floor only WARNs, so arming it by default is
	// byte-neutral to every verdict).
	DocsFloor *DocsFloorPolicy `json:"docs_floor,omitempty"`
	// ACS configures the ACS Go lane timeout. Absent ⇒ built-in defaults apply
	// (DefaultTimeout=60s). Replaces EVOLVE_ACS_GO_TIMEOUT_S env read.
	ACS *ACSConfig `json:"acs,omitempty"`
	// Paths configures path-discovery overrides. Absent ⇒ built-in defaults apply.
	// Replaces EVOLVE_KB_SEARCH_PATHS and EVOLVE_PHASE_ROOTS env reads.
	Paths *PathsConfig `json:"paths,omitempty"`
	// Worktree configures the per-cycle worktree base path. Absent ⇒ built-in
	// default (<root>/.evolve/worktrees). Replaces the EVOLVE_WORKTREE_BASE env
	// read (flag-reduction, ADR-0064): the operator override now flows from this
	// config block, loaded once, rather than a process env dial.
	Worktree *WorktreePolicy `json:"worktree,omitempty"`
	// Integrity configures the binary self-SHA integrity model (ADR-0065).
	// Absent ⇒ Mode="pipeline", Stage="shadow", ProvenanceRequired=true —
	// byte-neutral with the legacy single-pin ship check. Mode="phase" verifies
	// the per-phase agent-block chain; Stage="enforce" blocks (shadow logs only).
	Integrity *IntegrityPolicy `json:"integrity,omitempty"`
	// Sandbox configures the OS-sandbox subsystem. Absent ⇒ built-in default
	// applies (NestedFallback="off" — the verified-fallback write-canary is
	// opt-in; a fresh policy.json never halts a nested run).
	Sandbox *SandboxPolicy `json:"sandbox,omitempty"`
	// Fleet configures multi-cycle fleet planning: lane count, per-lane
	// concurrency, and the todo-source strategy. Absent ⇒ Count=1 —
	// byte-identical to today's single-cycle sequential execution.
	Fleet *FleetPolicy `json:"fleet,omitempty"`
	// Chain configures batch chaining for `evolve loop --until-inbox-empty`
	// (cycle 1075). Absent ⇒ chaining off with the compiled max_batches cap.
	// See policy_chain.go.
	Chain *ChainPolicy `json:"chain,omitempty"`
	// GoalStall configures the goal-stall escalation: after N consecutive
	// empty/blocked (non-shipping) cycles on the SAME goal, the loop stops
	// blindly re-dispatching it and self-files a weighted inbox todo naming the
	// stalled goal. Absent ⇒ Threshold=3 (the compiled-in safe default).
	GoalStall *GoalStallPolicy `json:"goal_stall,omitempty"`
	// ObservationMask configures the deterministic observation-masking window
	// (research-backed #1 token lever): tool observations older than a rolling
	// window of N turns have their bulky payload replaced by a placeholder in
	// downstream phase-agent context. Absent ⇒ WindowTurns=10 (the paper's
	// optimum). No new env flag — config-only.
	ObservationMask *ObservationMaskPolicy `json:"observation_mask,omitempty"`
	// Overlays configures skill-overlay injection: per-dispatch (phase/cli/model/
	// tier) skill bodies composed into an agent prompt above the cycle-context
	// boundary. Absent (nil) ⇒ the compiled default applies ({tiers:[deep,top]}
	// -> [fable]); a non-nil block with an empty Rules slice is an explicit
	// operator opt-out (no overlays at all). Schema + resolver: overlays.go.
	Overlays *OverlaysPolicy `json:"overlays,omitempty"`
}

// FailureFloor configures the failure-learning policy surface.
