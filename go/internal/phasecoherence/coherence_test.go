// Package phasecoherence cross-checks the parallel hand-authored phase
// surfaces (agents/evolve-<name>.md persona frontmatter vs
// .evolve/profiles/<name>.json) for contradictions — Invariant-1 meta-gate
// (campaign retro §4, migration step 4; architecture-design Option B).
//
// coherence_test.go — cycle-238 task `persona-tools-coherence-gate`
// (RED first). API contract for Builder (architecture blueprint B6):
//
//	type Violation struct {
//	    Persona  string // base name, e.g. "builder" (evolve- prefix stripped)
//	    Kind     string // "disallowed" | "undeclared" (tools checks)
//	    Severity string // "WARN" for both drift directions
//	    Message  string // eval vocabulary: contradiction|mismatch|disallowed|undeclared
//	}
//	type Options struct {
//	    AgentsFS   fs.FS             // root CONTAINING agents/ (prompts.Loader layout)
//	    ProfilesFS fs.FS             // profiles dir root: <name>.json at top level (profiles.Loader layout)
//	    Overrides  map[string]string // persona name → OS file path substituting agents/evolve-<name>.md
//	}
//	func Check(opts Options) ([]Violation, error)
//
// Semantics pinned by these tests (architecture R4/R5/R6/R11):
//   - persona agents/evolve-<name>.md pairs with <name>.json; either side
//     missing → pair silently skipped; non `evolve-`-prefixed .md ignored.
//   - `tools:` frontmatter only; `tools-gemini:`/`tools-generic:` are NOT the
//     tools line (multi-CLI coherence is out of scope). No tools: line → skip.
//   - profile without allowed_tools → no constraint → skip.
//   - normalization: base name before "(" — "Bash" ↔ "Bash(x:*)", "Skill" ↔
//     "Skill(code-review-simplify)"; disallowed_tools is NOT consulted.
//   - Kind "disallowed": persona declares a tool absent from allowed_tools.
//   - Kind "undeclared": profile allows a tool the persona omits (live
//     builder drift, scout F1).
//   - both directions are Severity WARN (eval C1 needs exit 0 on live tree).
package phasecoherence

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// personaMD renders an agents/evolve-<name>.md body in the real frontmatter
// format.
func personaMD(name string, fmLines ...string) string {
	b := "---\nname: evolve-" + name + "\ndescription: test fixture\n"
	for _, l := range fmLines {
		b += l + "\n"
	}
	return b + "---\n\n# " + name + "\n"
}

// fixtures builds the two fs.FS roots: agentsFS keys are persona base names →
// frontmatter lines already rendered via personaMD (placed under agents/);
// profilesFS keys are profile base names → raw JSON.
func fixtures(personas map[string]string, profilesJSON map[string]string) (fstest.MapFS, fstest.MapFS) {
	agents := fstest.MapFS{}
	for name, body := range personas {
		agents["agents/"+name+".md"] = &fstest.MapFile{Data: []byte(body)}
	}
	profs := fstest.MapFS{}
	for name, body := range profilesJSON {
		profs[name+".json"] = &fstest.MapFile{Data: []byte(body)}
	}
	return agents, profs
}

func TestCoherence_CleanPairNoViolations(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{"evolve-widget": personaMD("widget", `tools: ["Read", "Grep", "Bash"]`)},
		map[string]string{"widget": `{"name":"widget","role":"widget","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read","Grep","Bash"]}`},
	)
	vs, err := Check(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(vs) != 0 {
		t.Errorf("violations = %+v, want none", vs)
	}
}

// TestCoherence_PersonaDeclares_Disallowed — eval persona-tools-coherence-gate
// C2 (name pinned): persona lists "Write", profile allowed_tools lacks Write.
func TestCoherence_PersonaDeclares_Disallowed(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{"evolve-widget": personaMD("widget", `tools: ["Read", "Write"]`)},
		map[string]string{"widget": `{"name":"widget","role":"widget","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read"]}`},
	)
	vs, err := Check(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(vs) != 1 {
		t.Fatalf("violations = %+v, want exactly 1", vs)
	}
	v := vs[0]
	if v.Persona != "widget" {
		t.Errorf("Persona = %q, want %q", v.Persona, "widget")
	}
	if v.Kind != "disallowed" {
		t.Errorf("Kind = %q, want %q", v.Kind, "disallowed")
	}
	if v.Severity != "WARN" {
		t.Errorf("Severity = %q, want %q", v.Severity, "WARN")
	}
	// Eval C4 greps "contradiction|mismatch|disallowed" — the Message must
	// carry the vocabulary and name the drifting tool.
	if !strings.Contains(v.Message, "Write") {
		t.Errorf("Message %q does not name the drifting tool Write", v.Message)
	}
	if !strings.Contains(v.Message, "disallowed") && !strings.Contains(v.Message, "contradiction") {
		t.Errorf("Message %q missing eval vocabulary (disallowed|contradiction)", v.Message)
	}
}

// TestCoherence_ProfileAllows_UndeclaredTool — eval C3 (name pinned): profile
// allows WebSearch, persona tools: omits it (the live builder drift, F1).
func TestCoherence_ProfileAllows_UndeclaredTool(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{"evolve-widget": personaMD("widget", `tools: ["Read"]`)},
		map[string]string{"widget": `{"name":"widget","role":"widget","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read","WebSearch"]}`},
	)
	vs, err := Check(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(vs) != 1 {
		t.Fatalf("violations = %+v, want exactly 1", vs)
	}
	v := vs[0]
	if v.Persona != "widget" || v.Kind != "undeclared" || v.Severity != "WARN" {
		t.Errorf("violation = %+v, want {Persona:widget Kind:undeclared Severity:WARN}", v)
	}
	if !strings.Contains(v.Message, "WebSearch") || !strings.Contains(v.Message, "undeclared") {
		t.Errorf("Message %q must name WebSearch and say undeclared", v.Message)
	}
}

func TestCoherence_BashParenNormalization(t *testing.T) {
	// R6: bare "Bash" in the persona covers scoped "Bash(...)" in the
	// profile in BOTH directions (no disallowed AND no undeclared report).
	agents, profs := fixtures(
		map[string]string{"evolve-widget": personaMD("widget", `tools: ["Read", "Bash"]`)},
		map[string]string{"widget": `{"name":"widget","role":"widget","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read","Bash(scripts/research/kb-search.sh:*)"]}`},
	)
	vs, err := Check(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(vs) != 0 {
		t.Errorf("violations = %+v, want none (Bash ↔ Bash(*) must normalize)", vs)
	}
}

func TestCoherence_SkillParenNormalization(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{"evolve-widget": personaMD("widget", `tools: ["Read", "Skill"]`)},
		map[string]string{"widget": `{"name":"widget","role":"widget","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read","Skill(code-review-simplify)","Skill(security-review-scored)"]}`},
	)
	vs, err := Check(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(vs) != 0 {
		t.Errorf("violations = %+v, want none (Skill ↔ Skill(*) must normalize)", vs)
	}
}

func TestCoherence_ToolsGeminiLineIsNotTheToolsLine(t *testing.T) {
	// Persona has only the multi-CLI variants and NO bare tools: line → no
	// constraint declared → skip (architecture non-goal: claude line only).
	agents, profs := fixtures(
		map[string]string{"evolve-widget": personaMD("widget",
			`tools-gemini: ["ReadFile", "RunShell"]`,
			`tools-generic: ["read_file", "run_shell"]`)},
		map[string]string{"widget": `{"name":"widget","role":"widget","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read"]}`},
	)
	vs, err := Check(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(vs) != 0 {
		t.Errorf("violations = %+v, want none (tools-gemini: must not parse as tools:)", vs)
	}
}

func TestCoherence_ProfileWithoutAllowedToolsSkipped(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{"evolve-widget": personaMD("widget", `tools: ["Read", "Write"]`)},
		map[string]string{"widget": `{"name":"widget","role":"widget","cli":"claude-tmux","model_tier_default":"sonnet"}`},
	)
	vs, err := Check(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(vs) != 0 {
		t.Errorf("violations = %+v, want none (no allowed_tools → no constraint, R11)", vs)
	}
}

func TestCoherence_UnpairedAndNonPersonaFilesSkipped(t *testing.T) {
	// Persona without profile, profile without persona, and the
	// certain-to-exist non-persona .md files in agents/ (AGENTS.md,
	// agent-templates.md — architecture risk table) → all silently skipped.
	agents, profs := fixtures(
		map[string]string{
			"evolve-orphan": personaMD("orphan", `tools: ["Read"]`),
		},
		map[string]string{"unpersonaed": `{"name":"unpersonaed","role":"x","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read"]}`},
	)
	agents["agents/agent-templates.md"] = &fstest.MapFile{
		Data: []byte("---\ntools: [\"Read\"]\n---\nshared template, not a persona\n")}
	agents["agents/AGENTS.md"] = &fstest.MapFile{
		Data: []byte("# index, no frontmatter\n")}

	vs, err := Check(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(vs) != 0 {
		t.Errorf("violations = %+v, want none (unpaired/non-persona skip)", vs)
	}
}

func TestCoherence_OverrideSubstitutesPersona(t *testing.T) {
	// R5: Overrides["widget"] = <os path> replaces agents/evolve-widget.md
	// content. The on-FS persona is clean; the override drifts → the drift
	// must be reported (proving the override content was used).
	agents, profs := fixtures(
		map[string]string{"evolve-widget": personaMD("widget", `tools: ["Read"]`)},
		map[string]string{"widget": `{"name":"widget","role":"widget","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read"]}`},
	)
	overridePath := filepath.Join(t.TempDir(), "widget-drifted.md")
	if err := os.WriteFile(overridePath, []byte(personaMD("widget", `tools: ["Read", "Write"]`)), 0o644); err != nil {
		t.Fatal(err)
	}

	vs, err := Check(Options{
		AgentsFS:   agents,
		ProfilesFS: profs,
		Overrides:  map[string]string{"widget": overridePath},
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(vs) != 1 {
		t.Fatalf("violations = %+v, want exactly 1 (override content must be used)", vs)
	}
	if vs[0].Kind != "disallowed" || !strings.Contains(vs[0].Message, "Write") {
		t.Errorf("violation = %+v, want disallowed Write from the override file", vs[0])
	}
}

func TestCoherence_MissingAgentsDirErrors(t *testing.T) {
	// Fail loudly: an FS with no agents/ directory is an operator error
	// (wrong root), not an empty result.
	_, profs := fixtures(nil, map[string]string{"widget": `{"name":"widget","role":"widget","cli":"claude-tmux","model_tier_default":"sonnet"}`})
	if _, err := Check(Options{AgentsFS: fstest.MapFS{}, ProfilesFS: profs}); err == nil {
		t.Error("Check(no agents/ dir) = nil error, want error (fail loudly)")
	}
}
