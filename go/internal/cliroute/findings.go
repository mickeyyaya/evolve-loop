package cliroute

import (
	"slices"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

const migrateHint = "cli_routing replaces it; move it into the block with evolve cli-routing migrate"

type agentView struct {
	name    string
	prof    *profiles.Profile
	allowed []string
}

type familyPair struct {
	key    string
	names  [2]string
	chains [2][]string
}

func (c *compiler) checkTwoSources() {
	if len(c.pol.Pins) > 0 {
		c.add(SeverityError, "pins", "is set together with cli_routing — %s", migrateHint)
	}
	if wf := c.pol.Workflow; wf != nil && wf.UniversalFallback != nil {
		c.add(SeverityError, "workflow.universal_fallback", "is set together with cli_routing — %s", migrateHint)
	}
	if wf := c.pol.Workflow; wf != nil && wf.UniversalFallbackExclude != nil {
		c.add(SeverityError, "workflow.universal_fallback_exclude", "is set together with cli_routing — %s", migrateHint)
	}
	if rt := c.pol.Router; rt != nil && rt.CLI != "" {
		c.add(SeverityError, "router.cli", "is set together with cli_routing — %s", migrateHint)
	}
	if rt := c.pol.Router; rt != nil && rt.Model != "" {
		c.add(SeverityError, "router.model", "is set together with cli_routing — %s", migrateHint)
	}
}

func (c *compiler) agentRoles() map[string][]phasespec.Role {
	out := map[string][]phasespec.Role{}
	add := func(phase string) {
		agent, ok := c.agentOfPhase(phase)
		role := specOf(c.cat, phase).RoleOrDefault()
		if ok && !slices.Contains(out[agent], role) {
			out[agent] = append(out[agent], role)
		}
	}
	if c.cat != nil {
		for _, name := range c.cat.Names() {
			add(name)
		}
	}
	for _, contract := range phasecontract.Contracts() {
		add(contract.Phase)
	}
	for _, roles := range out {
		slices.Sort(roles)
	}
	return out
}

func selectionsOf(roles []phasespec.Role) []roleSel {
	out := make([]roleSel, 0, len(roles)+1)
	for _, role := range roles {
		out = append(out, roleSel{role: role, ok: true})
	}
	if len(roles) != 1 {
		out = append(out, roleSel{})
	}
	return out
}

func (c *compiler) checkAgents(t Table) {
	primaries := map[string][]string{}
	for _, name := range c.names {
		prof, err := t.profile(name)
		if err != nil {
			c.add(SeverityError, "profiles."+name, "%v", err)
			continue
		}
		view := agentView{name: name, prof: prof, allowed: t.allowed(name, prof)}
		for _, sel := range selectionsOf(t.roles[name]) {
			kept := c.checkSelection(t, view, sel)
			if _, set := primaries[name]; !set {
				primaries[name] = kept
			}
		}
		c.checkFloorTiers(t, name, *prof)
	}
	c.checkCrossFamily(t, primaries)
}

func (c *compiler) checkSelection(t Table, a agentView, sel roleSel) []string {
	picked, ruleErr := t.selectRule(a.name, sel, a.prof, a.allowed)
	if ruleErr == nil {
		c.checkCeiling(t, a, picked)
		return picked.chain
	}
	key := "agent." + a.name
	if ruleErr.Kind == RuleLeak {
		key = "cli_routing.agents." + c.agentKeys[a.name]
	}
	c.add(SeverityError, key, "%v", ruleErr)
	return nil
}

func (c *compiler) checkCeiling(t Table, a agentView, picked selection) {
	chain := t.withAfterChain(llmroute.Plan{Candidates: picked.chain}, a.allowed).Candidates
	for _, tier := range reachableTiers(*a.prof, t.models[a.name]) {
		tiers := llmroute.ApplySoftOverlay(llmroute.Plan{}, llmroute.Overlay{Tier: tier}, a.prof).Tiers
		if ceilingErr := t.ceilingRefusal(picked.rule, chain, tiers); ceilingErr != nil {
			c.add(SeverityError, "agent."+a.name+".tier_ceiling", "%v", ceilingErr)
		}
	}
}

func (c *compiler) checkFloorTiers(t Table, agent string, prof profiles.Profile) {
	if !profiles.IsClaudeFamilyFloor(agent) {
		return
	}
	for _, tier := range reachableTiers(prof, t.models[agent]) {
		tier = policy.TierName(policy.TierRank(tier))
		if ceiling, capped := t.tiers[tier]; capped && !slices.Contains(ceiling, claudeFamily) {
			c.add(SeverityError, "cli_routing.tiers."+tier, "excludes claude, but the floor agent %s runs at %s", agent, tier)
		}
	}
}

const unsetTierDispatchesAs = "balanced"

func reachableTiers(prof profiles.Profile, model string) []string {
	if model != "" {
		return []string{model}
	}
	start := prof.ModelTierDefault
	if start == "" {
		start = unsetTierDispatchesAs
	}
	tiers := []string{start}
	for _, situation := range sortedKeys(prof.ModelTierOverrides) {
		tiers = append(tiers, prof.ModelTierOverrides[situation])
	}
	return tiers
}

func (c *compiler) checkCrossFamily(t Table, primaries map[string][]string) {
	seen := map[string]bool{}
	for _, agent := range c.names {
		prof, err := t.profile(agent)
		if err != nil || prof.CrossFamilyWith == "" {
			continue
		}
		names := [2]string{agent, prof.CrossFamilyWith}
		sort.Strings(names[:])
		pair := familyPair{key: "cross_family_with." + names[0] + "+" + names[1], names: names}
		pair.chains = [2][]string{primaries[names[0]], primaries[names[1]]}
		if seen[pair.key] || len(pair.chains[0]) == 0 || len(pair.chains[1]) == 0 {
			continue
		}
		seen[pair.key] = true
		c.compareFamilies(pair, len(t.clis))
	}
}

func (c *compiler) compareFamilies(pair familyPair, familyCount int) {
	a, b := pair.chains[0], pair.chains[1]
	if fa, fb := familyOf(a[0]), familyOf(b[0]); fa == fb {
		severity := SeverityWarn
		if familyCount >= 2 {
			severity = SeverityError
		}
		c.add(severity, pair.key, "%s and %s both start on %s; cross_family_with needs different families", pair.names[0], pair.names[1], fa)
		return
	}
	if shared := sharedFamilies(a, b); len(shared) > 0 {
		c.add(SeverityWarn, pair.key, "%s and %s fall back onto the shared family %v", pair.names[0], pair.names[1], shared)
	}
}

func sharedFamilies(a, b []string) []string {
	var out []string
	for _, x := range a {
		fam := familyOf(x)
		if slices.ContainsFunc(b, func(y string) bool { return familyOf(y) == fam }) && !slices.Contains(out, fam) {
			out = append(out, fam)
		}
	}
	return out
}
