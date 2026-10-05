package cliroute

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type CompileOption func(*compiler)

func WithToolCapable(fn func(driver string) bool) CompileOption {
	return func(c *compiler) { c.toolCapable = fn }
}

func Compile(p policy.Policy, cat Catalog, profs ProfileSource, opts ...CompileOption) (Table, []Finding) {
	c := &compiler{pol: p, cat: cat, agentKeys: map[string]string{}}
	for _, opt := range opts {
		opt(c)
	}
	t := Table{pol: p, catalog: cat}
	t.profiles = c.snapshotProfiles(profs)
	if p.CLIRouting != nil {
		c.block = *p.CLIRouting
		t = c.compileDeclared(t)
	}
	t.findings = slices.Clone(c.findings)
	return t, slices.Clone(c.findings)
}

type compiler struct {
	block       policy.CLIRouting
	pol         policy.Policy
	cat         Catalog
	toolCapable func(string) bool
	names       []string
	agentKeys   map[string]string
	findings    []Finding
}

func (c *compiler) compileDeclared(t Table) Table {
	t.declared = true
	t.clis = c.compileCLIs()
	t.chains = c.compileSharedChains()
	agentChains, models := c.compileAgents()
	maps.Copy(t.chains, agentChains)
	t.models = models
	t.tiers = c.compileTiers(t.clis)
	t.stop = c.compileAfterChain()
	t.roles = c.agentRoles()
	t.agentKeys = maps.Clone(c.agentKeys)
	c.checkTwoSources()
	c.checkAgents(t)
	return t
}

func (c *compiler) add(severity Severity, key, format string, args ...any) {
	f := Finding{Severity: severity, Key: key, Message: fmt.Sprintf(format, args...)}
	if !slices.Contains(c.findings, f) {
		c.findings = append(c.findings, f)
	}
}

func (c *compiler) snapshotProfiles(profs ProfileSource) map[string]loadedProfile {
	snapshot := map[string]loadedProfile{}
	if profs == nil {
		c.add(SeverityError, "profiles", "no profile source was given, so no agent can be checked or resolved")
		return snapshot
	}
	names, err := profs.List()
	if err != nil {
		c.add(SeverityError, "profiles", "listing the profiles failed: %v", err)
		return snapshot
	}
	sort.Strings(names)
	c.names = names
	for _, name := range names {
		p, err := profs.Get(name)
		snapshot[name] = loadedProfile{profile: p, err: err}
	}
	return snapshot
}

func (c *compiler) isProfile(name string) bool {
	_, found := slices.BinarySearch(c.names, name)
	return found
}

func (c *compiler) driver(key, entry string) (string, bool) {
	d := llmroute.DefaultDriverForFamily(strings.TrimSpace(entry))
	if !llmroute.KnownDriver(d) {
		c.add(SeverityError, key, "unknown family or driver %q (families: claude, codex, agy, ollama; or a registered driver)", entry)
		return "", false
	}
	if c.toolCapable != nil && !c.toolCapable(d) {
		c.add(SeverityError, key, "%s cannot use tools, so it cannot run a phase", d)
		return "", false
	}
	return d, true
}

func (c *compiler) compileCLIs() []string {
	var out []string
	for _, entry := range c.block.CLIs {
		d, ok := c.driver("cli_routing.clis", entry)
		if ok && !slices.Contains(out, llmroute.Family(d)) {
			out = append(out, llmroute.Family(d))
		}
	}
	if !slices.Contains(out, claudeFamily) {
		c.add(SeverityError, "cli_routing.clis", "claude must be listed: the Claude-family floor agents run only on claude")
	}
	return out
}

func (c *compiler) compileChain(key string, entries []string) ([]string, bool) {
	if len(entries) == 0 {
		c.add(SeverityError, key, "an empty chain runs nothing; list at least one CLI")
		return nil, false
	}
	var out []string
	for _, entry := range entries {
		if d, ok := c.driver(key, entry); ok && !slices.Contains(out, d) {
			out = append(out, d)
		}
	}
	return out, len(out) > 0
}

func (c *compiler) compileSharedChains() map[string][]string {
	chains := map[string][]string{}
	if c.block.Default != nil {
		if chain, ok := c.compileChain("cli_routing.default", c.block.Default); ok {
			chains[ruleDefault] = chain
		}
	}
	for _, role := range sortedKeys(c.block.Work) {
		key := "cli_routing.work." + role
		if !slices.Contains(knownRoles, role) {
			c.add(SeverityError, key, "unknown role %q (roles: %s)", role, strings.Join(knownRoles, ", "))
			continue
		}
		if chain, ok := c.compileChain(key, c.block.Work[role]); ok {
			chains[workPrefix+role] = chain
		}
	}
	return chains
}

func (c *compiler) compileAgents() (map[string][]string, map[string]string) {
	chains, models := map[string][]string{}, map[string]string{}
	for _, key := range sortedKeys(c.block.Agents) {
		path := "cli_routing.agents." + key
		agent, ok := c.agentOf(key)
		if !ok {
			c.add(SeverityError, path, "%q is neither a tracked profile nor a phase of the merged catalog", key)
			continue
		}
		if prev, taken := c.agentKeys[agent]; taken {
			c.add(SeverityError, path, "names agent %s, which agents.%s already sets", agent, prev)
			continue
		}
		c.agentKeys[agent] = key
		rule := c.block.Agents[key]
		if chain, ok := c.compileChain(path, rule.CLI); ok {
			chains[agentsPrefix+agent] = chain
		}
		if rule.Model == "" {
			continue
		}
		if !slices.Contains(policy.TierNames(), rule.Model) {
			c.add(SeverityError, path+".model", "unknown tier %q (tiers: %s)", rule.Model, strings.Join(policy.TierNames(), ", "))
			continue
		}
		models[agent] = rule.Model
	}
	return chains, models
}

func (c *compiler) agentOf(key string) (string, bool) {
	if c.isProfile(key) {
		return key, true
	}
	return c.agentOfPhase(key)
}

func (c *compiler) agentOfPhase(phase string) (string, bool) {
	if contract, ok := phasecontract.For(phase); ok && c.isProfile(contract.AgentName) {
		return contract.AgentName, true
	}
	if c.cat == nil {
		return "", false
	}
	spec, ok := c.cat.Get(phasecontract.RegistryKey(phase))
	if !ok {
		return "", false
	}
	agent := strings.TrimPrefix(spec.AgentName(), "evolve-")
	return agent, c.isProfile(agent)
}

func (c *compiler) compileTiers(clis []string) map[string][]string {
	out := map[string][]string{}
	for _, tier := range sortedKeys(c.block.Tiers) {
		key := "cli_routing.tiers." + tier
		if !slices.Contains(policy.TierNames(), tier) {
			c.add(SeverityError, key, "unknown tier %q (tiers: %s)", tier, strings.Join(policy.TierNames(), ", "))
			continue
		}
		if len(c.block.Tiers[tier]) == 0 {
			c.add(SeverityError, key, "an empty ceiling leaves no CLI at %s", tier)
		}
		out[tier] = c.ceilingFamilies(key, c.block.Tiers[tier], clis)
	}
	return out
}

func (c *compiler) ceilingFamilies(key string, entries, clis []string) []string {
	families := []string{}
	for _, entry := range entries {
		d, ok := c.driver(key, entry)
		if !ok {
			continue
		}
		if fam := llmroute.Family(d); slices.Contains(clis, fam) {
			families = append(families, fam)
		} else {
			c.add(SeverityError, key, "%s is not in clis %v", fam, clis)
		}
	}
	return families
}

func (c *compiler) compileAfterChain() bool {
	switch c.block.AfterChain {
	case "", afterChainOn:
		return false
	case afterChainOff:
		return true
	}
	c.add(SeverityError, "cli_routing.after_chain", "unknown value %q (values: %s, %s)", c.block.AfterChain, afterChainOn, afterChainOff)
	return false
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
