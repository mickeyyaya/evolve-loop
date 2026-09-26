package phasecoherence

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func personaMD(name string, fmLines ...string) string {
	b := "---\nname: evolve-" + name + "\ndescription: test fixture\n"
	for _, l := range fmLines {
		b += l + "\n"
	}
	return b + "---\n\n# " + name + "\n"
}

// fixtures keys personas by file stem (evolve-<name>), placed under agents/, and profiles by name.
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
	if !strings.Contains(v.Message, "Write") {
		t.Errorf("Message %q does not name the drifting tool Write", v.Message)
	}
	if !strings.Contains(v.Message, "disallowed") && !strings.Contains(v.Message, "contradiction") {
		t.Errorf("Message %q missing eval vocabulary (disallowed|contradiction)", v.Message)
	}
}

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
	// Despite the name, the unpaired persona WARNs; only a profile without a persona and
	// the non-persona .md files stay silent.
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
	for _, v := range vs {
		if v.Kind != "unpaired" {
			t.Errorf("unexpected non-unpaired violation %+v (non-persona files must stay skipped)", v)
		}
		if v.Persona != "orphan" {
			t.Errorf("violation for %q — only the unpaired persona may warn", v.Persona)
		}
	}
	if len(vs) != 1 {
		t.Errorf("violations = %+v, want exactly the orphan unpaired WARN", vs)
	}
}

func TestCoherence_OverrideSubstitutesPersona(t *testing.T) {
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
	_, profs := fixtures(nil, map[string]string{"widget": `{"name":"widget","role":"widget","cli":"claude-tmux","model_tier_default":"sonnet"}`})
	if _, err := Check(Options{AgentsFS: fstest.MapFS{}, ProfilesFS: profs}); err == nil {
		t.Error("Check(no agents/ dir) = nil error, want error (fail loudly)")
	}
}

func TestCoherence_MissingProfilesFSErrors(t *testing.T) {
	agents, _ := fixtures(
		map[string]string{"evolve-widget": personaMD("widget", `tools: ["Read"]`)},
		nil,
	)
	if _, err := Check(Options{AgentsFS: agents}); err == nil {
		t.Error("Check(missing ProfilesFS) = nil error, want error")
	}
}

func TestCoherence_NilAgentsFSErrors(t *testing.T) {
	_, profs := fixtures(nil, map[string]string{"widget": `{"name":"widget","role":"widget","cli":"claude-tmux","model_tier_default":"sonnet"}`})
	if _, err := Check(Options{ProfilesFS: profs}); err == nil {
		t.Error("Check(missing AgentsFS) = nil error, want error")
	}
}

func TestCoherence_DirectoryEntrySkipped(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{"evolve-widget": personaMD("widget", `tools: ["Read"]`)},
		map[string]string{"widget": `{"name":"widget","role":"widget","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read"]}`},
	)
	agents["agents/evolve-directory"] = &fstest.MapFile{Mode: fs.ModeDir}

	vs, err := Check(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(vs) != 0 {
		t.Errorf("directory entries must be skipped; got violations %+v", vs)
	}
}

func TestCoherence_NilFrontmatterSkipped(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{"evolve-widget": "# widget\n\nno frontmatter\n"},
		map[string]string{"widget": `{"name":"widget","role":"widget","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read"]}`},
	)

	vs, err := Check(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(vs) != 0 {
		t.Errorf("nil frontmatter must be skipped; got violations %+v", vs)
	}
}

func TestCoherence_NonSliceToolsValSkipped(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{"evolve-widget": personaMD("widget", `tools: Read`)},
		map[string]string{"widget": `{"name":"widget","role":"widget","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read"]}`},
	)

	vs, err := Check(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(vs) != 0 {
		t.Errorf("non-slice tools value must be skipped; got violations %+v", vs)
	}
}

func TestCoherence_SkippedEntriesDoNotMaskValidViolations(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{
			"evolve-clean":        personaMD("clean", `tools: ["Read"]`),
			"evolve-no-front":     "# no frontmatter\n",
			"evolve-nonslice":     personaMD("nonslice", `tools: Read`),
			"evolve-widget-drift": personaMD("widget-drift", `tools: ["Read", "Write"]`),
		},
		map[string]string{
			"clean":        `{"name":"clean","role":"clean","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read"]}`,
			"no-front":     `{"name":"no-front","role":"no-front","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read"]}`,
			"nonslice":     `{"name":"nonslice","role":"nonslice","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read"]}`,
			"widget-drift": `{"name":"widget-drift","role":"widget-drift","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read"]}`,
		},
	)
	agents["agents/evolve-subdir"] = &fstest.MapFile{Mode: fs.ModeDir}

	vs, err := Check(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(vs) != 1 {
		t.Fatalf("violations = %+v, want exactly one real drift after skipped entries", vs)
	}
	if vs[0].Persona != "widget-drift" || vs[0].Kind != "disallowed" {
		t.Fatalf("violation = %+v, want widget-drift disallowed", vs[0])
	}
}

func TestCoherence_DispatchNonePersonaExempt(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{
			"evolve-operator": personaMD("operator", `tools: ["Read"]`, `dispatch: none`),
			"evolve-orphan":   personaMD("orphan", `tools: ["Read"]`),
		},
		map[string]string{},
	)
	vs, err := Check(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	for _, v := range vs {
		if v.Persona == "operator" {
			t.Errorf("dispatch:none persona must be exempt from the unpaired WARN; got %+v", v)
		}
	}
	found := false
	for _, v := range vs {
		if v.Persona == "orphan" && v.Kind == "unpaired" {
			found = true
		}
	}
	if !found {
		t.Errorf("unmarked unpaired persona must still WARN; got %+v", vs)
	}
}
