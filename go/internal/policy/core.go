package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

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
