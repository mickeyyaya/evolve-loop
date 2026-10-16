package policy

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

type OverlaysPolicy struct {
	Rules   []OverlayRule         `json:"rules,omitempty"`
	Advisor *AdvisorOverlayPolicy `json:"advisor,omitempty"`
}

type AdvisorOverlayPolicy struct {
	AllowList            []string `json:"allow_list,omitempty"`
	DenyList             []string `json:"deny_list,omitempty"`
	MaxSkillsPerDispatch int      `json:"max_skills_per_dispatch,omitempty"`
}

const defaultMaxSkillsPerDispatch = 2

type AdvisorSkillRejection struct {
	Skill  string `json:"skill"`
	Reason string `json:"reason"`
}

type OverlayRule struct {
	Phases       []string           `json:"phases,omitempty"`
	CLIs         []string           `json:"clis,omitempty"`
	Models       []string           `json:"models,omitempty"`
	Tiers        []string           `json:"tiers,omitempty"`
	When         []config.Condition `json:"when,omitempty"`
	WritesSource bool               `json:"writes_source,omitempty"`
	Skills       []string           `json:"skills,omitempty"`
}

type OverlayDispatch struct {
	Phase        string
	CLI          string
	Model        string
	Tier         string
	Signals      map[string]string
	WritesSource bool
}

const SelfReviewSkill = "code-review-simplify"

const DocumentWriteSkill = "docs-write"

func compiledDefaultOverlays() []OverlayRule {
	document := []config.Condition{{Field: config.SignalDeliverableKind, Op: "eq", Value: config.DeliverableKindDocument}}
	code := []config.Condition{{Field: config.SignalDeliverableKind, Op: "eq", Value: config.DeliverableKindCode}}
	return []OverlayRule{
		{Tiers: []string{"deep", "top"}, Skills: []string{"fable"}},
		{Phases: []string{"scout"}, When: document, Skills: []string{"solution-scout", DocumentWriteSkill}},
		{Phases: []string{"build"}, When: document, Skills: []string{"solution-build", DocumentWriteSkill}},
		{Phases: []string{"audit"}, When: document, Skills: []string{"solution-audit", DocumentWriteSkill}},
		{WritesSource: true, When: code, Skills: []string{"engineering-craft", SelfReviewSkill, DocumentWriteSkill}},
		{Phases: []string{"code-review"}, When: code, Skills: []string{"engineering-craft", SelfReviewSkill, "architecture-review", "quality-index"}},
	}
}

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

func DispatchFromPhaseRequest(phase, cli, model, tier string) OverlayDispatch {
	return OverlayDispatch{Phase: phase, CLI: cli, Model: model, Tier: tier}
}

func ResolveLaunchOverlaysFailOpen(projectRoot, phase, cli, model string) []string {
	pol, err := Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[overlays] WARN policy load failed (%v) — using compiled-default overlays\n", err)
		pol = Policy{}
	}
	return pol.ResolveOverlays(DispatchFromPhaseRequest(phase, cli, model, model))
}

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

func (p Policy) ClampAdvisorSkills(proposed, registry []string) (accepted []string, rejections []AdvisorSkillRejection) {
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
	return (d.WritesSource || !r.WritesSource) &&
		matchDim(r.Phases, d.Phase) &&
		matchDim(r.CLIs, d.CLI) &&
		matchDim(r.Models, d.Model) &&
		matchTier(r.Tiers, d.Tier) &&
		matchWhen(r.When, d.Signals)
}

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

func matchTier(selectors []string, tier string) bool {
	if matchDim(selectors, tier) {
		return true
	}
	canonical := canonicalTier(tier)
	if canonical == "" {
		return false
	}
	for _, sel := range selectors {
		if !isGlob(sel) && canonicalTier(sel) == canonical {
			return true
		}
	}
	return false
}

func canonicalTier(token string) string {
	return TierName(TierRank(token))
}

func isGlob(selector string) bool {
	return strings.ContainsAny(selector, `*?[\`)
}

func namesCanonicalTier(selector string) bool {
	if !isGlob(selector) {
		return canonicalTier(selector) != ""
	}
	for _, tier := range tierNames {
		if ok, err := path.Match(selector, tier); err == nil && ok {
			return true
		}
	}
	return false
}

func (p Policy) NonCanonicalOverlayTierSelectors() []string {
	if p.Overlays == nil {
		return nil
	}
	var out []string
	seen := map[string]struct{}{}
	for _, r := range p.Overlays.Rules {
		for _, sel := range r.Tiers {
			if _, dup := seen[sel]; dup || namesCanonicalTier(sel) {
				continue
			}
			seen[sel] = struct{}{}
			out = append(out, sel)
		}
	}
	return out
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
