// Package policy is the user-controlled rule layer that bounds the autonomous
// pipeline: which phases the routing advisor may NOT drop, and the hard
// per-phase CLI/model pins the dispatch resolver MUST honor.
//
// It is loaded from a single user-owned file (.evolve/policy.json) — distinct
// from the per-agent profiles (which are defaults the advisor/operator may
// vary). Policy is the TOP authority: a pin overrides even an operator's
// EVOLVE_<AGENT>_CLI/_MODEL env override (escape hatch: EVOLVE_POLICY_BYPASS=1),
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

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// Pin is a user-authored hard dispatch pin for a phase: an exact CLI and/or
// model the resolver MUST use. An empty field means "no pin for that
// dimension" (the resolver's normal precedence applies to it).
type Pin struct {
	CLI   string `json:"cli,omitempty"`
	Model string `json:"model,omitempty"`
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
	// FailureFloor is the ONE user surface for failure-learning policy
	// (failure floor Phase 4a). It tunes the LLM-learning layer only —
	// the deterministic floor (FailedRecord + retrospective/lesson
	// artifacts on every abnormal termination) is NON-configurable, like
	// the integrity floor.
	FailureFloor *FailureFloor `json:"failure_floor,omitempty"`
	// GC is the declarative retention policy for the .evolve data tree
	// (L3.1). The schema lives in internal/gc (a stdlib-only leaf this
	// package may import without weight); absent ⇒ gc defaults. The hard
	// rules — quarantine manual-only, ledger never touched, live runs never
	// touched — are NOT configurable here, by design.
	GC *gc.Policy `json:"gc,omitempty"`
	// Fanout configures the fan-out dispatch subsystem. Absent ⇒ built-in
	// defaults apply (concurrency=2, track_workers=true, cache_prefix=true).
	Fanout *FanoutPolicy `json:"fanout,omitempty"`
	// Observer configures phase liveness observation and watchdog behavior.
	// Absent ⇒ built-in defaults apply.
	Observer *ObserverPolicy `json:"observer,omitempty"`
	// Bridge configures operator-writable bridge override directories.
	// Absent ⇒ each bridge subsystem uses its built-in .evolve directory.
	Bridge *BridgePolicy `json:"bridge,omitempty"`
	// QuotaReset configures the quota-reset wake-time estimator. Absent ⇒
	// built-in defaults apply (DefaultHours=5.4167, no ResetAt override).
	QuotaReset *QuotaResetConfig `json:"quota_reset,omitempty"`
	// Dispatch configures the loop dispatch verification policy. Absent ⇒
	// built-in defaults apply (Policy="verify", RepeatThreshold=5).
	Dispatch *DispatchConfig `json:"dispatch,omitempty"`
	// Workflow configures loop and subagent workflow defaults. Absent ⇒
	// built-in defaults apply.
	Workflow *WorkflowPolicy `json:"workflow,omitempty"`
	// Retry configures phase retry, backoff, correction, and latency defaults.
	// Absent ⇒ built-in defaults apply.
	Retry *RetryPolicy `json:"retry,omitempty"`
	// Swarm configures swarm dispatch stage and port allocation. Absent ⇒
	// built-in defaults apply (Stage="shadow", PortBase=0).
	Swarm *SwarmPolicy `json:"swarm,omitempty"`
	// Gates configures persistent rollout stages for the contract, eval,
	// triage-cap, and review gates. Absent ⇒ built-in defaults apply.
	Gates *GatesPolicy `json:"gates,omitempty"`
	// Router configures advisor routing behavior and per-decision model
	// overrides. Absent ⇒ built-in defaults apply.
	Router *RouterPolicy `json:"router,omitempty"`
}

// FailureFloor configures the failure-learning policy surface.
type FailureFloor struct {
	// AlwaysLearn=false tunes LLM-retro richness down (memo-weight
	// learning after an audit FAIL); it can NEVER suppress the
	// deterministic floor. Absent ⇒ true.
	AlwaysLearn *bool `json:"always_learn,omitempty"`
	// AuditFailRoutesTo picks the learning phase after an audit FAIL:
	// "retrospective" (default) or "memo". Unknown values fall back to
	// the default — the floor guarantees SOME learning phase routes.
	AuditFailRoutesTo string `json:"audit_fail_routes_to,omitempty"`
}

// FailurePolicy resolves the failure_floor config with defaults applied:
// (true, "retrospective") for an absent/partial block; unknown route
// values fall back to the default. Pure.
func (p Policy) FailurePolicy() (alwaysLearn bool, auditFailRoutesTo string) {
	alwaysLearn, auditFailRoutesTo = true, "retrospective"
	if p.FailureFloor == nil {
		return alwaysLearn, auditFailRoutesTo
	}
	if p.FailureFloor.AlwaysLearn != nil {
		alwaysLearn = *p.FailureFloor.AlwaysLearn
	}
	// Closed vocabulary: unknown values fall back to the default so the floor
	// guarantees SOME learning phase routes regardless of a typo.
	switch p.FailureFloor.AuditFailRoutesTo {
	case "retrospective", "memo":
		auditFailRoutesTo = p.FailureFloor.AuditFailRoutesTo
	}
	return alwaysLearn, auditFailRoutesTo
}

// evaluatorFloorPhase is the single non-removable floor phase: a plan can never
// reach ship without an evaluator. Kept here (not router) because the
// non-removability is a policy-layer guarantee. Mirrors router.EvaluatorFloorPhase
// (each layer independently guarantees the evaluator — defense in depth; a single
// shared const would create an import cycle). Divergence trips
// router's TestEvaluatorFloorPhase_SingleSource.
const evaluatorFloorPhase = "audit"

// FloorPhases resolves the configured ship-floor. It returns (floor, overridden):
// when overridden is false the caller MUST fall back to the router's structural
// default (this keeps the default floor's definition in one place — the router —
// rather than duplicating {tdd,build,audit} here). When overridden is true the
// returned floor is the user's list with the non-removable evaluator phase
// guaranteed present (appended last if absent). Pure; never mutates the receiver.
func (p Policy) FloorPhases() (floor []string, overridden bool) {
	if len(p.ShipFloor) == 0 {
		return nil, false
	}
	out := append([]string(nil), p.ShipFloor...)
	if !contains(out, evaluatorFloorPhase) {
		out = append(out, evaluatorFloorPhase)
	}
	return out, true
}

// MergeMandatory returns base plus any phase in MandatoryPhases not already
// present, preserving order. ADDITIVE — policy can only ADD mandatory phases,
// never remove them from the configured spine (and the non-configurable
// integrity floor applies on top regardless). This is the single merge used at
// EVERY config-load site (the autonomous loop's composition root AND the
// per-phase router.PolicyForProject) so a policy-mandatory phase is honored
// uniformly, including by self-skipping phases.
func (p Policy) MergeMandatory(base []string) []string {
	if len(p.MandatoryPhases) == 0 {
		return base
	}
	seen := make(map[string]struct{}, len(base))
	for _, ph := range base {
		seen[ph] = struct{}{}
	}
	out := append([]string(nil), base...)
	for _, ph := range p.MandatoryPhases {
		if ph == "" {
			continue
		}
		if _, ok := seen[ph]; ok {
			continue
		}
		seen[ph] = struct{}{}
		out = append(out, ph)
	}
	return out
}

// PinFor returns the pin for phase and whether a non-empty one exists.
func (p Policy) PinFor(phase string) (Pin, bool) {
	pin, ok := p.Pins[phase]
	if !ok || (pin.CLI == "" && pin.Model == "") {
		return Pin{}, false
	}
	return pin, true
}

// Load reads policy.json at path. An ABSENT file is not an error — policy is
// optional and an empty Policy means "no user rules" (advisor + resolver use
// their built-in defaults). A present-but-malformed file IS an error: a
// typo'd rule must fail loudly rather than silently disabling the user's
// policy (a silent-fallback here would defeat the whole point of a guardrail).
func Load(path string) (Policy, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Policy{}, nil
	}
	if err != nil {
		return Policy{}, fmt.Errorf("policy: read %s: %w", path, err)
	}
	var p Policy
	if err := json.Unmarshal(raw, &p); err != nil {
		return Policy{}, fmt.Errorf("policy: parse %s: %w", path, err)
	}
	return p, nil
}

// ValidatePin checks a pin against a phase profile's guardrails and returns a
// non-nil error describing the first breach (CLI family outside allowed_clis,
// or model tier outside the envelope). A nil profile or nil constraint means
// "nothing to validate" → ok. Used at load time so an out-of-bounds policy
// fails loudly before any dispatch.
func ValidatePin(phase string, pin Pin, prof *profiles.Profile) error {
	if prof == nil {
		return nil
	}
	if pin.CLI != "" && len(prof.AllowedCLIs) > 0 &&
		!contains(prof.AllowedCLIs, "all") && !contains(prof.AllowedCLIs, baseCLI(pin.CLI)) {
		return fmt.Errorf("policy: pin for phase %q: cli %q not in allowed_clis %v",
			phase, baseCLI(pin.CLI), prof.AllowedCLIs)
	}
	if pin.Model != "" && prof.ModelTierEnvelope != nil {
		rank := TierRank(pin.Model)
		minR, maxR := TierRank(prof.ModelTierEnvelope.Min), TierRank(prof.ModelTierEnvelope.Max)
		if rank > 0 && minR > 0 && maxR > 0 && (rank < minR || rank > maxR) {
			return fmt.Errorf("policy: pin for phase %q: model %q (tier rank %d) outside envelope [%s..%s]",
				phase, pin.Model, rank, prof.ModelTierEnvelope.Min, prof.ModelTierEnvelope.Max)
		}
	}
	return nil
}

// --- canonical tier/CLI vocabulary (mirror of setup.go; see package doc) ---

// TierRank maps a canonical tier (fast/balanced/deep), a legacy alias
// (haiku/sonnet/opus), or an exact model identifier to 1/2/3; 0 =
// unclassifiable (the envelope check is skipped for rank 0). Exported so
// callers that must REJECT (not exempt) an unclassifiable tier — e.g. the
// phase registrar clamping a minted phase — can detect rank 0 themselves.
func TierRank(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "fast", "haiku":
		return 1
	case "balanced", "sonnet":
		return 2
	case "deep", "opus":
		return 3
	}
	l := strings.ToLower(s)
	switch {
	case strings.Contains(l, "haiku"):
		return 1
	case strings.Contains(l, "sonnet"):
		return 2
	case strings.Contains(l, "opus"):
		return 3
	}
	return 0
}

// baseCLI strips driver suffixes: claude-tmux/claude-p → claude, codex-tmux →
// codex, agy-tmux → agy.
func baseCLI(cli string) string {
	return strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(cli), "-tmux"), "-p")
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// FanoutPolicy configures the fan-out dispatch subsystem. Loaded from
// .evolve/policy.json "fanout" block; absent block ⇒ built-in defaults apply.
// Prefer Policy.FanoutConfig() for default-resolved access.
type FanoutPolicy struct {
	// Concurrency is the max parallel workers in flight. 0/absent ⇒ 2.
	Concurrency int `json:"concurrency,omitempty"`
	// TimeoutSecs is the per-worker timeout. 0/absent ⇒ fanoutdispatch built-in.
	TimeoutSecs int `json:"timeout_secs,omitempty"`
	// CancelOnConsensus cancels remaining workers when ConsensusK voters agree.
	CancelOnConsensus bool `json:"cancel_on_consensus,omitempty"`
	// ConsensusK is the consensus threshold. 0/absent ⇒ fanoutdispatch built-in.
	ConsensusK int `json:"consensus_k,omitempty"`
	// ConsensusPollSecs is the poll interval. 0/absent ⇒ fanoutdispatch built-in.
	ConsensusPollSecs int `json:"consensus_poll_secs,omitempty"`
	// TrackWorkers tracks active fanout worker PIDs. Nil/absent ⇒ true.
	TrackWorkers *bool `json:"track_workers,omitempty"`
	// CachePrefixEnabled writes shared cache-prefix.md for siblings. Nil/absent ⇒ true.
	CachePrefixEnabled *bool `json:"cache_prefix_enabled,omitempty"`
	// TestExecutor overrides the fanout worker command for test harnesses.
	TestExecutor string `json:"test_executor,omitempty"`
}

// FanoutConfig returns a FanoutPolicy with all defaults resolved. Concurrency
// defaults to 2 (min 1); TrackWorkers and CachePrefixEnabled default to true
// (returned pointers are never nil). Int fields use 0 as the fanoutdispatch
// built-in-default sentinel.
func (p Policy) FanoutConfig() FanoutPolicy {
	tw, cp := true, true
	out := FanoutPolicy{
		Concurrency:        2,
		TrackWorkers:       &tw,
		CachePrefixEnabled: &cp,
	}
	f := p.Fanout
	if f == nil {
		return out
	}
	if f.Concurrency >= 1 {
		out.Concurrency = f.Concurrency
	}
	out.TimeoutSecs = f.TimeoutSecs
	out.CancelOnConsensus = f.CancelOnConsensus
	out.ConsensusK = f.ConsensusK
	out.ConsensusPollSecs = f.ConsensusPollSecs
	if f.TrackWorkers != nil {
		out.TrackWorkers = f.TrackWorkers
	}
	if f.CachePrefixEnabled != nil {
		out.CachePrefixEnabled = f.CachePrefixEnabled
	}
	out.TestExecutor = f.TestExecutor
	return out
}

// ObserverPolicy configures phase observation and inactivity watchdogs.
// Pointer fields preserve the distinction between an omitted value and an
// explicit zero/false override (for example nudge_s=0 disables nudging).
type ObserverPolicy struct {
	Autospawn        *bool  `json:"autospawn,omitempty"`
	PollS            *int   `json:"poll_s,omitempty"`
	StallS           *int   `json:"stall_s,omitempty"`
	NudgeS           *int   `json:"nudge_s,omitempty"`
	NudgeBody        string `json:"nudge_body,omitempty"`
	EOFGraceS        int    `json:"eof_grace_s,omitempty"`
	WatchdogPollS    *int   `json:"watchdog_poll_s,omitempty"`
	WatchdogWarnPct  *int   `json:"watchdog_warn_pct,omitempty"`
	WatchdogGraceS   *int   `json:"watchdog_grace_s,omitempty"`
	WatchdogDisabled bool   `json:"watchdog_disabled,omitempty"`
}

// ObserverConfig returns an ObserverPolicy with all defaults resolved.
// Returned pointer fields are always non-nil.
func (p Policy) ObserverConfig() ObserverPolicy {
	autospawn, pollS, stallS, nudgeS := true, 5, 600, 300
	watchdogPollS, watchdogWarnPct, watchdogGraceS := 15, 75, 10
	out := ObserverPolicy{
		Autospawn:       &autospawn,
		PollS:           &pollS,
		StallS:          &stallS,
		NudgeS:          &nudgeS,
		WatchdogPollS:   &watchdogPollS,
		WatchdogWarnPct: &watchdogWarnPct,
		WatchdogGraceS:  &watchdogGraceS,
	}
	if p.Observer == nil {
		return out
	}
	o := p.Observer
	if o.Autospawn != nil {
		out.Autospawn = o.Autospawn
	}
	if o.PollS != nil {
		out.PollS = o.PollS
	}
	if o.StallS != nil {
		out.StallS = o.StallS
	}
	if o.NudgeS != nil {
		out.NudgeS = o.NudgeS
	}
	out.NudgeBody = o.NudgeBody
	out.EOFGraceS = o.EOFGraceS
	if o.WatchdogPollS != nil {
		out.WatchdogPollS = o.WatchdogPollS
	}
	if o.WatchdogWarnPct != nil {
		out.WatchdogWarnPct = o.WatchdogWarnPct
	}
	if o.WatchdogGraceS != nil {
		out.WatchdogGraceS = o.WatchdogGraceS
	}
	out.WatchdogDisabled = o.WatchdogDisabled
	return out
}

// BridgePolicy configures operator-writable bridge override directories and
// timing overrides. Empty string fields preserve each subsystem's built-in
// .evolve directory. Zero int fields mean "use the bridge package built-in
// default" (the bridge's defaultIfZero helper handles the zero sentinel).
type BridgePolicy struct {
	ManifestDir string `json:"manifest_dir,omitempty"`
	CatalogDir  string `json:"catalog_dir,omitempty"`
	RecipeDir   string `json:"recipe_dir,omitempty"`
	// Timing overrides (seconds). 0 = use bridge built-in default.
	BootTimeoutS       int `json:"boot_timeout_s,omitempty"`
	ArtifactTimeoutS   int `json:"artifact_timeout_s,omitempty"`
	ArtifactMaxExtends int `json:"artifact_max_extends,omitempty"`
	ScrollbackLines    int `json:"scrollback_lines,omitempty"`
}

// BridgeConfig returns the configured bridge policy. Zero int fields mean
// "use bridge built-in defaults"; the bridge package resolves them via
// defaultIfZero.
func (p Policy) BridgeConfig() BridgePolicy {
	if p.Bridge == nil {
		return BridgePolicy{}
	}
	return *p.Bridge
}

// QuotaResetConfig configures the quota-reset wake-time estimator (quotareset package).
// Replaces the EVOLVE_QUOTA_RESET_AT and EVOLVE_QUOTA_RESET_HOURS env reads.
type QuotaResetConfig struct {
	// ResetAt is an operator-supplied ISO 8601 wake-time override. Empty = no override.
	ResetAt string `json:"reset_at,omitempty"`
	// DefaultHours is the fallback wake duration when no override or hint file
	// is present. Zero = use built-in default (5.4167 ≈ 5h25min).
	DefaultHours float64 `json:"default_hours,omitempty"`
}

// QuotaResetConfig returns a QuotaResetConfig with defaults resolved.
// When absent from policy.json the zero value means "use quotareset built-in defaults".
func (p Policy) QuotaResetConfig() QuotaResetConfig {
	if p.QuotaReset == nil {
		return QuotaResetConfig{}
	}
	return *p.QuotaReset
}

// DispatchConfig configures the loop dispatch verification policy and circuit-breaker.
// Replaces EVOLVE_DISPATCH_POLICY and EVOLVE_DISPATCH_REPEAT_THRESHOLD env reads.
type DispatchConfig struct {
	// Policy selects dispatch verification: "off" / "verify" (default) / "stop".
	Policy string `json:"policy,omitempty"`
	// RepeatThreshold is the same-cycle repeat count that trips the circuit-breaker.
	// Zero / absent ⇒ built-in default (5).
	RepeatThreshold int `json:"repeat_threshold,omitempty"`
}

const defaultDispatchRepeatThreshold = 5

// DispatchConfig returns a DispatchConfig with defaults resolved.
func (p Policy) DispatchConfig() DispatchConfig {
	c := DispatchConfig{Policy: "verify", RepeatThreshold: defaultDispatchRepeatThreshold}
	if p.Dispatch == nil {
		return c
	}
	if p.Dispatch.Policy != "" {
		c.Policy = p.Dispatch.Policy
	}
	if p.Dispatch.RepeatThreshold > 0 {
		c.RepeatThreshold = p.Dispatch.RepeatThreshold
	}
	return c
}

// WorkflowPolicy is the .evolve/policy.json "workflow" block.
type WorkflowPolicy struct {
	MaxConsecutiveFails   int               `json:"max_consecutive_fails,omitempty"`
	MaxCyclesCap          int               `json:"max_cycles_cap,omitempty"`
	AutoPrune             *bool             `json:"auto_prune,omitempty"`
	BackfillEnabled       *bool             `json:"backfill_enabled,omitempty"`
	CycleBudget           string            `json:"cycle_budget,omitempty"`
	AllowDeepResearch     bool              `json:"allow_deep_research,omitempty"`
	AllowDocDelete        bool              `json:"allow_doc_delete,omitempty"`
	DiffComplexityDisable bool              `json:"diff_complexity_disable,omitempty"`
	AuditorTierOverride   string            `json:"auditor_tier_override,omitempty"`
	PhaseEnables          map[string]string `json:"phase_enables,omitempty"`
	ConsensusAuditEnabled *bool             `json:"consensus_audit_enabled,omitempty"`
	// PSMASEnabled enables the Phase Scheduling and Management Advisor
	// Subsystem. Absent/false = disabled (opt-in). Replaces EVOLVE_PSMAS_SKIP.
	PSMASEnabled *bool `json:"psmas_enabled,omitempty"`
}

// WorkflowConfig is the resolved workflow configuration with defaults applied.
type WorkflowConfig struct {
	MaxConsecutiveFails   int
	MaxCyclesCap          int
	AutoPrune             bool
	BackfillEnabled       bool
	CycleBudget           string
	AllowDeepResearch     bool
	AllowDocDelete        bool
	DiffComplexityDisable bool
	AuditorTierOverride   string
	PhaseEnables          map[string]string
	ConsensusAuditEnabled bool
	PSMASEnabled          bool
}

// WorkflowConfig returns workflow configuration with built-in defaults resolved.
func (p Policy) WorkflowConfig() WorkflowConfig {
	c := WorkflowConfig{
		MaxConsecutiveFails:   1,
		MaxCyclesCap:          25,
		AutoPrune:             true,
		BackfillEnabled:       true,
		ConsensusAuditEnabled: true,
	}
	if p.Workflow == nil {
		return c
	}
	if p.Workflow.MaxConsecutiveFails > 0 {
		c.MaxConsecutiveFails = p.Workflow.MaxConsecutiveFails
	}
	if p.Workflow.MaxCyclesCap > 0 {
		c.MaxCyclesCap = p.Workflow.MaxCyclesCap
	}
	if p.Workflow.AutoPrune != nil {
		c.AutoPrune = *p.Workflow.AutoPrune
	}
	if p.Workflow.BackfillEnabled != nil {
		c.BackfillEnabled = *p.Workflow.BackfillEnabled
	}
	c.CycleBudget = p.Workflow.CycleBudget
	c.AllowDeepResearch = p.Workflow.AllowDeepResearch
	c.AllowDocDelete = p.Workflow.AllowDocDelete
	c.DiffComplexityDisable = p.Workflow.DiffComplexityDisable
	c.AuditorTierOverride = p.Workflow.AuditorTierOverride
	c.PhaseEnables = p.Workflow.PhaseEnables
	if p.Workflow.ConsensusAuditEnabled != nil {
		c.ConsensusAuditEnabled = *p.Workflow.ConsensusAuditEnabled
	}
	if p.Workflow.PSMASEnabled != nil {
		c.PSMASEnabled = *p.Workflow.PSMASEnabled
	}
	return c
}

// RetryPolicy is the .evolve/policy.json "retry" block.
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

// RouterPolicy is the .evolve/policy.json "router" block.
type RouterPolicy struct {
	RouterReplan string `json:"router_replan,omitempty"`
	RoutingJudge bool   `json:"routing_judge,omitempty"`
	ReconDigest  bool   `json:"recon_digest,omitempty"`
	ReplanDepth  int    `json:"replan_depth,omitempty"`
	PlanModel    string `json:"plan_model,omitempty"`
	ProposeModel string `json:"propose_model,omitempty"`
	CLI          string `json:"cli,omitempty"`
	Model        string `json:"model,omitempty"`
}

// RouterConfig returns router configuration with built-in defaults resolved.
func (p Policy) RouterConfig() RouterPolicy {
	c := RouterPolicy{RouterReplan: "shadow", ReplanDepth: 1}
	if p.Router == nil {
		return c
	}
	if p.Router.RouterReplan != "" {
		c.RouterReplan = p.Router.RouterReplan
	}
	c.RoutingJudge = p.Router.RoutingJudge
	c.ReconDigest = p.Router.ReconDigest
	if p.Router.ReplanDepth > 0 {
		c.ReplanDepth = p.Router.ReplanDepth
	}
	c.PlanModel = p.Router.PlanModel
	c.ProposeModel = p.Router.ProposeModel
	c.CLI = p.Router.CLI
	c.Model = p.Router.Model
	return c
}

// GatesPolicy is the .evolve/policy.json "gates" block.
type GatesPolicy struct {
	ContractGate  string `json:"contract_gate,omitempty"`
	EvalGate      string `json:"eval_gate,omitempty"`
	TriageCapGate string `json:"triage_cap_gate,omitempty"`
	ReviewGate    string `json:"review_gate,omitempty"`
}

// GatesConfig is the resolved gate configuration with defaults applied.
type GatesConfig struct {
	ContractGate  string
	EvalGate      string
	TriageCapGate string
	ReviewGate    string
}

// GatesConfig returns persistent gate stages with built-in defaults resolved.
func (p Policy) GatesConfig() GatesConfig {
	c := GatesConfig{
		ContractGate:  "enforce",
		EvalGate:      "enforce",
		TriageCapGate: "enforce",
		ReviewGate:    "off",
	}
	if p.Gates == nil {
		return c
	}
	if p.Gates.ContractGate != "" {
		c.ContractGate = p.Gates.ContractGate
	}
	if p.Gates.EvalGate != "" {
		c.EvalGate = p.Gates.EvalGate
	}
	if p.Gates.TriageCapGate != "" {
		c.TriageCapGate = p.Gates.TriageCapGate
	}
	if p.Gates.ReviewGate != "" {
		c.ReviewGate = p.Gates.ReviewGate
	}
	return c
}

// RetryConfig returns retry configuration with defaults and safety bounds.
func (p Policy) RetryConfig() RetryConfig {
	c := RetryConfig{
		PhaseMaxAttempts:          defaultPhaseMaxAttempts,
		RetryBackoffBaseS:         defaultRetryBackoffBaseS,
		PhaseLatencyCeilingS:      defaultPhaseLatencyCeilingS,
		ContractCorrectionRetries: defaultContractCorrectionRetries,
	}
	if p.Retry == nil {
		return c
	}
	if p.Retry.PhaseMaxAttempts > 0 {
		c.PhaseMaxAttempts = min(p.Retry.PhaseMaxAttempts, maxPhaseMaxAttempts)
	}
	if p.Retry.retryBackoffBaseSSet {
		c.RetryBackoffBaseS = max(p.Retry.RetryBackoffBaseS, 0)
	} else if p.Retry.RetryBackoffBaseS > 0 {
		c.RetryBackoffBaseS = p.Retry.RetryBackoffBaseS
	}
	if p.Retry.PhaseLatencyCeilingS > 0 {
		c.PhaseLatencyCeilingS = p.Retry.PhaseLatencyCeilingS
	}
	if p.Retry.contractCorrectionRetriesSet && p.Retry.ContractCorrectionRetries >= 0 {
		c.ContractCorrectionRetries = min(p.Retry.ContractCorrectionRetries, maxContractCorrectionRetries)
	} else if p.Retry.ContractCorrectionRetries > 0 {
		c.ContractCorrectionRetries = min(p.Retry.ContractCorrectionRetries, maxContractCorrectionRetries)
	}
	return c
}
