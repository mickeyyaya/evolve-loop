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
// Layering: imports profiles + stdlib only, so the dispatch resolver (llmroute)
// and the advisor can consult it without a heavy dependency. The tier/CLI
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
		rank := tierRank(pin.Model)
		minR, maxR := tierRank(prof.ModelTierEnvelope.Min), tierRank(prof.ModelTierEnvelope.Max)
		if rank > 0 && minR > 0 && maxR > 0 && (rank < minR || rank > maxR) {
			return fmt.Errorf("policy: pin for phase %q: model %q (tier rank %d) outside envelope [%s..%s]",
				phase, pin.Model, rank, prof.ModelTierEnvelope.Min, prof.ModelTierEnvelope.Max)
		}
	}
	return nil
}

// --- canonical tier/CLI vocabulary (mirror of setup.go; see package doc) ---

// tierRank maps a canonical tier (fast/balanced/deep), a legacy alias
// (haiku/sonnet/opus), or an exact model identifier to 1/2/3; 0 =
// unclassifiable (the envelope check is skipped for rank 0).
func tierRank(s string) int {
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
