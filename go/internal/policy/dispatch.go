package policy

// FanoutPolicy is the "fanout" block; prefer Policy.FanoutConfig for resolved values.
type FanoutPolicy struct {
	// Concurrency: below 1 means 2.
	Concurrency int `json:"concurrency,omitempty"`
	// TimeoutSecs: 0 means the fanoutdispatch built-in.
	TimeoutSecs       int  `json:"timeout_secs,omitempty"`
	CancelOnConsensus bool `json:"cancel_on_consensus,omitempty"`
	// ConsensusK: 0 means the fanoutdispatch built-in.
	ConsensusK int `json:"consensus_k,omitempty"`
	// ConsensusPollSecs: 0 means the fanoutdispatch built-in.
	ConsensusPollSecs int `json:"consensus_poll_secs,omitempty"`
	// TrackWorkers: nil means true.
	TrackWorkers *bool `json:"track_workers,omitempty"`
	// CachePrefixEnabled writes a shared cache-prefix.md for siblings; nil means true.
	CachePrefixEnabled *bool `json:"cache_prefix_enabled,omitempty"`
	// TestExecutor overrides the fanout worker command for test harnesses.
	TestExecutor string `json:"test_executor,omitempty"`
}

// FanoutConfig resolves the fanout block; the returned pointers are never nil.
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

// ObserverPolicy configures phase observation and watchdogs; pointers keep an explicit zero (nudge_s=0 disables) distinct from absent.
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

// ObserverConfig resolves the observer block; the returned pointers are never nil.
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

// BridgePolicy holds bridge directory and timing overrides; an empty string or zero int means the built-in.
type BridgePolicy struct {
	ManifestDir        string `json:"manifest_dir,omitempty"`
	CatalogDir         string `json:"catalog_dir,omitempty"`
	RecipeDir          string `json:"recipe_dir,omitempty"`
	BootTimeoutS       int    `json:"boot_timeout_s,omitempty"`
	ArtifactTimeoutS   int    `json:"artifact_timeout_s,omitempty"`
	ArtifactMaxExtends int    `json:"artifact_max_extends,omitempty"`
	ScrollbackLines    int    `json:"scrollback_lines,omitempty"`
	// PhaseArtifactTimeoutS is keyed on the bridge agent label; see PhaseArtifactTimeouts.
	PhaseArtifactTimeoutS map[string]int `json:"phase_artifact_timeout_s,omitempty"`
	// AnthropicBaseURL is the proxy base URL; empty means no proxy.
	AnthropicBaseURL string `json:"anthropic_base_url,omitempty"`
}

// defaultPhaseArtifactTimeoutS keys each phase by both its agent label and persona alias (the two differ
// permanently), and stays narrow so every other phase keeps the 300s hang-detection builtin.
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

// PhaseArtifactTimeouts returns a fresh map of compiled per-phase budgets merged with positive overrides.
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

// BridgeConfig returns the bridge block, or the zero value when absent.
func (p Policy) BridgeConfig() BridgePolicy {
	if p.Bridge == nil {
		return BridgePolicy{}
	}
	return *p.Bridge
}

// QuotaResetConfig configures the quotareset wake-time estimator.
type QuotaResetConfig struct {
	// ResetAt is an ISO 8601 wake-time override; empty means none.
	ResetAt string `json:"reset_at,omitempty"`
	// DefaultHours is the fallback wake duration; 0 means the built-in 5.4167 (about 5h25m).
	DefaultHours float64 `json:"default_hours,omitempty"`
}

// QuotaResetConfig returns the quota_reset block, or the zero value (quotareset defaults) when absent.
func (p Policy) QuotaResetConfig() QuotaResetConfig {
	if p.QuotaReset == nil {
		return QuotaResetConfig{}
	}
	return *p.QuotaReset
}

// CLIHealthConfig configures the CLI-health subsystem; EVOLVE_CLI_HEALTH=0 still disables all of it.
type CLIHealthConfig struct {
	// ProactiveProbe (opt-in) benches capped CLI families before any phase boots them.
	ProactiveProbe bool `json:"proactive_probe,omitempty"`
}

// CLIHealthConfig returns the cli_health block, or the zero value (probe off) when absent.
func (p Policy) CLIHealthConfig() CLIHealthConfig {
	if p.CLIHealth == nil {
		return CLIHealthConfig{}
	}
	return *p.CLIHealth
}

// DispatchConfig configures loop dispatch verification and its repeat circuit-breaker.
type DispatchConfig struct {
	// Policy is "off", "verify" (default) or "stop".
	Policy string `json:"policy,omitempty"`
	// RepeatThreshold is the same-cycle repeat count that trips the breaker; non-positive means 5.
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
