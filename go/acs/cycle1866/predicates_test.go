//go:build acs

package cycle1866

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseinventory"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var spineProfiles = map[string]string{
	"build": "builder",
	"audit": "auditor",
	"tdd":   "tdd-engineer",
}

func realRegistryPath(root string) string {
	return filepath.Join(root, "docs", "architecture", "phase-registry.json")
}

func loadRealCatalog(t *testing.T, root string) phasespec.Catalog {
	t.Helper()
	cat, err := phasespec.Load(realRegistryPath(root))
	if err != nil {
		t.Fatalf("load the real phase registry: %v", err)
	}
	return cat
}

func profileOf(spec phasespec.PhaseSpec) string {
	return strings.TrimPrefix(spec.AgentName(), "evolve-")
}

func trackedProfilesAndPersonas(t *testing.T, root string) map[string]bool {
	t.Helper()
	out, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--", ".evolve/profiles", "agents")
	if err != nil || code != 0 {
		t.Fatalf("git ls-files exit=%d: %v", code, err)
	}
	tracked := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		tracked[line] = true
	}
	return tracked
}

func TestC1866_001_EveryLLMCatalogPhaseResolvesToTrackedProfileAndPersona(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cat := loadRealCatalog(t, root)
	tracked := trackedProfilesAndPersonas(t, root)
	checked := 0
	for _, name := range cat.Names() {
		spec, _ := cat.Get(name)
		if spec.KindOrDefault() != "llm" {
			continue
		}
		checked++
		profile := ".evolve/profiles/" + profileOf(spec) + ".json"
		if !tracked[profile] {
			t.Errorf("phase %s: AgentName %q resolves to profile %s, which is not a tracked profile", name, spec.AgentName(), profile)
		}
		persona := "agents/" + spec.AgentName() + ".md"
		if !tracked[persona] {
			t.Errorf("phase %s: AgentName %q has no tracked persona %s", name, spec.AgentName(), persona)
		}
	}
	if checked < len(spineProfiles) {
		t.Fatalf("only %d llm phases in the real registry; the catalog failed to load its spine", checked)
	}
	for phase, want := range spineProfiles {
		spec, ok := cat.Get(phase)
		if !ok {
			t.Errorf("spine phase %s missing from the real registry", phase)
			continue
		}
		if got := profileOf(spec); got != want {
			t.Errorf("phase %s resolves to profile %q (AgentName %q), want %q", phase, got, spec.AgentName(), want)
		}
	}
}

type fixedCatalog map[string]phasespec.PhaseSpec

func (c fixedCatalog) Get(name string) (phasespec.PhaseSpec, bool) {
	spec, ok := c[name]
	return spec, ok
}

func (c fixedCatalog) Names() []string {
	names := make([]string, 0, len(c))
	for name := range c {
		names = append(names, name)
	}
	return names
}

func profileSource(names ...string) cliroute.ProfileSource {
	files := fstest.MapFS{}
	for _, name := range names {
		files[name+".json"] = &fstest.MapFile{Data: []byte(`{"name":"` + name + `"}`)}
	}
	return profiles.NewFromFS(files)
}

func routingPolicy(agentKeys ...string) policy.Policy {
	agents := map[string]policy.AgentRule{}
	for _, key := range agentKeys {
		agents[key] = policy.AgentRule{CLI: []string{"claude"}}
	}
	return policy.Policy{CLIRouting: &policy.CLIRouting{CLIs: []string{"claude"}, Agents: agents}}
}

func collision(findings []cliroute.Finding) (cliroute.Finding, bool) {
	for _, f := range findings {
		if f.Severity == cliroute.SeverityError && strings.Contains(f.Message, "already sets") {
			return f, true
		}
	}
	return cliroute.Finding{}, false
}

func TestC1866_002_CLIRoutingResolvesPhaseAgentFromCatalogAlone(t *testing.T) {
	buildDeclaringForgeSmith := fixedCatalog{
		"build": {Name: "build", Agent: "evolve-forge-smith", Role: "build"},
	}
	profs := profileSource("builder", "forge-smith")

	_, findings := cliroute.Compile(routingPolicy("build", "forge-smith"), buildDeclaringForgeSmith, profs)
	if _, ok := collision(findings); !ok {
		t.Errorf("agents.build and agents.forge-smith did not collide: build must resolve to the catalog's agent forge-smith, findings=%+v", findings)
	}

	_, findings = cliroute.Compile(routingPolicy("build", "builder"), buildDeclaringForgeSmith, profs)
	if f, ok := collision(findings); ok {
		t.Errorf("agents.build collided with agents.builder although the catalog names forge-smith; a second phase-to-agent table still answers: %+v", f)
	}
}

func TestC1866_003_CLIRoutingRealRegistryStillMapsSpineToTrackedProfiles(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cat := loadRealCatalog(t, root)
	profs := profiles.NewFromDir(filepath.Join(root, ".evolve", "profiles"))
	for phase, profile := range spineProfiles {
		_, findings := cliroute.Compile(routingPolicy(phase, profile), cat, profs)
		if _, ok := collision(findings); !ok {
			t.Errorf("agents.%s did not resolve to profile %s over the real registry, findings=%+v", phase, profile, findings)
		}
	}
	_, findings := cliroute.Compile(routingPolicy("no-such-phase"), cat, profs)
	if !hasNeitherFinding(findings, "no-such-phase") {
		t.Errorf("an unknown agents key must be refused as neither a profile nor a phase, findings=%+v", findings)
	}
}

func hasNeitherFinding(findings []cliroute.Finding, key string) bool {
	for _, f := range findings {
		if f.Key == "cli_routing.agents."+key && strings.Contains(f.Message, "neither a tracked profile nor a phase") {
			return true
		}
	}
	return false
}

func TestC1866_004_PhaseInventoryProjectsTheCatalogAgent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cat := loadRealCatalog(t, root)
	project := t.TempDir()
	emptyUserRoot := t.TempDir()
	res, err := phaseinventory.Build(phaseinventory.Options{
		ProjectRoot:  project,
		RegistryPath: realRegistryPath(root),
		Roots:        []string{emptyUserRoot},
		Force:        true,
	})
	if err != nil {
		t.Fatalf("phaseinventory.Build: %v", err)
	}
	raw, err := os.ReadFile(res.OutputPath)
	if err != nil {
		t.Fatalf("read inventory: %v", err)
	}
	var inv phaseinventory.Inventory
	if err := json.Unmarshal(raw, &inv); err != nil {
		t.Fatalf("parse inventory: %v", err)
	}
	agents := map[string]string{}
	for _, entry := range inv.Phases {
		agents[entry.Name] = entry.Agent
		if spec, ok := cat.Get(entry.Name); ok && entry.Agent != spec.AgentName() {
			t.Errorf("inventory agent for %s = %q, catalog AgentName = %q", entry.Name, entry.Agent, spec.AgentName())
		}
	}
	for phase, profile := range spineProfiles {
		got, ok := agents[phase]
		if !ok {
			t.Errorf("inventory has no entry for spine phase %s", phase)
			continue
		}
		if strings.TrimPrefix(got, "evolve-") != profile {
			t.Errorf("inventory shows agent %q for %s, want one resolving to tracked profile %s", got, phase, profile)
		}
	}
}
