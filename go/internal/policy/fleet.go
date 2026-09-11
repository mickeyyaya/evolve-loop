package policy

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

type RetryPolicy struct {
	PhaseMaxAttempts          int `json:"phase_max_attempts,omitempty"`
	RetryBackoffBaseS         int `json:"retry_backoff_base_s,omitempty"`
	PhaseLatencyCeilingS      int `json:"phase_latency_ceiling_s,omitempty"`
	ContractCorrectionRetries int `json:"contract_correction_retries,omitempty"`

	retryBackoffBaseSSet         bool
	contractCorrectionRetriesSet bool
}

// UnmarshalJSON records explicit zero values for the two settings where zero
// disables behavior. Plain struct zero values still mean "use defaults".
func (r *RetryPolicy) UnmarshalJSON(data []byte) error {
	type retryPolicy RetryPolicy
	var decoded retryPolicy
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*r = RetryPolicy(decoded)
	_, r.retryBackoffBaseSSet = fields["retry_backoff_base_s"]
	_, r.contractCorrectionRetriesSet = fields["contract_correction_retries"]
	return nil
}

// RetryConfig is the resolved retry configuration with defaults applied.
type RetryConfig struct {
	PhaseMaxAttempts          int
	RetryBackoffBaseS         int
	PhaseLatencyCeilingS      int
	ContractCorrectionRetries int
}

const (
	defaultPhaseMaxAttempts          = 2
	maxPhaseMaxAttempts              = 5
	defaultRetryBackoffBaseS         = 5
	defaultPhaseLatencyCeilingS      = 900
	defaultContractCorrectionRetries = 2
	maxContractCorrectionRetries     = 5
)

// SwarmPolicy is the .evolve/policy.json "swarm" block.
// Replaces the EVOLVE_SWARM_STAGE and EVOLVE_SWARM_PORT_BASE env reads.
type SwarmPolicy struct {
	// Stage selects the swarm dispatch stage: "off" / "shadow" / "advisory" / "enforce".
	// Empty/absent ⇒ "shadow" (byte-identical delegate to inner runner).
	Stage string `json:"stage,omitempty"`
	// PortBase is the operator override for the writer dev-server port base.
	// Zero/absent ⇒ swarm.DefaultPortBase.
	PortBase int `json:"port_base,omitempty"`
}

// SwarmConfig is the resolved swarm configuration with defaults applied.
type SwarmConfig struct {
	Stage    string
	PortBase int
}

// BootPolicy is the .evolve/policy.json "boot" block.
type BootPolicy struct {
	// BinaryRefresh selects the boot-time binary staleness self-heal
	// (cmd_loop_boot_refresh.go; docs/chronicle/2026-08-binary-lag.md):
	// "auto" (default) rebuilds + re-execs when the running binary's build
	// stamp is behind HEAD with a go/ source delta; "off" disables it for
	// deliberate old-binary pins (incident bisects). Unknown words resolve to
	// "auto" — the self-heal is integrity posture, so a typo must not
	// silently disable it.
	BinaryRefresh string `json:"binary_refresh,omitempty"`
}

// BootBinaryRefresh resolves the boot.binary_refresh stage word: "off" iff
// the operator wrote exactly "off"; everything else (absent block, empty,
// unknown) is "auto".
func (p Policy) BootBinaryRefresh() string {
	if p.Boot != nil && p.Boot.BinaryRefresh == "off" {
		return "off"
	}
	return "auto"
}

// WorktreePolicy is the .evolve/policy.json "worktree" block. Replaces the
// EVOLVE_WORKTREE_BASE env read (flag-reduction, ADR-0064).
type WorktreePolicy struct {
	// Base is the operator override for the per-cycle worktree base directory
	// (e.g. a writable mount when the in-project .evolve/worktrees location is
	// not writable). Empty/absent ⇒ the caller's built-in default applies.
	Base string `json:"base,omitempty"`
}

// WorktreeBase returns the operator override for the per-cycle worktree base
// (policy.json worktree.base), or "" if absent — in which case every reader keeps
// its built-in <root>/.evolve/worktrees default. Unlike SwarmConfig there is no
// resolved-config struct: this is a single scalar with no default to apply (the
// readers own the default), so a bare accessor is the whole surface.
func (p Policy) WorktreeBase() string {
	if p.Worktree == nil {
		return ""
	}
	return p.Worktree.Base
}

// WorktreeBaseFor loads the policy at projectRoot's .evolve/policy.json and
// returns the resolved worktree.base override. Fail-open: a missing OR malformed
// policy yields "" so the pre-batch readiness probe simply selects a default
// writable base; the loud malformed-policy failure still surfaces at the cycle's
// own policy.Load. Lets preflight agree with the orchestrator on the operator
// worktree base without each caller re-implementing the load.
func WorktreeBaseFor(projectRoot string) string {
	pol, err := Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		return ""
	}
	return pol.WorktreeBase()
}

// SwarmConfig returns swarm configuration with built-in defaults resolved.
// Stage defaults to "shadow" — matching the previous swarmStage() default branch
// (empty/unknown → stageOff, i.e. shadow/delegate behavior).
// PortBase defaults to 0 — matching portBaseFromEnv's "unset/invalid → 0" behavior.
func (p Policy) SwarmConfig() SwarmConfig {
	c := SwarmConfig{Stage: "shadow"}
	if p.Swarm == nil {
		return c
	}
	if p.Swarm.Stage != "" {
		c.Stage = p.Swarm.Stage
	}
	c.PortBase = p.Swarm.PortBase
	return c
}

// FleetPolicy is the .evolve/policy.json "fleet" block (S1 of the
// FLEET-AS-POLICY goal). Establishes the configuration surface multi-lane
// fleet planning will read; this cycle ships no behavior change.
type FleetPolicy struct {
	// Count is the number of fleet lanes. Zero/negative/absent ⇒ 1.
	Count int `json:"count,omitempty"`
	// Concurrency is the number of lanes run in parallel. Zero/negative/absent
	// ⇒ follows the resolved Count.
	Concurrency int `json:"concurrency,omitempty"`
	// MinLanes is the operator's capacity floor for the quota-aware wave shrink:
	// the shrink never drops the wave below MinLanes lanes, even when required
	// CLI families are benched. The operator asserts "I have budget for this many
	// concurrent lanes" — e.g. min_lanes=2 keeps 2 lanes on the healthy claude
	// family through a transient codex rate-limit, instead of the blind −1-per-
	// benched-family collapse to sequential. Zero/negative/absent ⇒ 1 (the
	// unconditional min-1 floor, byte-identical to pre-field behaviour). Clamped
	// to ≤ Count (you cannot floor above the configured lane count).
	MinLanes int `json:"min_lanes,omitempty"`
	// PlanSource selects the todo-source strategy: "triage" / "manual".
	// Empty/absent ⇒ "triage". Unlike most closed-vocab blocks in this
	// package, an UNKNOWN value fails safe to "manual" (not the default),
	// per the goal spec — and the getter surfaces a warning rather than
	// logging it, keeping this package I/O-free.
	PlanSource string `json:"plan_source,omitempty"`
	// Scheduling selects the lane-dispatch strategy: "wave" (the wave-barrier
	// Supervisor.Run — today's behaviour) or "pool" (fleet.RunPool, the rolling
	// lane pool that backfills a replacement on any lane exit instead of waiting
	// for the wave barrier). Empty/absent ⇒ "wave". Unlike PlanSource, an UNKNOWN
	// value fails safe to "wave" (NOT the new mode) plus a surfaced warning — an
	// operator typo must never silently escalate into the unsoaked pool scheduler.
	// Mirrors SwarmPolicy.Stage / FleetBudgetPolicy.Stage's shadow-first, config-
	// selected Strategy idiom — not a feature flag.
	Scheduling string `json:"scheduling,omitempty"`
	// Landing selects how PASS fleet lanes reach main: "per-lane" (today's
	// behaviour — each lane independently ff-merges/pushes) or "prefix-queue"
	// (the single-writer prefix composer, fleet.PrefixQueue, owns the sole
	// main-push path and speculates over composed queue prefixes). Empty/absent
	// ⇒ "per-lane". Like Scheduling, an UNKNOWN value fails safe to "per-lane"
	// (NOT the composer) plus a surfaced warning — an operator typo must never
	// silently route landing through the unsoaked composer. Mirrors Scheduling's
	// shadow-first, config-selected Strategy idiom — not a feature flag.
	Landing string `json:"landing,omitempty"`
	// Budget is the OPT-IN quota-driven lane-sizing block (Q4 of the quota
	// budgeting campaign). Absent ⇒ FleetConfig.Budget is nil and the wave
	// never probes quota — zero added latency, byte-identical lanes. Present
	// ⇒ the wave measures each family's quota + the pipeline's pace and sizes
	// against real headroom (fleetbudget.Plan), applied only when
	// Stage=="enforce".
	Budget *FleetBudgetPolicy `json:"budget,omitempty"`
	// StarvationK is the number of consecutive work-supply-starved waves
	// (realized lanes < configured, not a quota shrink) after which the loop
	// self-files a weighted inbox todo naming the starvation cause. Zero/absent
	// ⇒ 3. A positive value overrides.
	StarvationK int `json:"starvation_k,omitempty"`
	// StarvationWeight is the weight of that self-filed todo. Zero/absent ⇒ 0.9;
	// a positive value below the 0.9 floor clamps UP (a starvation signal is
	// never a low-weight afterthought).
	StarvationWeight float64 `json:"starvation_weight,omitempty"`
}

// FleetBudgetPolicy is the .evolve/policy.json "fleet.budget" block: the
// operator's opt-in tunables for quota-driven lane sizing. The sizing is in
// each CLI's NATIVE units (remaining fraction + reset time), never dollars.
type FleetBudgetPolicy struct {
	// Stage selects shadow (compute + log the decision, do NOT resize) vs
	// enforce (apply the computed lane count + pacing). Empty/absent ⇒
	// "shadow"; an unknown value fails safe to "shadow" plus a surfaced
	// warning. Mirrors SwarmPolicy.Stage's shadow-first idiom — a config-
	// selected Strategy, not a feature flag.
	Stage string `json:"stage,omitempty"`
	// CapacityCycles models how many cycles a 100%-quota window affords; with
	// SafetyFraction it is the precondition for the budget-sizing branch
	// (fleetbudget only sizes when both are set). Zero/absent ⇒ no sizing
	// (reset-pace / floor fallback), so a present block with no tunables is a
	// legible reset-pace soak.
	CapacityCycles float64 `json:"capacity_cycles,omitempty"`
	// SafetyFraction is the headroom fraction in (0,1] the budget keeps in
	// reserve. Validity is the fleetbudget allocator's SSOT (it sizes only for
	// 0<safety≤1); this block carries the raw value without duplicating that
	// check.
	SafetyFraction float64 `json:"safety_fraction,omitempty"`
	// HistoryWindow is the number of most-recent cycles rolled up for the pace
	// estimate. Zero/absent ⇒ the built-in default.
	HistoryWindow int `json:"history_window,omitempty"`
}

// defaultFleetBudgetHistoryWindow is the pace-rollup window used when a present
// fleet.budget block omits history_window: enough recent cycles for a stable
// median without reaching back into stale pace regimes.
const defaultFleetBudgetHistoryWindow = 10

// FleetBudgetConfig is the resolved fleet.budget block. It is non-nil on
// FleetConfig only when the operator supplied the block — the caller reads a
// nil Budget as "quota budgeting off, do not probe".
type FleetBudgetConfig struct {
	// Stage is "shadow" (compute + log, never resize) or "enforce" (apply).
	Stage          string
	CapacityCycles float64
	Safety         float64
	HistoryWindow  int
}

// FleetConfig is the resolved fleet configuration with defaults applied.
// Warnings carries non-fatal advisories (e.g. an unknown PlanSource) for the
// caller to log/report; the getter itself performs no I/O.
type FleetConfig struct {
	Count       int
	Concurrency int
	// MinLanes is the resolved quota-shrink floor (≥1, ≤Count). The quota-aware
	// wave shrink never drops below it — the operator's asserted concurrent-lane
	// budget survives a transient CLI-family bench.
	MinLanes   int
	PlanSource string
	// Scheduling is the resolved lane-dispatch strategy: "wave" (default,
	// wave-barrier) or "pool" (rolling lane pool). An unknown raw value resolves
	// to "wave" with a surfaced warning — the new mode is never silently entered.
	Scheduling string
	// Landing is the resolved main-push strategy: "per-lane" (default, each lane
	// lands independently) or "prefix-queue" (the single-writer composer owns the
	// only main-push path). An unknown raw value resolves to "per-lane" with a
	// surfaced warning — the composer is never silently entered.
	Landing string
	// Budget is the resolved quota-budgeting block, or nil when the operator
	// supplied no fleet.budget block (the default — quota budgeting off).
	Budget *FleetBudgetConfig
	// StarvationK is the resolved consecutive-starved-wave fire threshold (≥1,
	// default 3). StarvationWeight is the resolved self-filed-todo weight
	// (≥0.9, default 0.9).
	StarvationK      int
	StarvationWeight float64
	Warnings         []string
}

// defaultStarvationK / defaultStarvationWeight are the compiled fleet-starvation
// defaults, surfaced even on the p.Fleet==nil path (the cycle-542 C542_005
// lesson: seed struct-literal defaults BEFORE the nil early return).
const (
	defaultStarvationK      = 3
	defaultStarvationWeight = 0.9
)

// FleetConfig returns fleet configuration with built-in defaults resolved.
// Count defaults to 1 and clamps any non-positive override back to 1 — a
// fleet block never yields a zero-lane or negative-lane wave. Concurrency
// defaults to the resolved Count when absent/non-positive, independent of
// whether Count itself was overridden. PlanSource defaults to "triage"; an
// unknown value fails safe to "manual" plus a surfaced warning naming the
// rejected value.
func (p Policy) FleetConfig() FleetConfig {
	c := FleetConfig{Count: 1, Concurrency: 1, MinLanes: 1, PlanSource: "triage",
		Scheduling: "wave", Landing: "per-lane", StarvationK: defaultStarvationK, StarvationWeight: defaultStarvationWeight}
	if p.Fleet == nil {
		return c
	}
	if p.Fleet.Count > 0 {
		c.Count = p.Fleet.Count
	}
	c.Concurrency = c.Count
	if p.Fleet.Concurrency > 0 {
		c.Concurrency = p.Fleet.Concurrency
	}
	// MinLanes floor: default 1 (unconditional min-1 shrink). A positive override
	// raises the floor but never above Count — a floor above the lane count is
	// meaningless, so clamp rather than surface a nonsensical wave width.
	if p.Fleet.MinLanes > 1 {
		c.MinLanes = p.Fleet.MinLanes
		if c.MinLanes > c.Count {
			c.MinLanes = c.Count
		}
	}
	switch p.Fleet.PlanSource {
	case "", "triage":
		c.PlanSource = "triage"
	case "manual":
		c.PlanSource = "manual"
	default:
		c.PlanSource = "manual"
		c.Warnings = append(c.Warnings, fmt.Sprintf("fleet.plan_source: unknown value %q, falling back to \"manual\"", p.Fleet.PlanSource))
	}
	// Scheduling: closed vocab, fail-safe to "wave" (NOT the new "pool" mode) —
	// a typo must never silently opt an operator into the unsoaked pool scheduler.
	switch p.Fleet.Scheduling {
	case "", "wave":
		c.Scheduling = "wave"
	case "pool":
		c.Scheduling = "pool"
	default:
		c.Scheduling = "wave"
		c.Warnings = append(c.Warnings, fmt.Sprintf("fleet.scheduling: unknown value %q, falling back to \"wave\"", p.Fleet.Scheduling))
	}
	// Landing: closed vocab, fail-safe to "per-lane" (NOT the composer) — a typo
	// must never silently route the main-push path through the unsoaked composer.
	switch p.Fleet.Landing {
	case "", "per-lane":
		c.Landing = "per-lane"
	case "prefix-queue":
		c.Landing = "prefix-queue"
	default:
		c.Landing = "per-lane"
		c.Warnings = append(c.Warnings, fmt.Sprintf("fleet.landing: unknown value %q, falling back to \"per-lane\"", p.Fleet.Landing))
	}
	if p.Fleet.Budget != nil {
		b := &FleetBudgetConfig{
			Stage:          "shadow",
			CapacityCycles: p.Fleet.Budget.CapacityCycles,
			Safety:         p.Fleet.Budget.SafetyFraction,
			HistoryWindow:  defaultFleetBudgetHistoryWindow,
		}
		switch p.Fleet.Budget.Stage {
		case "enforce":
			b.Stage = "enforce"
		case "", "shadow":
			// b.Stage already defaults to "shadow".
		default:
			// Unknown value fails safe to the "shadow" default (already set) plus a
			// surfaced warning naming the rejected value.
			c.Warnings = append(c.Warnings, fmt.Sprintf("fleet.budget.stage: unknown value %q, falling back to \"shadow\"", p.Fleet.Budget.Stage))
		}
		if p.Fleet.Budget.HistoryWindow > 0 {
			b.HistoryWindow = p.Fleet.Budget.HistoryWindow
		}
		c.Budget = b
	}
	// Starvation-observer tunables: a positive starvation_k overrides the
	// default fire threshold; a positive starvation_weight overrides but clamps
	// UP to the 0.9 floor (never silently under-weighted).
	if p.Fleet.StarvationK > 0 {
		c.StarvationK = p.Fleet.StarvationK
	}
	if p.Fleet.StarvationWeight > 0 {
		c.StarvationWeight = p.Fleet.StarvationWeight
		if c.StarvationWeight < defaultStarvationWeight {
			c.StarvationWeight = defaultStarvationWeight
		}
	}
	return c
}

// defaultObservationMaskWindow is the rolling window (in evictable tool
// observations) kept unmasked by default — the empirical optimum (M=10) from
// the "Complexity Trap" observation-masking result (arXiv 2508.21433).
