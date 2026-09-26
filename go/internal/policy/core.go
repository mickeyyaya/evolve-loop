package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// FailureFloor configures the failure-learning policy surface.
type FailureFloor struct {
	// AlwaysLearn=false only makes the retro lighter; it never suppresses the deterministic floor.
	AlwaysLearn *bool `json:"always_learn,omitempty"`
	// AuditFailRoutesTo is "retrospective" (default) or "memo".
	AuditFailRoutesTo string `json:"audit_fail_routes_to,omitempty"`
}

// FailurePolicy resolves failure_floor, defaulting to (true, "retrospective").
func (p Policy) FailurePolicy() (alwaysLearn bool, auditFailRoutesTo string) {
	alwaysLearn, auditFailRoutesTo = true, "retrospective"
	if p.FailureFloor == nil {
		return alwaysLearn, auditFailRoutesTo
	}
	if p.FailureFloor.AlwaysLearn != nil {
		alwaysLearn = *p.FailureFloor.AlwaysLearn
	}
	// An unknown route falls back to the default so some learning phase always runs.
	switch p.FailureFloor.AuditFailRoutesTo {
	case "retrospective", "memo":
		auditFailRoutesTo = p.FailureFloor.AuditFailRoutesTo
	}
	return alwaysLearn, auditFailRoutesTo
}

// IntegrityPolicy configures the per-phase binary-integrity model.
// See ADR-0065.
type IntegrityPolicy struct {
	// Mode is "pipeline" (default, the single-pin ship check) or "phase" (the per-phase chain).
	Mode string `json:"mode,omitempty"`
	// Stage is "shadow" (default, log-only) or "enforce" (block on a violation).
	Stage string `json:"stage,omitempty"`
	// ProvenanceRequired (default true) accepts a cross-binary chain only when the
	// binary's build commit is an ancestor of HEAD or the operator authorized it.
	ProvenanceRequired *bool `json:"provenance_required,omitempty"`
}

// IntegrityMode resolves the integrity block, defaulting to ("pipeline", "shadow", true).
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

// evaluatorFloorPhase mirrors router.EvaluatorFloorPhase; a shared const would
// create an import cycle, and TestEvaluatorFloorPhase_SingleSource catches drift.
const evaluatorFloorPhase = "audit"

// FloorPhases returns the ship floor with "audit" guaranteed; overridden=false means use the router's default.
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

// FloorEnrolls reports whether the `floor` array enrolls the closeout gate id.
func (p Policy) FloorEnrolls(id string) bool {
	for _, g := range p.Floor {
		if g.ID == id {
			return true
		}
	}
	return false
}

// MergeMandatory returns base plus any missing MandatoryPhases, in order, without mutating base.
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

// Load reads policy.json at path: an absent file is an empty Policy, a malformed one is an error.
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

// ValidatePin returns the first breach of the profile's allowed_clis or model_tier_envelope; a nil profile passes.
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

// TierRank maps a tier, legacy alias or model id to 1-4 (fast..top); 0 means unclassifiable.
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

// BaseCLI strips driver suffixes ("-tmux", "-p") repeatedly: claude-tmux → claude.
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
