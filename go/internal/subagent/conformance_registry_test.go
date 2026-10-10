package subagent

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func repoProfilesDir(t *testing.T) string {
	t.Helper()
	var roots []string
	if wd, err := os.Getwd(); err == nil {
		roots = append(roots, wd)
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		roots = append(roots, filepath.Dir(file))
	}
	for _, root := range roots {
		dir := root
		for i := 0; i < 10; i++ {
			cand := filepath.Join(dir, ".evolve", "profiles")
			if info, err := os.Stat(cand); err == nil && info.IsDir() {
				return cand
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	t.Fatal("repo .evolve/profiles not found from cwd or source path — the role↔profile drift-guard cannot run; this must fail, not silently pass")
	return ""
}

func TestAgentRoles_EveryRoleHasProfile(t *testing.T) {
	profDir := repoProfilesDir(t)
	for _, role := range agentRoles {
		p := filepath.Join(profDir, role+".json")
		if _, err := os.Stat(p); err != nil {
			t.Errorf("agentRoles entry %q has no profile at %s — allow-list and profiles have drifted", role, p)
		}
	}
}

func TestAgentRoles_SSOTIntegrity(t *testing.T) {
	t.Parallel()
	if len(agentRoles) == 0 {
		t.Fatal("agentRoles is empty")
	}
	seen := map[string]bool{}
	for _, role := range agentRoles {
		if seen[role] {
			t.Errorf("duplicate role in agentRoles: %q", role)
		}
		seen[role] = true
		if !agentRolePattern.MatchString(role) {
			t.Errorf("agentRolePattern does not match its own canonical role %q", role)
		}
	}
	for _, bad := range []string{"", "Scout", "scout2", "not-a-role", "auditor-worker-x", "builder ", " builder", "scout|builder"} {
		if agentRolePattern.MatchString(bad) {
			t.Errorf("agentRolePattern should reject %q but matched it", bad)
		}
	}
}

func TestAgentRoles_DerivedFromPhaseContractRegistry(t *testing.T) {
	allowed := make(map[string]bool, len(agentRoles))
	for _, r := range agentRoles {
		allowed[r] = true
	}

	var registryAgents int
	for _, c := range phasecontract.Contracts() {
		if c.NoArtifact || c.AgentName == "" {
			if c.AgentName != "" && allowed[c.AgentName] {
				t.Errorf("agentRoles contains %q, a NoArtifact registry phase (%s) with no profile — the derivation over-reached", c.AgentName, c.Phase)
			}
			continue
		}
		registryAgents++
		if !allowed[c.AgentName] {
			t.Errorf("agentRoles is missing registry agent %q (phase %s) — the allow-list drifted from phasecontract", c.AgentName, c.Phase)
		}
	}
	if registryAgents == 0 {
		t.Fatal("premise broken: phasecontract registered no dispatchable agents")
	}

	for _, r := range nonRegistryRoles {
		if !allowed[r] {
			t.Errorf("agentRoles dropped non-registry role %q — the derivation must be a union, not a substitution", r)
		}
	}

	for i := 1; i < len(agentRoles); i++ {
		if agentRoles[i-1] > agentRoles[i] {
			t.Fatalf("agentRoles is not sorted at index %d (%q > %q) — derivation order is nondeterministic", i, agentRoles[i-1], agentRoles[i])
		}
	}
}
