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

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// IntegrityPolicy configures the per-phase binary-integrity model (ADR-0065).
type IntegrityPolicy struct {
	// Mode: "pipeline" (default — the legacy single-pin ship check) or "phase"
	// (verify the per-phase agent-block chain). Unknown ⇒ "pipeline".
	Mode string `json:"mode,omitempty"`
	// Stage: "shadow" (default — record + verify but log-only, never block) or
	// "enforce" (block on a chain/provenance violation). Unknown ⇒ "shadow".
	Stage string `json:"stage,omitempty"`
	// ProvenanceRequired: when true (default), a resume re-pin / cross-binary
	// chain is only accepted when the binary's embedded build-commit is an
	// ancestor of HEAD (or an explicit operator authorization is present).
	ProvenanceRequired *bool `json:"provenance_required,omitempty"`
}

// IntegrityMode resolves the integrity sub-policy with safe defaults applied:
// ("pipeline", "shadow", true) for an absent/partial block; unknown mode/stage
// values fall back to the default. Pure; never mutates the receiver.
func (p Policy) IntegrityMode() (mode, stage string, provenanceRequired bool) {
	mode, stage, provenanceRequired = "pipeline", "shadow", true
	if p.Integrity == nil {
		return mode, stage, provenanceRequired
	}
	switch p.Integrity.Mode {
	case "pipeline", "phase":
		mode = p.Integrity.Mode
	}
	switch p.Integrity.Stage {
	case "shadow", "enforce":
		stage = p.Integrity.Stage
	}
	if p.Integrity.ProvenanceRequired != nil {
		provenanceRequired = *p.Integrity.ProvenanceRequired
	}
	return mode, stage, provenanceRequired
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

// FloorEnrolls reports whether the policy `floor` array contains a closeout
// gate with the given id (e.g. "dossier-closeout"). Pure; nil-safe — an empty
// policy enrolls nothing.
func (p Policy) FloorEnrolls(id string) bool {
	for _, g := range p.Floor {
		if g.ID == id {
			return true
		}
	}
	return false
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
		!contains(prof.AllowedCLIs, "all") && !contains(prof.AllowedCLIs, BaseCLI(pin.CLI)) {
		return fmt.Errorf("policy: pin for phase %q: cli %q not in allowed_clis %v",
			phase, BaseCLI(pin.CLI), prof.AllowedCLIs)
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

// TierRank maps a canonical tier (fast/balanced/deep/top), a legacy alias
// (haiku/sonnet/opus), or an exact model identifier to 1/2/3/4; 0 =
// unclassifiable (the envelope check is skipped for rank 0). "top" is the
// frontier tier (modelcatalog.CanonicalTiers) and outranks deep/opus, so an
// envelope ceiling of "deep" still excludes it. Exported so
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
	case "top":
		return 4
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

// BaseCLI is the single exported base-name normalizer for driver-qualified CLI
// names: claude-tmux/claude-p → claude, codex-tmux → codex, agy-tmux → agy.
// It strips "-tmux" then "-p" repeatedly until neither suffix matches, so a
// (never-occurring-in-practice) doubly-qualified name like "codex-tmux-p"
// still resolves to its bare family "codex". This is the ONE exported source
// consolidating the formerly-duplicated policy.baseCLI and
// bridge.baseCLIName (cycle-440 MR4b, F2/F3).
func BaseCLI(cli string) string {
	s := strings.TrimSpace(cli)
	for {
		next := strings.TrimSuffix(strings.TrimSuffix(s, "-tmux"), "-p")
		if next == s {
			return next
		}
		s = next
	}
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
	// AnthropicBaseURL is the operator override for the Anthropic API base URL
	// (proxy mode). Replaces EVOLVE_ANTHROPIC_BASE_URL env read. Empty = no proxy.
	AnthropicBaseURL string `json:"anthropic_base_url,omitempty"`
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
	// StrictAudit selects the strict (legacy-blocking) audit posture. Absent/false
	// = fluent-by-default (ship on a WARN audit verdict; the failure-adapter is
	// awareness-only on recurring failures). True restores legacy blocking: WARN is
	// promoted to FAIL in both the audit phase and the ship audit-binding, and the
	// failure-adapter's first matching rule BLOCKs. Replaces the EVOLVE_STRICT_AUDIT
	// env read (flag-reduction, ADR-0064). A plain bool (not *bool): false is the
	// product default, so an absent block and an explicit false are the same posture.
	StrictAudit bool `json:"strict_audit,omitempty"`
	// CompactPrompts enables on-demand reference-section stripping from disk-loaded
	// agent docs before dispatch (strips "## Reference Index (Layer 3, on-demand)"
	// and everything after it). Absent/nil = default ON; explicit false opts out.
	CompactPrompts *bool `json:"compact_prompts,omitempty"`
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
	StrictAudit           bool
	// CompactPrompts mirrors WorkflowPolicy.CompactPrompts with the default applied.
	// Default true: phase runners strip the on-demand reference tail before dispatch.
	CompactPrompts bool
}

// WorkflowConfig returns workflow configuration with built-in defaults resolved.
func (p Policy) WorkflowConfig() WorkflowConfig {
	c := WorkflowConfig{
		MaxConsecutiveFails: 1,
		MaxCyclesCap:        25,
		// Cycle count is optional: with no explicit --cycles the advisor decides
		// how many cycles the goal needs — completion-driven (stop when the
		// backlog drains), bounded by MaxCyclesCap. Override with
		// workflow.cycle_budget="off" in policy.json to restore a fixed count.
		CycleBudget:           "enforce",
		AutoPrune:             true,
		BackfillEnabled:       true,
		ConsensusAuditEnabled: true,
		CompactPrompts:        true, // default ON: strips ~23 KB/cycle of reference tails
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
	if p.Workflow.CycleBudget != "" {
		c.CycleBudget = p.Workflow.CycleBudget
	}
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
	c.StrictAudit = p.Workflow.StrictAudit
	if p.Workflow.CompactPrompts != nil {
		c.CompactPrompts = *p.Workflow.CompactPrompts
	}
	return c
}

// StrictAuditFor loads the policy at projectRoot's .evolve/policy.json and returns
// the resolved workflow.strict_audit posture. Fail-open: a missing OR malformed
// policy yields false (fluent default) so a typo can never silently ARM the opt-in
// strict tightening — the loud malformed-policy failure still surfaces at the
// cycle's own policy.Load. The audit phase and the ship audit-binding both read
// strict mode from here (they have projectRoot but not the orchestrator's
// once-resolved WorkflowConfig), mirroring WorktreeBaseFor's loader pattern.
func StrictAuditFor(projectRoot string) bool {
	pol, err := Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		return false
	}
	return pol.WorkflowConfig().StrictAudit
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
		Scheduling: "wave", StarvationK: defaultStarvationK, StarvationWeight: defaultStarvationWeight}
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
const defaultObservationMaskWindow = 10

// ObservationMaskPolicy is the .evolve/policy.json "observation_mask" block and
// its own resolved-config type (mirrors the RouterPolicy raw==resolved idiom):
// a single WindowTurns knob. It also serves as ObservationMaskConfig's return
// value with defaults applied.
type ObservationMaskPolicy struct {
	// WindowTurns is how many of the newest evictable tool observations stay
	// unmasked. Zero/negative/absent ⇒ defaultObservationMaskWindow (10). A
	// resolved WindowTurns<=0 is never produced by the getter; when a caller
	// passes it straight to phasestream.MaskStaleObservations, <=0 means
	// "feature off" (byte-identical passthrough).
	WindowTurns int `json:"window_turns,omitempty"`
}

// ObservationMaskConfig returns the observation-mask window with the built-in
// default resolved. An absent block, or a non-positive window_turns override,
// yields WindowTurns=10; a positive override passes through. No I/O, no env.
func (p Policy) ObservationMaskConfig() ObservationMaskPolicy {
	c := ObservationMaskPolicy{WindowTurns: defaultObservationMaskWindow}
	if p.ObservationMask != nil && p.ObservationMask.WindowTurns > 0 {
		c.WindowTurns = p.ObservationMask.WindowTurns
	}
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
	// TopNGate is the build->audit top_n task-binding gate's rollout dial
	// (internal/topngate). Default "enforce": a build report whose ## Task:
	// slug falls outside triage ## top_n aborts before audit.
	TopNGate string `json:"topn_gate,omitempty"`
	// ReportSizeGate is the report-size (handoff-summary token budget) gate's own
	// rollout dial (cycle-565 Slice S1). Unlike the other gates it defaults to
	// "shadow", not "enforce": the inbox spec calls for shadow/warn BEFORE
	// enforce so the budget is observed before it can block a cycle.
	ReportSizeGate string `json:"report_size_gate,omitempty"`
}

// GatesConfig is the resolved gate configuration with defaults applied.
type GatesConfig struct {
	ContractGate   string
	EvalGate       string
	TriageCapGate  string
	ReviewGate     string
	ReportSizeGate string
	TopNGate       string
}

// GatesConfig returns persistent gate stages with built-in defaults resolved.
func (p Policy) GatesConfig() GatesConfig {
	c := GatesConfig{
		ContractGate:   "enforce",
		EvalGate:       "enforce",
		TriageCapGate:  "enforce",
		ReviewGate:     "off",
		ReportSizeGate: "shadow", // shadow/warn first, per the Slice S1 inbox spec
		TopNGate:       "enforce",
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
	if p.Gates.ReportSizeGate != "" {
		c.ReportSizeGate = p.Gates.ReportSizeGate
	}
	if p.Gates.TopNGate != "" {
		c.TopNGate = p.Gates.TopNGate
	}
	return c
}

// ReportBudgetPolicy is the .evolve/policy.json "report_budget" block: the
// per-artifact token budgets the report-size gate enforces (cycle-565 Slice S1).
// Separate from GatesPolicy so the budget VALUE and the gate STAGE are dialed
// independently. Absent ⇒ built-in defaults apply.
type ReportBudgetPolicy struct {
	HandoffTokens int `json:"handoff_tokens,omitempty"`
}

// ReportBudgetConfig is the resolved report-budget configuration with defaults
// applied. HandoffTokens is the ~2K default token budget for the never-evict
// "## Handoff Summary" section (policy-sourced per phase_settings_from_config_not_code).
type ReportBudgetConfig struct {
	HandoffTokens int
}

// ReportBudgetConfig returns the report-size token budgets with built-in
// defaults resolved. An absent or empty report_budget block resolves the 2000
// default; an explicit positive value overrides it.
func (p Policy) ReportBudgetConfig() ReportBudgetConfig {
	c := ReportBudgetConfig{HandoffTokens: 2000}
	if p.ReportBudget == nil {
		return c
	}
	if p.ReportBudget.HandoffTokens > 0 {
		c.HandoffTokens = p.ReportBudget.HandoffTokens
	}
	return c
}

// RetroAutofilePolicy is the .evolve/policy.json "retro_autofile" block. It
// tunes the retro→inbox preventive-actions autofiler (internal/retrofile):
// DefaultWeight is the weight applied to a filed preventive-action item that
// carries no per-action weight_hint. Absent ⇒ the compiled-in safe default.
type RetroAutofilePolicy struct {
	DefaultWeight float64 `json:"default_weight,omitempty"`
}

// RetroAutofileDefaultWeight returns the default weight for auto-filed retro
// preventive-action inbox items. Absent/non-positive block ⇒ 0.75 (the
// compiled-in safe default); a present positive DefaultWeight overrides it.
// Sourced from policy, never a Go literal at the call site
// (feedback_phase_settings_from_config_not_code).
func (p Policy) RetroAutofileDefaultWeight() float64 {
	const safeDefault = 0.75
	if p.RetroAutofile == nil || p.RetroAutofile.DefaultWeight <= 0 {
		return safeDefault
	}
	return p.RetroAutofile.DefaultWeight
}

// SandboxPolicy is the .evolve/policy.json "sandbox" block. NestedFallback
// selects the verified-fallback rollout stage for nested runs where the inner
// OS sandbox can't apply: "off" (default — no canary), "shadow" (run the
// write-canary and WARN if the outer environment is unverified), or "enforce"
// (HALT the batch if unverified). Resolved to a config.Stage via parseGateStage
// at the composition root; unknown values map to off (canary disabled).
type SandboxPolicy struct {
	NestedFallback string `json:"nested_fallback,omitempty"`
}

// SandboxConfig returns sandbox configuration with built-in defaults resolved.
// Empty/absent NestedFallback ⇒ "off" (canary opt-in; a fresh policy.json never
// runs the write-canary nor halts a nested run).
func (p Policy) SandboxConfig() SandboxPolicy {
	c := SandboxPolicy{NestedFallback: "off"}
	if p.Sandbox == nil {
		return c
	}
	if p.Sandbox.NestedFallback != "" {
		c.NestedFallback = p.Sandbox.NestedFallback
	}
	return c
}

// RecoveryPolicy is the .evolve/policy.json "recovery" block.
// It surfaces the ADR-0044 Unified Phase Recovery rollout stage so operators
// can set phase_recovery = "enforce" in policy.json without an env var.
type RecoveryPolicy struct {
	PhaseRecovery string `json:"phase_recovery,omitempty"`
}

// RecoveryConfig returns recovery configuration with built-in defaults resolved.
// Empty/absent PhaseRecovery ⇒ "shadow" (behavior-neutral first-ship default).
func (p Policy) RecoveryConfig() RecoveryPolicy {
	c := RecoveryPolicy{PhaseRecovery: "shadow"}
	if p.Recovery == nil {
		return c
	}
	if p.Recovery.PhaseRecovery != "" {
		c.PhaseRecovery = p.Recovery.PhaseRecovery
	}
	return c
}

// MergeGatePolicy is the .evolve/policy.json "merge_gate" block — the config-as-code
// surface for the merge-to-main gate (no flags). Stage drives the
// shadow→advisory→enforce rollout; the remaining fields are the cadence-scaling
// thresholds the advisor reads to decide when accumulated milestone work is
// promoted to main.
type MergeGatePolicy struct {
	// Stage selects the rollout stage: "off" / "shadow" / "advisory" / "enforce".
	// Empty/absent ⇒ "shadow" (gate runs and records its would-be verdict but
	// promotes nothing — byte-neutral first deploy over the riskiest action). The
	// composition root translates this string to a config.Stage via parseStage,
	// whose closed vocabulary maps any UNKNOWN value (e.g. a "enforced" typo) to
	// StageOff — a fail-safe that disables the gate rather than guessing, so a
	// misspelling can never silently arm auto-merge.
	Stage string `json:"stage,omitempty"`
	// BatchWaveCount is how many completed campaign waves accumulate before the
	// advisor fires the gate (cadence scaling). Zero/absent ⇒ 1 (gate per wave).
	BatchWaveCount int `json:"batch_wave_count,omitempty"`
	// BatchChurnLOC is the diff-size ceiling (changed LOC) above which the advisor
	// prefers batching over per-wave promotion. Zero/absent ⇒ 800.
	BatchChurnLOC int `json:"batch_churn_loc,omitempty"`
	// BlockSeverity is the build severity at or above which the gate hard-defers
	// promotion. Empty/absent ⇒ "HIGH".
	BlockSeverity string `json:"block_severity,omitempty"`
	// CarryoverStallCycles is the anti-starvation bound: when a feature's oldest
	// unpicked P0/P1 carryover has aged this many cycles, force a feature-complete
	// promotion attempt. Zero/absent ⇒ 8.
	CarryoverStallCycles int `json:"carryover_stall_cycles,omitempty"`
}

// MergeGateConfig is the resolved merge-gate configuration with defaults applied.
type MergeGateConfig struct {
	Stage                string
	BatchWaveCount       int
	BatchChurnLOC        int
	BlockSeverity        string
	CarryoverStallCycles int
}

// MergeGateConfig returns merge-gate configuration with built-in defaults
// resolved. The zero-value Policy{} yields the safe defaults (stage="shadow",
// so an absent block is provably behavior-neutral). Each numeric threshold
// overrides only when > 0 and each string only when non-empty, so a partial
// block can never silently produce an unsafe zero threshold. Pure.
func (p Policy) MergeGateConfig() MergeGateConfig {
	c := MergeGateConfig{
		Stage:                "shadow",
		BatchWaveCount:       1,
		BatchChurnLOC:        800,
		BlockSeverity:        "HIGH",
		CarryoverStallCycles: 8,
	}
	if p.MergeGate == nil {
		return c
	}
	if p.MergeGate.Stage != "" {
		c.Stage = p.MergeGate.Stage
	}
	if p.MergeGate.BatchWaveCount > 0 {
		c.BatchWaveCount = p.MergeGate.BatchWaveCount
	}
	if p.MergeGate.BatchChurnLOC > 0 {
		c.BatchChurnLOC = p.MergeGate.BatchChurnLOC
	}
	if p.MergeGate.BlockSeverity != "" {
		c.BlockSeverity = p.MergeGate.BlockSeverity
	}
	if p.MergeGate.CarryoverStallCycles > 0 {
		c.CarryoverStallCycles = p.MergeGate.CarryoverStallCycles
	}
	return c
}

// ParallelEvaluatePolicy is the .evolve/policy.json "parallel_evaluate" block —
// the config-as-code surface for post-build evaluate-phase parallelization (no
// flags). Stage drives the off→shadow→enforce rollout; Concurrency bounds the
// parallel runner pool. See [[phase_settings_from_config_not_code]].
type ParallelEvaluatePolicy struct {
	// Stage selects the rollout stage: "off" / "shadow" / "enforce".
	// Empty/absent ⇒ "off" (dispatcher dormant; byte-identical baseline).
	// Any unknown value (e.g. a typo) maps to "off" — a fail-safe so a
	// misspelling can never silently arm the parallel dispatcher.
	Stage string `json:"stage,omitempty"`
	// Concurrency bounds the parallel runner pool when Stage="enforce".
	// Zero/negative/absent ⇒ 3 (the soak sweet spot: ~11% saving, diminishing
	// past it). Applies only; use cases where an explicit cap is needed.
	Concurrency int `json:"concurrency,omitempty"`
}

// ParallelEvaluateConfig is the resolved parallel-evaluate configuration with
// defaults applied.
type ParallelEvaluateConfig struct {
	Stage       string
	Concurrency int
}

// ParallelEvaluateConfig returns parallel-evaluate configuration with built-in
// defaults resolved. The zero-value Policy{} yields safe defaults (stage="off",
// concurrency=3). Stage overrides apply only for the closed vocabulary
// {"off","shadow","advisory","enforce"}; any other value falls back to "off"
// (fail-safe). Concurrency overrides apply only when > 0. Pure.
func (p Policy) ParallelEvaluateConfig() ParallelEvaluateConfig {
	c := ParallelEvaluateConfig{Stage: "off", Concurrency: 3}
	if p.ParallelEvaluate == nil {
		return c
	}
	if s := p.ParallelEvaluate.Stage; s != "" {
		switch s {
		case "off", "shadow", "advisory", "enforce":
			c.Stage = s
		default:
			c.Stage = "off"
		}
	}
	if p.ParallelEvaluate.Concurrency > 0 {
		c.Concurrency = p.ParallelEvaluate.Concurrency
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

// ClassifyPolicy configures the cycle-failure classifier (internal/cycleclassify).
// Loaded from .evolve/policy.json "classify" block; absent block ⇒ built-in
// defaults apply. Replaces EVOLVE_HANG_CLASSIFIER env read.
type ClassifyPolicy struct {
	// HangClassifier enables the two-factor exit-transport-hang reclassification:
	// when true, a SHIPPED-verdict report + matching git commit reclassifies an
	// apparent integrity-breach as ClassExitTransportHang (1h retention vs 7d).
	// Default false — hang detection is opt-in so a misconfigured git log never
	// silently masks a real breach.
	HangClassifier bool `json:"hang_classifier,omitempty"`
}

// ClassifyConfig returns the classifier configuration with defaults resolved.
// An absent block yields ClassifyPolicy{HangClassifier: false} — safe default.
func (p Policy) ClassifyConfig() ClassifyPolicy {
	if p.Classify == nil {
		return ClassifyPolicy{}
	}
	return *p.Classify
}

// CatalogPolicy configures the model catalog subsystem.
// Loaded from .evolve/policy.json "catalog" block; absent block ⇒ built-in
// defaults apply. Replaces EVOLVE_MODELCATALOG_AUTOREFRESH env read.
type CatalogPolicy struct {
	// AutoRefresh controls whether the cycle-start live model-catalog refresh
	// runs. Nil/absent ⇒ true (opt-out semantics: default on, set false to
	// disable). Replaces EVOLVE_MODELCATALOG_AUTOREFRESH=0.
	AutoRefresh *bool `json:"auto_refresh,omitempty"`

	// AllowedFamilies is a per-CLI model-family allow-list (e.g. agy:[gemini]).
	// A CLI's live-queried candidate ids are filtered to these families BEFORE
	// classification (modelquery.RefreshDeps threads this through FilterByFamily),
	// so a cross-family id can never reach the classifier (D7: "agy must not have
	// Claude models"). Nil/absent ⇒ no constraint — byte-identical to today for
	// every CLI that does not opt in.
	AllowedFamilies map[string][]string `json:"allowed_families,omitempty"`
}

// CatalogConfig returns the catalog configuration with defaults resolved.
// AutoRefresh defaults to true (opt-out); the returned pointer is never nil.
func (p Policy) CatalogConfig() CatalogPolicy {
	enabled := true
	out := CatalogPolicy{AutoRefresh: &enabled}
	if p.Catalog == nil {
		return out
	}
	if p.Catalog.AutoRefresh != nil {
		out.AutoRefresh = p.Catalog.AutoRefresh
	}
	// Pass AllowedFamilies through unchanged — nil stays nil ("no constraint");
	// never default to an empty map (callers/tests distinguish nil from empty).
	out.AllowedFamilies = p.Catalog.AllowedFamilies
	return out
}

// ACSConfig configures the ACS Go lane timeout.
// Loaded from .evolve/policy.json "acs" block; absent block ⇒ built-in
// defaults apply (DefaultTimeout=60s). Replaces EVOLVE_ACS_GO_TIMEOUT_S env read.
type ACSConfig struct {
	// GoTimeoutS overrides the whole-Go-lane timeout in seconds. 0 = use DefaultTimeout.
	GoTimeoutS int `json:"go_timeout_s,omitempty"`
}

// ACSTimeoutConfig returns the ACS timeout configuration.
// An absent block returns ACSConfig{GoTimeoutS:0} — callers must treat 0 as
// "use DefaultTimeout" to avoid a zero-duration timeout.
func (p Policy) ACSTimeoutConfig() ACSConfig {
	if p.ACS == nil {
		return ACSConfig{}
	}
	return *p.ACS
}

// PathsConfig configures path-discovery overrides.
// Loaded from .evolve/policy.json "paths" block; absent block ⇒ built-in
// defaults apply. Replaces EVOLVE_KB_SEARCH_PATHS and EVOLVE_PHASE_ROOTS reads.
type PathsConfig struct {
	// KBSearchPaths is a colon-separated list of KB search roots.
	// Empty ⇒ built-in default (knowledge-base/research/:.evolve/instincts/lessons/:docs/research/).
	// Replaces EVOLVE_KB_SEARCH_PATHS.
	KBSearchPaths string `json:"kb_search_paths,omitempty"`
	// PhaseRoots is a colon-separated list of phase discovery roots.
	// Empty ⇒ built-in default (.evolve/phases). Replaces EVOLVE_PHASE_ROOTS.
	PhaseRoots string `json:"phase_roots,omitempty"`
}

// PathsConfig returns the paths configuration.
// An absent block returns PathsConfig{} — callers fall back to built-in defaults.
func (p Policy) PathsConfig() PathsConfig {
	if p.Paths == nil {
		return PathsConfig{}
	}
	return *p.Paths
}
