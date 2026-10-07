package cliroute

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

const (
	claudeFamily  = "claude"
	ruleDefault   = "default"
	ruleProfile   = "profile"
	agentsPrefix  = "agents:"
	workPrefix    = "work:"
	afterChainOn  = "other_clis"
	afterChainOff = "stop"
)

var knownRoles = []string{string(phasespec.RolePlan), string(phasespec.RoleBuild), string(phasespec.RoleEvaluate), string(phasespec.RoleControl)}

type Table struct {
	declared  bool
	clis      []string
	chains    map[string][]string
	models    map[string]string
	tiers     map[string][]string
	stop      bool
	pol       policy.Policy
	catalog   Catalog
	profiles  map[string]loadedProfile
	roles     map[string][]phasespec.Role
	agentKeys map[string]string
	findings  []Finding
}

type loadedProfile struct {
	profile profiles.Profile
	err     error
}

type roleSel struct {
	role phasespec.Role
	ok   bool
}

type selection struct {
	rule    string
	chain   []string
	dropped []string
}

func (t Table) Findings() []Finding {
	return slices.Clone(t.findings)
}

func (t Table) profile(agent string) (*profiles.Profile, error) {
	loaded, listed := t.profiles[agent]
	if !listed {
		return nil, nil
	}
	if loaded.err != nil {
		return nil, fmt.Errorf("profile %s does not load: %w", agent, loaded.err)
	}
	p := loaded.profile
	return &p, nil
}

func (t Table) legacyProfile(agent string) *profiles.Profile {
	loaded, listed := t.profiles[agent]
	if !listed || loaded.err != nil {
		return nil
	}
	p := loaded.profile
	return &p
}

func (t Table) allowed(agent string, prof *profiles.Profile) []string {
	floored := profiles.IsClaudeFamilyFloor(agent)
	out := []string{}
	for _, fam := range t.clis {
		if (!floored || fam == claudeFamily) && prof.AllowsFamily(fam) {
			out = append(out, fam)
		}
	}
	return out
}

func (t Table) roleFor(req Request) roleSel {
	if req.Phase != "" {
		return roleSel{role: specOf(t.catalog, req.Phase).RoleOrDefault(), ok: true}
	}
	if roles := t.roles[req.Agent]; len(roles) == 1 {
		return roleSel{role: roles[0], ok: true}
	}
	return roleSel{}
}

func specOf(cat Catalog, phase string) phasespec.PhaseSpec {
	if cat != nil {
		if spec, ok := cat.Get(phasecontract.RegistryKey(phase)); ok {
			return spec
		}
	}
	return phasespec.PhaseSpec{Name: phase}
}

func (t Table) ruleFor(agent string, sel roleSel) (string, []string) {
	keys := []string{agentsPrefix + agent}
	if sel.ok {
		keys = append(keys, workPrefix+string(sel.role))
	}
	keys = append(keys, ruleDefault)
	for _, k := range keys {
		if chain, ok := t.chains[k]; ok {
			return k, chain
		}
	}
	return ruleProfile, nil
}

func (t Table) selectRule(agent string, sel roleSel, prof *profiles.Profile, allowed []string) (selection, *RuleError) {
	rule, chain := t.ruleFor(agent, sel)
	if rule == ruleProfile {
		chain = profileChain(prof)
	}
	kept, dropped := partition(chain, allowed)
	if strings.HasPrefix(rule, agentsPrefix) && len(dropped) > 0 {
		return selection{}, &RuleError{Rule: rule, Kind: RuleLeak, Dropped: dropped, Allowed: allowed}
	}
	if len(kept) == 0 {
		return selection{}, &RuleError{Rule: rule, Kind: RuleEmpty, Dropped: dropped, Allowed: allowed}
	}
	return selection{rule: rule, chain: kept, dropped: dropped}, nil
}

func (t Table) withAfterChain(plan llmroute.Plan, allowed []string) llmroute.Plan {
	if t.stop {
		return plan
	}
	return llmroute.ApplyUniversalFallback(plan, afterChainTail(allowed))
}

func (t Table) ceilingRefusal(rule string, chain, tiers []string) *CeilingError {
	if len(t.tiers) == 0 {
		return nil
	}
	capped := llmroute.Plan{TierCeiling: t.tiers}
	for _, tier := range tiers {
		if slices.ContainsFunc(chain, func(cli string) bool { return capped.Permits(cli, tier) }) {
			return nil
		}
	}
	return &CeilingError{Rule: rule, Tiers: slices.Clone(tiers), Chain: slices.Clone(chain), Ceiling: cloneCeiling(t.tiers)}
}

func afterChainTail(allowed []string) []string {
	out := make([]string, 0, len(allowed))
	for _, fam := range allowed {
		out = append(out, llmroute.DefaultDriverForFamily(fam))
	}
	return out
}

func profileChain(prof *profiles.Profile) []string {
	if prof == nil || prof.CLI == "" {
		return nil
	}
	return llmroute.ChainFor(prof.CLI, prof).Candidates
}

func partition(chain, allowed []string) (kept, dropped []string) {
	for _, c := range chain {
		if slices.Contains(allowed, familyOf(c)) {
			kept = append(kept, c)
		} else {
			dropped = append(dropped, c)
		}
	}
	return kept, dropped
}

func leadWith(primary string, chain []string) []string {
	if primary == "" {
		return chain
	}
	out := []string{primary}
	for _, c := range chain {
		if c != primary {
			out = append(out, c)
		}
	}
	return out
}

func cloneCeiling(in map[string][]string) map[string][]string {
	out := make(map[string][]string, len(in))
	for tier, families := range in {
		out[tier] = slices.Clone(families)
	}
	return out
}
