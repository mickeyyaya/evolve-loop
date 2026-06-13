// artifact_coherence_test.go — cycle-238 task
// `persona-output-artifact-coherence` (RED first). API contract for Builder
// (architecture blueprint B11, reuses Options/Violation from coherence.go):
//
//	func CheckArtifactNames(opts Options) ([]Violation, error)
//
// Semantics pinned by these tests (architecture R7 + eval
// persona-output-artifact-coherence C2/C3/C4):
//   - persona side: FIRST whitespace/quote-delimited token ending in .md on
//     the `output-format:` frontmatter line; no output-format: line → skip
//     (eval C3).
//   - profile side: path.Base(output_artifact) with {cycle}-style template
//     segments stripped (they live in the dir part).
//   - basename mismatch ⇒ Severity WARN Violation whose Message carries BOTH
//     names (eval vocabulary "mismatch").
//   - persona declares output-format but the paired profile has NO
//     output_artifact ⇒ WARN (eval C4 pins "flagged as WARN"; this
//     deliberately overrides blueprint B11's "skip" — the eval is the audit
//     authority, and a persona promising an artifact nobody contracts for is
//     exactly the I-3(d) incoherence class).
//
// Forward-protection for the I-3(d) incident: persona said plan-review.md,
// profile said plan-review-report.md → batch-fatal exit=81 ×2.
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

// TestArtifactCoherence_Mismatch — eval C2 (name pinned): exact replica of
// the I-3(d) incident shape.
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
	// The WARN must carry BOTH names — that is the whole diagnostic value
	// (and "mismatch" is eval-C4-class vocabulary).
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
	// The output-format prose may mention other .md names later; only the
	// FIRST .md token is the declared artifact (architecture risk table).
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

// TestArtifactCoherence_NoFrontmatter — eval C3 (name pinned): personas
// without an output-format: line (non-output phases) are silently skipped.
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

// TestArtifactCoherence_ProfileMissingField — eval C4 (name pinned): persona
// declares output-format but the profile has no output_artifact → WARN.
// (Eval C4 wins over blueprint B11's "skip" — see package comment.)
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

// TestArtifactCoherence_SkipsNonMdProfileArtifact — a phase whose contract
// deliverable is JSON (memo → carryover-todos.json) may legitimately mention
// a SECONDARY .md artifact in its output-format prose (memo.md). Comparing
// that .md token against a .json deliverable guarantees a false mismatch;
// the check must skip when the profile artifact is not .md.
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

// TestArtifactCoherence_DirQualifiedPersonaTokenMatchesBasename — the
// reflector persona declares "learn/reflector-synthesis.md" while the
// profile's output_artifact ends .../learn/reflector-synthesis.md: same
// file, but the persona token keeps the dir prefix and the profile side is
// path.Base'd. Both sides must be compared by basename.
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
