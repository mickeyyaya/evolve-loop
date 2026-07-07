// overlays.go — skill-overlay resolver (cycle-609 skill-overlays-bridge-layer).
//
// The nil-able-pointer + resolver idiom mirrors ObserverPolicy/ObserverConfig
// (policy.go): Policy.Overlays == nil ⇒ the compiled default applies; a non-nil
// block with an empty Rules slice is an explicit operator opt-out (zero
// overlays, NOT the default); a non-nil block with rules resolves the UNION of
// every matching rule's skills, deduped, in stable (first-seen) order.
//
// This is the policy-layer resolver only — a pure, side-effect-free mapping
// from a dispatch descriptor to an ordered skill list. It carries no producer:
// nothing constructs an OverlayDispatch from a live BridgeRequest yet, so the
// audit-F1 protected-surface gap (skill dirs referenced by overlays) stays
// dormant. Wiring Engine.Launch to inject these skills — the Tier producer that
// makes F1 reachable — MUST land in the same slice that extends
// protectedSurfaceFragments (guards/integrity_surface.go), and that file is on
// the control-plane boundary (no autonomous --class cycle may edit it). So the
// injection seam is intentionally deferred to an out-of-cycle manual ship; see
// build-report.md § Deferred.
package policy

import (
	"os"
	"path"
	"path/filepath"
)

// OverlaysPolicy is the operator-configurable skill-overlay block. Rules are
// evaluated in order; a dispatch may match several (their skills union).
type OverlaysPolicy struct {
	Rules []OverlayRule `json:"rules,omitempty"`
	// Advisor bounds what the advisor may PROPOSE per dispatch (allow/deny list,
	// per-dispatch cap). nil ⇒ compiled defaults (unbounded allow, empty deny,
	// max_skills_per_dispatch=2). Advisory adds; the clamp never widens policy.
	Advisor *AdvisorOverlayPolicy `json:"advisor,omitempty"`
}

// AdvisorOverlayPolicy is the operator's clamp on advisor-proposed skills.
type AdvisorOverlayPolicy struct {
	AllowList            []string `json:"allow_list,omitempty"`
	DenyList             []string `json:"deny_list,omitempty"`
	MaxSkillsPerDispatch int      `json:"max_skills_per_dispatch,omitempty"`
}

// defaultMaxSkillsPerDispatch is the compiled cap applied when the operator
// leaves overlays.advisor (or its max) unset.
const defaultMaxSkillsPerDispatch = 2

// AdvisorSkillRejection records one clamped-out advisor proposal. Reason is one
// of the literal strings "not-in-registry", "denylisted",
// "over-max-skills-per-dispatch" — logged to advisor-rejections.json, never a
// silent drop.
type AdvisorSkillRejection struct {
	Skill  string `json:"skill"`
	Reason string `json:"reason"`
}

// OverlayRule matches a dispatch when EVERY non-empty selector dimension
// matches. An empty dimension is a wildcard. Patterns are glob (path.Match), so
// a literal like "audit" matches only itself while "gpt-*" matches "gpt-5".
type OverlayRule struct {
	Phases []string `json:"phases,omitempty"`
	CLIs   []string `json:"clis,omitempty"`
	Models []string `json:"models,omitempty"`
	Tiers  []string `json:"tiers,omitempty"`
	Skills []string `json:"skills,omitempty"`
}

// OverlayDispatch is the descriptor a resolver call is keyed on — the phase,
// driver (cli), model, and capability tier of a single agent launch.
type OverlayDispatch struct {
	Phase string
	CLI   string
	Model string
	Tier  string
}

// compiledDefaultOverlays is the single source of the built-in overlay set —
// the ONLY Go literal tier→skills mapping in this package. Operators override
// it wholesale via policy.json's overlays block (absent ⇒ this default; empty
// rules ⇒ opt out).
func compiledDefaultOverlays() []OverlayRule {
	return []OverlayRule{
		{Tiers: []string{"deep", "top"}, Skills: []string{"fable"}},
	}
}

// ResolveOverlays returns the ordered, deduped skill list that applies to the
// given dispatch. See the file header for the nil/empty/rules contract.
func (p Policy) ResolveOverlays(d OverlayDispatch) []string {
	rules := compiledDefaultOverlays()
	if p.Overlays != nil {
		rules = p.Overlays.Rules // explicit block (possibly empty ⇒ opt-out)
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

// SkillRegistryFromFS reads the skill registry from the filesystem: every
// immediate subdirectory of skillsDir that contains a SKILL.md is a skill, its
// directory name the registry entry. A directory without SKILL.md is not a
// skill and is excluded. An empty (or SKILL.md-less) skillsDir is a valid,
// degenerate registry — it returns an empty list, not an error — so every
// proposal clamps to nothing rather than the loader failing on a legitimate
// empty state. This is the single source of the advisor's allowed skill names;
// no hand-maintained list exists (drift-proof, AC5).
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
			continue // no SKILL.md ⇒ not a skill
		}
		out = append(out, e.Name())
	}
	return out, nil
}

// ClampAdvisorSkills disposes of an advisor's proposed skill set against the
// filesystem registry and the operator's overlays.advisor block: "advisor
// proposes, kernel disposes". Accepted skills keep the advisor's proposal order
// (its own priority ordering — never re-sorted). Every clamped-out skill yields
// exactly one AdvisorSkillRejection with a literal reason, so nothing is
// silently dropped. Membership is verbatim string equality against the
// registry, so a proposal containing a path separator (or any string not a
// registry entry) never resolves — the clamp never joins/normalizes a proposal
// against the skills root before comparing (AC3 injection guard).
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

// ResolveOverlaysWithAdvisor returns the additive union of the static overlay
// rules (ResolveOverlays) and the already-clamped advisor skills, deduped in
// stable static-first order. A nil/empty advisor proposal is byte-identical to
// ResolveOverlays — advisory adds, never replaces policy (AC1). Callers MUST
// pass skills already run through ClampAdvisorSkills; this merge does not
// re-validate them against the registry.
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

// toSet builds a membership set from a string slice.
func toSet(ss []string) map[string]struct{} {
	m := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		m[s] = struct{}{}
	}
	return m
}

// matches reports whether every non-empty selector dimension of the rule is
// satisfied by the dispatch.
func (r OverlayRule) matches(d OverlayDispatch) bool {
	return matchDim(r.Phases, d.Phase) &&
		matchDim(r.CLIs, d.CLI) &&
		matchDim(r.Models, d.Model) &&
		matchDim(r.Tiers, d.Tier)
}

// matchDim reports whether value satisfies a selector dimension. An empty
// patterns slice is a wildcard (matches anything); otherwise value must
// glob-match at least one pattern.
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
