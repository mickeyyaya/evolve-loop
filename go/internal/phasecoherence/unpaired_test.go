// unpaired_test.go — inbox dispatchable-agent-profile-completeness
// (2026-06-10T09-42Z): cycle-270's debugger persona existed, its profile
// didn't, and the route died exit=10 at launch with no earlier signal —
// because Check() silently skipped persona↔profile pairs with a missing
// side. These tests pin the visibility fix:
//
//  1. A persona without a profile → Kind "unpaired" WARN naming both paths
//     (the runner's typed fail-fast is the runtime half, landed cycle 276;
//     this is the look-ahead half).
//  2. "-reference" personas (auditor-reference, …) are documentation by
//     convention — never dispatched, no profile expected, NO violation.
//  3. TestRepoPersonaProfilePairing — the drift gate against the REAL tree:
//     every dispatchable persona is paired, with an explicit allowlist for
//     the intentional singletons. New unpaired personas fail CI here, which
//     is even earlier than batch preflight.
package phasecoherence

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCoherence_UnpairedPersonaWarns(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{"evolve-debugger": personaMD("debugger", `tools: ["Read", "Bash"]`)},
		map[string]string{}, // no profile — the cycle-270 shape
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

// repoRootForPairing walks up from this source file to the repo root (the
// directory containing agents/ and .evolve/profiles/).
func repoRootForPairing(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// <root>/go/internal/phasecoherence/unpaired_test.go → 4 Dir() calls
	// (file → phasecoherence → internal → go → <root>).
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))))
	if _, err := os.Stat(filepath.Join(root, "agents")); err != nil {
		t.Skipf("repo layout not found from %s: %v", thisFile, err)
	}
	return root
}

// TestRepoPersonaProfilePairing is the bijection drift gate on the live tree.
//
// Direction A (persona → profile): every agents/evolve-<name>.md must have
// .evolve/profiles/<name>.json, except documentation ("-reference") and the
// allowlisted intentional singletons. Adding a dispatchable persona without
// its profile fails THIS test at CI time instead of exit=10 at launch.
//
// Direction B (profile → persona): every profile must have a persona at
// agents/evolve-<name>.md or agents/<name>.md, except allowlisted
// non-persona profiles.
func TestRepoPersonaProfilePairing(t *testing.T) {
	root := repoRootForPairing(t)

	// Direction-A allowlist: personas that are intentionally undispatchable.
	personaOnly := map[string]string{
		"operator":      "human-operator playbook, never machine-dispatched",
		"swarm-planner": "swarm is EVOLVE_SWARM_STAGE=shadow; MUST be paired before swarm promotion (plan task CF.2)",
	}
	// Direction-B allowlist: profiles whose prompt source is not an
	// agents/evolve-*.md persona.
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
	for _, e := range profEntries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".json") {
			continue
		}
		name := strings.TrimSuffix(n, ".json")
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
