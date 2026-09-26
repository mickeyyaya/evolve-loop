package phasecoherence

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/repostate"
)

func TestCoherence_UnpairedPersonaWarns(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{"evolve-debugger": personaMD("debugger", `tools: ["Read", "Bash"]`)},
		map[string]string{},
	)
	vs, err := Check(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	var hit *Violation
	for i := range vs {
		if vs[i].Persona == "debugger" && vs[i].Kind == "unpaired" {
			hit = &vs[i]
		}
	}
	if hit == nil {
		t.Fatalf("RED (cycle-270): persona without profile produced no 'unpaired' violation (got %+v) — the gap class stays invisible until launch exit=10", vs)
	}
	if hit.Severity != "WARN" {
		t.Errorf("unpaired severity = %s, want WARN (visibility, not a hard gate — eval C1 needs exit 0 on live tree)", hit.Severity)
	}
	if !strings.Contains(hit.Message, "debugger.json") {
		t.Errorf("unpaired message must name the missing profile path; got %q", hit.Message)
	}
}

func TestCoherence_ReferencePersonaIsDocumentation(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{"evolve-auditor-reference": personaMD("auditor-reference", `tools: ["Read"]`)},
		map[string]string{},
	)
	vs, err := Check(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(vs) != 0 {
		t.Errorf("-reference persona is documentation, never dispatched — want no violations, got %+v", vs)
	}
}

// repoRootForPairing returns the repo root four directories above this file; every real-tree test uses it.
func repoRootForPairing(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))))
	if _, err := os.Stat(filepath.Join(root, "agents")); err != nil {
		t.Skipf("repo layout not found from %s: %v", thisFile, err)
	}
	return root
}

// trackedProfiles returns the git-tracked profile names. An untracked profile is a runtime
// mint that never reaches main, so the profile-to-persona check must not bind it.
func trackedProfiles(root string) (map[string]bool, error) {
	return repostate.TrackedSet(root, ".evolve/profiles", ".json")
}

func TestRepoPersonaProfilePairing(t *testing.T) {
	root := repoRootForPairing(t)

	personaOnly := map[string]string{
		"operator":      "human-operator playbook, never machine-dispatched",
		"swarm-planner": "swarm is EVOLVE_SWARM_STAGE=shadow; MUST be paired before swarm promotion (plan task CF.2)",
	}
	// Untracked runtime mints need no entry here: trackedProfiles excludes them structurally.
	profileOnly := map[string]string{
		"evaluator":   "persona projected from skills/evaluator",
		"inspirer":    "persona projected from skills/inspirer",
		"tool-policy": "shared tool-policy fragment, not an agent",
	}

	agentEntries, err := os.ReadDir(filepath.Join(root, "agents"))
	if err != nil {
		t.Fatalf("read agents/: %v", err)
	}
	profDir := filepath.Join(root, ".evolve", "profiles")

	for _, e := range agentEntries {
		n := e.Name()
		if e.IsDir() || !strings.HasPrefix(n, "evolve-") || !strings.HasSuffix(n, ".md") {
			continue
		}
		name := strings.TrimPrefix(strings.TrimSuffix(n, ".md"), "evolve-")
		if strings.HasSuffix(name, "-reference") {
			continue // documentation by convention
		}
		if why, ok := personaOnly[name]; ok {
			t.Logf("allowlisted persona-only %q: %s", name, why)
			continue
		}
		if _, err := os.Stat(filepath.Join(profDir, name+".json")); err != nil {
			t.Errorf("persona agents/%s has no profile .evolve/profiles/%s.json — dispatch dies exit=10 at launch (cycle-270 class); pair it or allowlist it here with a reason", n, name)
		}
	}

	profEntries, err := os.ReadDir(profDir)
	if err != nil {
		t.Fatalf("read profiles/: %v", err)
	}
	tracked, terr := trackedProfiles(root)
	if terr == nil && len(tracked) == 0 {
		// A pathspec that matches nothing exits 0, so an empty set on the real tree means a
		// misresolved root or sparse checkout; accepting it would unbind the whole gate.
		terr = fmt.Errorf("empty tracked-profile set at %s — pathspec matched nothing", root)
	}
	if terr != nil {
		// Without git context, bind every on-disk profile: the stricter direction.
		t.Logf("trackedProfiles: %v — binding all on-disk profiles", terr)
		tracked = nil
	}
	for _, e := range profEntries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".json") {
			continue
		}
		name := strings.TrimSuffix(n, ".json")
		if tracked != nil && !tracked[name] {
			t.Logf("untracked profile %q: runtime-minted state, not bound", name)
			continue
		}
		if why, ok := profileOnly[name]; ok {
			t.Logf("allowlisted profile-only %q: %s", name, why)
			continue
		}
		_, errA := os.Stat(filepath.Join(root, "agents", "evolve-"+name+".md"))
		_, errB := os.Stat(filepath.Join(root, "agents", name+".md"))
		if errA != nil && errB != nil {
			t.Errorf("profile .evolve/profiles/%s has no persona at agents/evolve-%s.md or agents/%s.md — a profile with no prompt source is dead config; delete it or allowlist with a reason", n, name, name)
		}
	}
}
