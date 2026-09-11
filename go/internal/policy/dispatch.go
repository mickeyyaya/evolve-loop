package policy

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
	// PhaseArtifactTimeoutS overrides the artifact-wait budget for individual
	// phases, keyed on bridge AGENT LABEL. Positive values only; see
	// PhaseArtifactTimeouts for the merge rules. Absent/empty = compiled
	// defaults only.
	PhaseArtifactTimeoutS map[string]int `json:"phase_artifact_timeout_s,omitempty"`
	// AnthropicBaseURL is the operator override for the Anthropic API base URL
	// (proxy mode). Replaces EVOLVE_ANTHROPIC_BASE_URL env read. Empty = no proxy.
	AnthropicBaseURL string `json:"anthropic_base_url,omitempty"`
}

// defaultPhaseArtifactTimeoutS is the compiled per-phase artifact-wait budget
// (seconds), keyed on the bridge AGENT LABEL — the vocabulary
// core.BridgeRequest.Agent carries and Engine.Launch dispatches with.
// internal/phases/retro launches Agent:"retrospective", so a map keyed only on
// the core phase name "retro" would be unit-green and live-dead; both
// vocabularies carry the budget because the phase-name/agent-label skew is
// permanent.
//
// retro=900s: the grown retro contract (report + preventive_actions +
// disposition.json) no longer fits the 300s builtin — cycle-1048's retro was
// ctx-canceled at ~608s mid-authoring, losing the artifact entirely.
//
// The deep-tier analysis phases (tdd, build, audit, adversarial-review) carry
// 1200s: the bridge's base artifact-wait is 300s (bridge.tmuxArtifactTimeoutS)
// and the deterministic reviewer grants at most 6 extends
// (bridge.defaultArtifactMaxExtends), so ~650s of wall clock was the effective
// ceiling for every phase this map did not name. SIX cycles died there in one
// day with codes=[missing_artifact] — report + acs absent, a content-free infra
// FAIL — across FOUR phase types (audit ×2 and retro on cycle-1201,
// adversarial-review on 1217, tdd on 1218/1219), the last on a QUIET host, which
// rules out contention as the sole cause: load made it worse, the budget was the
// floor. An opus-tier agent doing real analysis on this repo legitimately needs
// more than 11 minutes. Both vocabularies are listed for the same reason retro
// carries two keys (the cycle-1054 unit-green/live-dead defect): the runner
// dispatches Agent = the core phase NAME ("tdd"/"build"/"audit"), while the
// persona vocabulary spells the same phases "tdd-engineer"/"builder"/"auditor",
// and the skew is permanent.
//
// This list stays NARROW by design. Every phase NOT named here deliberately
// keeps the 300s builtin, so global hang detection is not weakened across the
// board to fix the phases that legitimately think for a long time — a wedged
// scout/intent/triage/ship is still surfaced in ~11 minutes, not ~2.3 hours.
var defaultPhaseArtifactTimeoutS = map[string]int{
	"retrospective":      900,
	"retro":              900,
	"tdd":                1200,
	"tdd-engineer":       1200,
	"build":              1200,
	"builder":            1200,
	"audit":              1200,
	"auditor":            1200,
	"adversarial-review": 1200,
}

// PhaseArtifactTimeouts resolves the per-phase artifact-wait budgets in
// seconds, keyed on bridge agent label: the compiled defaults merged with the
// operator's phase_artifact_timeout_s overrides. Overrides are positive-only —
// a zero or negative entry is rejected rather than applied, so a typo can
// neither silently restore a compiled default's pre-fix cliff nor yield a
// negative deadline. A listed POSITIVE value is authoritative in both
// directions: an operator may deliberately raise or lower a compiled default,
// because explicit config outranks a compiled guess. A phase with no entry
// resolves 0, the bridge's "use the built-in default" sentinel (300s), and the
// global ArtifactTimeoutS is never read or written here. Returns a fresh map on
// every call so a mutating caller cannot poison later resolutions.
func (p BridgePolicy) PhaseArtifactTimeouts() map[string]int {
	out := make(map[string]int, len(defaultPhaseArtifactTimeoutS)+len(p.PhaseArtifactTimeoutS))
	for phase, budget := range defaultPhaseArtifactTimeoutS {
		out[phase] = budget
	}
	for phase, budget := range p.PhaseArtifactTimeoutS {
		if budget > 0 {
			out[phase] = budget
		}
	}
	return out
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

// CLIHealthConfig configures the CLI-health subsystem. ProactiveProbe enables
// the per-cycle, concurrent usage/status probe that benches capped families
// BEFORE any phase boots them — complementing the reactive bench (which only
// learns of a cap after a phase already burned a boot). Off by default; the
// EVOLVE_CLI_HEALTH=0 env gate remains the master kill-switch for the whole
// subsystem (canary + probe).
type CLIHealthConfig struct {
	ProactiveProbe bool `json:"proactive_probe,omitempty"`
}

// CLIHealthConfig returns the CLI-health config; the zero value (absent block)
// means ProactiveProbe=false.
func (p Policy) CLIHealthConfig() CLIHealthConfig {
	if p.CLIHealth == nil {
		return CLIHealthConfig{}
	}
	return *p.CLIHealth
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
