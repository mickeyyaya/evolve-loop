package subagent

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// conformance_registry_test.go — B6: keep the agent-role allow-list SSOT
// single-sourced and internally consistent.
//
// B1 collapsed the allow-list to one source (agentRoles → agentRolePattern).
// B6 closes the remaining drift surface: the allow-list and the on-disk
// profiles must agree (dispatch loads ProfilesDir/<role>.json, run.go:179), so
// adding a role in exactly one place is enforced rather than assumed. (Session
// reaping — the other B6 scale knob — is already covered by internal/swarm:
// TestReap_KillsAllLiveAndMarksReaped, TestReapRunSessions_KillsOwnRegistryOnly,
// et al.; not re-tested here per single-source.)

// repoProfilesDir locates the repo's .evolve/profiles by walking up from the
// working directory (absolute even under -trimpath) and, as a fallback, the
// source file's directory. It FAILS rather than skips when neither finds the
// dir: a drift-guard that can silently skip is no guard at all, so an
// unlocatable profiles dir must surface as a failure, not a hidden pass.
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

// TestAgentRoles_EveryRoleHasProfile enforces that the dispatch allow-list and
// the profiles are single-sourced: every canonical role must have a
// <role>.json profile. Adding a role to agentRoles without its profile would
// fail at dispatch (run.go:179 loads ProfilesDir/<role>.json); this catches the
// drift at test time instead.
func TestAgentRoles_EveryRoleHasProfile(t *testing.T) {
	profDir := repoProfilesDir(t)
	for _, role := range agentRoles {
		p := filepath.Join(profDir, role+".json")
		if _, err := os.Stat(p); err != nil {
			t.Errorf("agentRoles entry %q has no profile at %s — allow-list and profiles have drifted", role, p)
		}
	}
}

// TestAgentRoles_SSOTIntegrity pins that the allow-list has no duplicates and
// that the derived agentRolePattern matches exactly the canonical roles while
// rejecting non-members, case variants, and worker-name-shaped strings (which
// take the separate parseAgentName path, not the bare allow-list).
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
