package phasecoherence

import (
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestArtifactCoherence_MatchedPairNoMismatch(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{"evolve-builder": personaMD("builder",
			`tools: ["Read"]`,
			`output-format: "build-report.md — Design Decision, Files Changed table, Test Results"`)},
		map[string]string{"builder": `{"name":"builder","role":"builder","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read"],"output_artifact":".evolve/runs/cycle-{cycle}/build-report.md"}`},
	)
	vs, err := CheckArtifactNames(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("CheckArtifactNames: %v", err)
	}
	if len(vs) != 0 {
		t.Errorf("violations = %+v, want none ({cycle} template dir part must strip)", vs)
	}
}

func TestArtifactCoherence_Mismatch(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{"evolve-plan-review": personaMD("plan-review",
			`output-format: "plan-review.md — ## Findings, ## Verdict"`)},
		map[string]string{"plan-review": `{"name":"plan-review","role":"plan-review","cli":"claude-tmux","model_tier_default":"sonnet","output_artifact":".evolve/runs/cycle-{cycle}/plan-review-report.md"}`},
	)
	vs, err := CheckArtifactNames(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("CheckArtifactNames: %v", err)
	}
	if len(vs) != 1 {
		t.Fatalf("violations = %+v, want exactly 1", vs)
	}
	v := vs[0]
	if v.Persona != "plan-review" {
		t.Errorf("Persona = %q, want %q", v.Persona, "plan-review")
	}
	if v.Severity != "WARN" {
		t.Errorf("Severity = %q, want %q", v.Severity, "WARN")
	}
	if !strings.Contains(v.Message, "plan-review.md") {
		t.Errorf("Message %q missing persona artifact plan-review.md", v.Message)
	}
	if !strings.Contains(v.Message, "plan-review-report.md") {
		t.Errorf("Message %q missing profile artifact plan-review-report.md", v.Message)
	}
	if !strings.Contains(v.Message, "mismatch") {
		t.Errorf("Message %q missing eval vocabulary (mismatch)", v.Message)
	}
}

func TestArtifactCoherence_FirstMdTokenWins(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{"evolve-scout": personaMD("scout",
			`output-format: "scout-report.md — Gap Analysis table, Handoff JSON (see agent-templates.md)"`)},
		map[string]string{"scout": `{"name":"scout","role":"scout","cli":"claude-tmux","model_tier_default":"sonnet","output_artifact":".evolve/runs/cycle-{cycle}/scout-report.md"}`},
	)
	vs, err := CheckArtifactNames(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("CheckArtifactNames: %v", err)
	}
	if len(vs) != 0 {
		t.Errorf("violations = %+v, want none (first .md token is the artifact)", vs)
	}
}

func TestArtifactCoherence_NoFrontmatter(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{"evolve-debugger": personaMD("debugger", `tools: ["Read", "Bash"]`)},
		map[string]string{"debugger": `{"name":"debugger","role":"debugger","cli":"claude-tmux","model_tier_default":"sonnet","output_artifact":".evolve/runs/cycle-{cycle}/debug-report.md"}`},
	)
	vs, err := CheckArtifactNames(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("CheckArtifactNames: %v", err)
	}
	if len(vs) != 0 {
		t.Errorf("violations = %+v, want none (no output-format: → skip)", vs)
	}
}

func TestArtifactCoherence_ProfileMissingField(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{"evolve-observer": personaMD("observer",
			`output-format: "observer-report.md — ## Findings"`)},
		map[string]string{"observer": `{"name":"observer","role":"observer","cli":"claude-tmux","model_tier_default":"sonnet"}`},
	)
	vs, err := CheckArtifactNames(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("CheckArtifactNames: %v", err)
	}
	if len(vs) != 1 {
		t.Fatalf("violations = %+v, want exactly 1 (declared artifact with no profile contract)", vs)
	}
	v := vs[0]
	if v.Persona != "observer" || v.Severity != "WARN" {
		t.Errorf("violation = %+v, want {Persona:observer Severity:WARN}", v)
	}
	if !strings.Contains(v.Message, "observer-report.md") || !strings.Contains(v.Message, "output_artifact") {
		t.Errorf("Message %q must name the declared artifact and the missing output_artifact field", v.Message)
	}
}

func TestArtifactCoherence_MissingAgentsDirErrors(t *testing.T) {
	_, profs := fixtures(nil, map[string]string{"widget": `{"name":"widget","role":"widget","cli":"claude-tmux","model_tier_default":"sonnet"}`})
	if _, err := CheckArtifactNames(Options{AgentsFS: fstest.MapFS{}, ProfilesFS: profs}); err == nil {
		t.Error("CheckArtifactNames(no agents/ dir) = nil error, want error (fail loudly)")
	}
}

func TestArtifactCoherence_MissingProfilesFSErrors(t *testing.T) {
	agents, _ := fixtures(
		map[string]string{"evolve-widget": personaMD("widget", `output-format: "widget-report.md"`)},
		nil,
	)
	if _, err := CheckArtifactNames(Options{AgentsFS: agents}); err == nil {
		t.Error("CheckArtifactNames(missing ProfilesFS) = nil error, want error")
	}
}

func TestArtifactCoherence_OverrideReadErrorReturnsError(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{"evolve-widget": personaMD("widget", `output-format: "widget-report.md"`)},
		map[string]string{"widget": `{"name":"widget","role":"widget","cli":"claude-tmux","model_tier_default":"sonnet","output_artifact":"widget-report.md"}`},
	)
	_, err := CheckArtifactNames(Options{
		AgentsFS:   agents,
		ProfilesFS: profs,
		Overrides:  map[string]string{"widget": filepath.Join(t.TempDir(), "missing.md")},
	})
	if err == nil {
		t.Fatal("CheckArtifactNames(missing override) = nil error, want error")
	}
}

func TestArtifactCoherence_SkipsNonMdProfileArtifact(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{"evolve-memo": personaMD("memo",
			`output-format: "carryover-todos.json (primary) plus memo.md observations"`)},
		map[string]string{"memo": `{"name":"memo","role":"memo","cli":"claude-tmux","model_tier_default":"sonnet","output_artifact":".evolve/runs/cycle-{cycle}/carryover-todos.json"}`},
	)
	vs, err := CheckArtifactNames(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("CheckArtifactNames: %v", err)
	}
	if len(vs) != 0 {
		t.Errorf("violations = %+v, want none (non-.md profile artifact must skip the .md comparison)", vs)
	}
}

func TestArtifactCoherence_DirQualifiedPersonaTokenMatchesBasename(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{"evolve-reflector": personaMD("reflector",
			`output-format: "learn/reflector-synthesis.md — ## Synthesis"`)},
		map[string]string{"reflector": `{"name":"reflector","role":"reflector","cli":"claude-tmux","model_tier_default":"sonnet","output_artifact":".evolve/runs/cycle-{cycle}/learn/reflector-synthesis.md"}`},
	)
	vs, err := CheckArtifactNames(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("CheckArtifactNames: %v", err)
	}
	if len(vs) != 0 {
		t.Errorf("violations = %+v, want none (dir-qualified persona token must match by basename)", vs)
	}
}
