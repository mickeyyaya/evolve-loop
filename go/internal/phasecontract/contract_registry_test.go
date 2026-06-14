package phasecontract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Layer 1 of the deliverable-contract feature (ADR-0034): the Contract registry
// is the single source of truth for WHERE each agent writes its deliverable,
// WHAT kind it is (markdown report vs JSON artifact), and the well-formedness
// rules. These RED tests pin the contract before the implementation exists.

func TestFor_CoversAllEightAgents(t *testing.T) {
	want := []string{
		"build", "scout", "tdd", "audit", "intent", "triage", // 6 phase agents
		"router",       // the LLM routing brain (a.k.a. advisor) — routing-plan.json
		"orchestrator", // host-side driver — cycle-state.json
	}
	for _, phase := range want {
		c, ok := For(phase)
		if !ok {
			t.Errorf("For(%q): want a contract, got none", phase)
			continue
		}
		if c.Phase != phase {
			t.Errorf("For(%q): Phase=%q, want %q", phase, c.Phase, phase)
		}
		if c.ArtifactName == "" {
			t.Errorf("For(%q): empty ArtifactName", phase)
		}
	}
}

// ship's deliverable is the pushed commit, not a file — but it must still
// resolve a contract so the contract gate is EXPLICIT (PASS) rather than
// fail-open-on-unknown, which logged "no contract registered for phase
// \"ship\"" every cycle. The contract declares NoArtifact so the verifier
// treats it as trivially well-formed.
func TestFor_Ship_NoArtifactContract(t *testing.T) {
	c, ok := For("ship")
	if !ok {
		t.Fatal(`For("ship"): want a contract (ship had none → contract-gate fail-open WARN every cycle)`)
	}
	if !c.NoArtifact {
		t.Error("ship contract must declare NoArtifact (its deliverable is the pushed commit)")
	}
	if c.ArtifactName != "" {
		t.Errorf("ship NoArtifact contract should have empty ArtifactName, got %q", c.ArtifactName)
	}
}

// Completeness: every mandatory spine phase (scout/build/audit/ship) and the
// conditional-mandatory tdd must have a registered contract, so a phase added
// to the spine can never again silently lack one — the exact ship gap that let
// the contract gate fail open every cycle.
func TestMandatoryPhasesHaveContracts(t *testing.T) {
	for _, p := range []string{"scout", "build", "audit", "ship", "tdd"} {
		if _, ok := For(p); !ok {
			t.Errorf("mandatory phase %q has no registered contract", p)
		}
	}
}

func TestFor_UnknownPhase(t *testing.T) {
	if c, ok := For("nope"); ok {
		t.Errorf("For(unknown): want (_, false), got %+v", c)
	}
}

func TestContract_KindDispatch(t *testing.T) {
	cases := map[string]Kind{
		"build":        KindMarkdown,
		"audit":        KindMarkdown,
		"router":       KindJSON,
		"orchestrator": KindJSON,
	}
	for phase, wantKind := range cases {
		c, _ := For(phase)
		if c.Kind != wantKind {
			t.Errorf("For(%q).Kind=%d, want %d", phase, c.Kind, wantKind)
		}
	}
}

func TestContract_ArtifactNames(t *testing.T) {
	cases := map[string]string{
		"build":        "build-report.md",
		"scout":        "scout-report.md",
		"tdd":          "test-report.md", // NOT tdd-report.md — runtime truth (hook)
		"audit":        "audit-report.md",
		"intent":       "intent.md",
		"triage":       "triage-report.md",
		"router":       "routing-plan.json",
		"orchestrator": "cycle-state.json",
	}
	for phase, want := range cases {
		c, _ := For(phase)
		if c.ArtifactName != want {
			t.Errorf("For(%q).ArtifactName=%q, want %q", phase, c.ArtifactName, want)
		}
	}
}

func TestContract_ArtifactPath_WorkspaceTarget(t *testing.T) {
	c, _ := For("build")
	r := Roots{Workspace: "/ws", Worktree: "/wt", EvolveDir: "/ev"}
	if got, want := c.ArtifactPath(r), filepath.Join("/ws", "build-report.md"); got != want {
		t.Errorf("build ArtifactPath=%q, want %q", got, want)
	}
}

func TestContract_ArtifactPath_EvolveDirTarget(t *testing.T) {
	// orchestrator's cycle-state.json lives in .evolve/, not the cycle workspace.
	c, _ := For("orchestrator")
	r := Roots{Workspace: "/ws", Worktree: "/wt", EvolveDir: "/ev"}
	if got, want := c.ArtifactPath(r), filepath.Join("/ev", "cycle-state.json"); got != want {
		t.Errorf("orchestrator ArtifactPath=%q, want %q", got, want)
	}
}

func TestContract_WriteTarget(t *testing.T) {
	workspaceAgents := []string{"build", "scout", "tdd", "audit", "intent", "triage", "router"}
	for _, phase := range workspaceAgents {
		c, _ := For(phase)
		if c.WriteTarget != "workspace" {
			t.Errorf("For(%q).WriteTarget=%q, want workspace", phase, c.WriteTarget)
		}
	}
	if c, _ := For("orchestrator"); c.WriteTarget != "evolve_dir" {
		t.Errorf("orchestrator WriteTarget=%q, want evolve_dir", c.WriteTarget)
	}
}

func TestContracts_ReturnsWholeRegistry(t *testing.T) {
	all := Contracts()
	if len(all) != 9 {
		t.Fatalf("Contracts() len=%d, want 9", len(all))
	}
	seen := map[string]bool{}
	for _, c := range all {
		seen[c.Phase] = true
	}
	for _, phase := range []string{"build", "scout", "tdd", "audit", "intent", "triage", "router", "orchestrator", "ship"} {
		if !seen[phase] {
			t.Errorf("Contracts() missing %q", phase)
		}
	}
}

func TestContract_JSONContractsDeclareRequiredKeys(t *testing.T) {
	// router/advisor writes a BARE JSON ARRAY (routing-plan.json) — no required
	// keys (router-contract-bare-array-vs-plan-key). The alias must still
	// resolve to the canonical router contract.
	advisor, _ := For("advisor")
	if len(advisor.RequiredKeys) != 0 {
		t.Errorf("advisor/router RequiredKeys=%v, want none (bare array)", advisor.RequiredKeys)
	}
	if advisor.Phase != "router" {
		t.Errorf("advisor alias should resolve to canonical router; got Phase=%q", advisor.Phase)
	}
	orch, _ := For("orchestrator")
	if !contains(orch.RequiredKeys, "cycle_id") {
		t.Errorf("orchestrator RequiredKeys=%v, want to contain 'cycle_id'", orch.RequiredKeys)
	}
}

// TestArtifactNameMatchesProfileOutput is the DRIFT-DETECTOR (mirrors
// TestProducersDeclareCanonical): the contract's ArtifactName must equal the
// basename of the agent profile's output_artifact, so the two path-resolution
// mechanisms (the in-process runner hook AND subagent.resolveArtifactPath which
// reads profile.output_artifact) can never disagree again. advisor (no standard
// profile) and orchestrator (host-side machine state, not a persona report) are
// exempt and checked separately.
func TestArtifactNameMatchesProfileOutput(t *testing.T) {
	profileDir := filepath.Join("..", "..", "..", ".evolve", "profiles")
	// contract phase -> profile basename (profile name = agent name)
	profileOf := map[string]string{
		"build":  "builder",
		"scout":  "scout",
		"tdd":    "tdd-engineer",
		"audit":  "auditor",
		"intent": "intent",
		"triage": "triage",
	}
	for phase, profName := range profileOf {
		c, _ := For(phase)
		path := filepath.Join(profileDir, profName+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s: read profile %s: %v", phase, path, err)
			continue
		}
		var prof struct {
			OutputArtifact string `json:"output_artifact"`
		}
		if err := json.Unmarshal(data, &prof); err != nil {
			t.Errorf("%s: parse profile: %v", phase, err)
			continue
		}
		if got := filepath.Base(prof.OutputArtifact); got != c.ArtifactName {
			t.Errorf("DRIFT %s: profile output_artifact basename=%q, contract ArtifactName=%q",
				phase, got, c.ArtifactName)
		}
	}
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
