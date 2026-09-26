package policy

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// OverlaysPolicy is the "overlays" block; a dispatch may match several rules, and their skills union.
type OverlaysPolicy struct {
	Rules []OverlayRule `json:"rules,omitempty"`
	// Advisor bounds what the advisor may propose; nil means allow all, deny none, max 2.
	Advisor *AdvisorOverlayPolicy `json:"advisor,omitempty"`
}

// AdvisorOverlayPolicy is the operator's clamp on advisor-proposed skills.
type AdvisorOverlayPolicy struct {
	AllowList            []string `json:"allow_list,omitempty"`
	DenyList             []string `json:"deny_list,omitempty"`
	MaxSkillsPerDispatch int      `json:"max_skills_per_dispatch,omitempty"`
}

const defaultMaxSkillsPerDispatch = 2

// AdvisorSkillRejection records one clamped-out proposal, logged to advisor-rejections.json.
type AdvisorSkillRejection struct {
	Skill string `json:"skill"`
	// Reason is "not-in-registry", "not-allowlisted", "denylisted" or "over-max-skills-per-dispatch".
	Reason string `json:"reason"`
}

// OverlayRule matches when every non-empty selector glob-matches (path.Match) and every When clause holds.
type OverlayRule struct {
	Phases []string `json:"phases,omitempty"`
	CLIs   []string `json:"clis,omitempty"`
	Models []string `json:"models,omitempty"`
	Tiers  []string `json:"tiers,omitempty"`
	// When keys the rule on the cycle's signals; a clause on an absent signal never matches.
	// See ADR-0099.
	When   []config.Condition `json:"when,omitempty"`
	Skills []string           `json:"skills,omitempty"`
}

// OverlayDispatch describes one agent launch for overlay resolution; nil Signals leaves every When rule inert.
type OverlayDispatch struct {
	Phase   string
	CLI     string
	Model   string
	Tier    string
	Signals map[string]string
}

// compiledDefaultOverlays is the only built-in tier→skills mapping; a policy overlays block replaces it wholesale.
func compiledDefaultOverlays() []OverlayRule {
	document := []config.Condition{{Field: config.SignalDeliverableKind, Op: "eq", Value: config.DeliverableKindDocument}}
	return []OverlayRule{
		{Tiers: []string{"deep", "top"}, Skills: []string{"fable"}},
		{Phases: []string{"scout"}, When: document, Skills: []string{"solution-scout"}},
		{Phases: []string{"build"}, When: document, Skills: []string{"solution-build"}},
		{Phases: []string{"audit"}, When: document, Skills: []string{"solution-audit"}},
	}
}

// CompiledDefaultOverlaySkills lists the compiled-default skills; ProtectedSurfaceManifest is pinned to it.
func CompiledDefaultOverlaySkills() []string {
	var out []string
	seen := map[string]struct{}{}
	for _, r := range compiledDefaultOverlays() {
		for _, s := range r.Skills {
			if _, dup := seen[s]; dup {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}

// ResolveOverlays returns the deduped skills, in first-seen order, of every rule matching d.
func (p Policy) ResolveOverlays(d OverlayDispatch) []string {
	rules := compiledDefaultOverlays()
	if p.Overlays != nil {
		rules = p.Overlays.Rules
	}
	var out []string
	seen := map[string]struct{}{}
	for _, r := range rules {
		if !r.matches(d) {
			continue
		}
		for _, s := range r.Skills {
			if s == "" {
				continue
			}
			if _, dup := seen[s]; dup {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}

// DispatchFromPhaseRequest builds an OverlayDispatch; pass tier="" unless model_routing=auto produced a tier.
func DispatchFromPhaseRequest(phase, cli, model, tier string) OverlayDispatch {
	return OverlayDispatch{Phase: phase, CLI: cli, Model: model, Tier: tier}
}

// ResolveLaunchOverlaysFailOpen resolves overlays for the non-phase launch seams, where model doubles as the tier.
func ResolveLaunchOverlaysFailOpen(projectRoot, phase, cli, model string) []string {
	pol, err := Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	// Fail open: these seams have no report to carry the error, unlike the phase runner.
	if err != nil {
		fmt.Fprintf(os.Stderr, "[overlays] WARN policy load failed (%v) — using compiled-default overlays\n", err)
		pol = Policy{}
	}
	return pol.ResolveOverlays(DispatchFromPhaseRequest(phase, cli, model, model))
}

// SkillRegistryFromFS lists skillsDir's subdirectories that contain a SKILL.md; none is a valid empty registry.
func SkillRegistryFromFS(skillsDir string) ([]string, error) {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(skillsDir, e.Name(), "SKILL.md")); err != nil {
			continue
		}
		out = append(out, e.Name())
	}
	return out, nil
}

// ClampAdvisorSkills filters proposals by registry and overlays.advisor in order, recording each rejection.
func (p Policy) ClampAdvisorSkills(proposed, registry []string) (accepted []string, rejections []AdvisorSkillRejection) {
	// Exact string membership, never path-normalized, so a path-like proposal can never resolve.
	inRegistry := make(map[string]struct{}, len(registry))
	for _, s := range registry {
		inRegistry[s] = struct{}{}
	}
	var allow, deny map[string]struct{}
	maxSkills := defaultMaxSkillsPerDispatch
	if p.Overlays != nil && p.Overlays.Advisor != nil {
		a := p.Overlays.Advisor
		if a.MaxSkillsPerDispatch > 0 {
			maxSkills = a.MaxSkillsPerDispatch
		}
		if len(a.AllowList) > 0 {
			allow = toSet(a.AllowList)
		}
		if len(a.DenyList) > 0 {
			deny = toSet(a.DenyList)
		}
	}
	for _, s := range proposed {
		if _, ok := inRegistry[s]; !ok {
			rejections = append(rejections, AdvisorSkillRejection{Skill: s, Reason: "not-in-registry"})
			continue
		}
		if allow != nil {
			if _, ok := allow[s]; !ok {
				rejections = append(rejections, AdvisorSkillRejection{Skill: s, Reason: "not-allowlisted"})
				continue
			}
		}
		if _, ok := deny[s]; ok {
			rejections = append(rejections, AdvisorSkillRejection{Skill: s, Reason: "denylisted"})
			continue
		}
		if len(accepted) >= maxSkills {
			rejections = append(rejections, AdvisorSkillRejection{Skill: s, Reason: "over-max-skills-per-dispatch"})
			continue
		}
		accepted = append(accepted, s)
	}
	return accepted, rejections
}

// ResolveOverlaysWithAdvisor appends already-clamped advisor skills, deduped, after ResolveOverlays' result.
func (p Policy) ResolveOverlaysWithAdvisor(d OverlayDispatch, clampedAdvisorSkills []string) []string {
	out := p.ResolveOverlays(d)
	if len(clampedAdvisorSkills) == 0 {
		return out
	}
	seen := toSet(out)
	for _, s := range clampedAdvisorSkills {
		if s == "" {
			continue
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func toSet(ss []string) map[string]struct{} {
	m := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		m[s] = struct{}{}
	}
	return m
}

func (r OverlayRule) matches(d OverlayDispatch) bool {
	return matchDim(r.Phases, d.Phase) &&
		matchDim(r.CLIs, d.CLI) &&
		matchDim(r.Models, d.Model) &&
		matchDim(r.Tiers, d.Tier) &&
		matchWhen(r.When, d.Signals)
}

// matchWhen fails closed: an absent signal, a non-string value or an unknown op
// never matches, in either polarity.
func matchWhen(when []config.Condition, signals map[string]string) bool {
	for _, c := range when {
		v, ok := signals[c.Field]
		want, isStr := c.Value.(string)
		if !ok || !isStr {
			return false
		}
		switch c.Op {
		case "eq", "==":
			if v != want {
				return false
			}
		case "ne", "!=":
			if v == want {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func matchDim(patterns []string, value string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, pat := range patterns {
		if ok, err := path.Match(pat, value); err == nil && ok {
			return true
		}
	}
	return false
}
