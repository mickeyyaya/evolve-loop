package policy

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

type FailureFloor struct {
	AlwaysLearn       *bool  `json:"always_learn,omitempty"`
	AuditFailRoutesTo string `json:"audit_fail_routes_to,omitempty"`
}

func (p Policy) FailurePolicy() (alwaysLearn bool, auditFailRoutesTo string) {
	alwaysLearn, auditFailRoutesTo = true, "retrospective"
	if p.FailureFloor == nil {
		return alwaysLearn, auditFailRoutesTo
	}
	if p.FailureFloor.AlwaysLearn != nil {
		alwaysLearn = *p.FailureFloor.AlwaysLearn
	}
	switch p.FailureFloor.AuditFailRoutesTo {
	case "retrospective", "memo":
		auditFailRoutesTo = p.FailureFloor.AuditFailRoutesTo
	}
	return alwaysLearn, auditFailRoutesTo
}

type IntegrityPolicy struct {
	Mode               string `json:"mode,omitempty"`
	Stage              string `json:"stage,omitempty"`
	ProvenanceRequired *bool  `json:"provenance_required,omitempty"`
}

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

const evaluatorFloorPhase = "audit"

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

func (p Policy) FloorEnrolls(id string) bool {
	for _, g := range p.Floor {
		if g.ID == id {
			return true
		}
	}
	return false
}

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

func (p Policy) PinFor(phase string) (Pin, bool) {
	pin, ok := p.Pins[phase]
	if !ok || (pin.CLI == "" && pin.Model == "") {
		return Pin{}, false
	}
	return pin, true
}

func Load(path string) (Policy, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Policy{}, nil
	}
	if err != nil {
		return Policy{}, fmt.Errorf("policy: read %s: %w", path, err)
	}
	p, err := Parse(raw)
	if err != nil {
		return Policy{}, fmt.Errorf("policy: parse %s: %w", path, err)
	}
	return p, nil
}

func LoadOrDefaults(path string, warn io.Writer) Policy {
	p, err := Load(path)
	if err != nil {
		fmt.Fprintf(warn, "[policy] WARN %v; the compiled defaults apply\n", err)
	}
	return p
}

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

func BaseCLI(cli string) string {
	return profiles.BaseCLI(cli)
}

var tierNames = []string{"fast", "balanced", "deep", "top"}

func TierNames() []string {
	return slices.Clone(tierNames)
}

func TierName(rank int) string {
	if rank < 1 || rank > len(tierNames) {
		return ""
	}
	return tierNames[rank-1]
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
