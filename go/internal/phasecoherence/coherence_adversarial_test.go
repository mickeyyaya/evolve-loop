package phasecoherence

// coherence_adversarial_test.go — cycle-281 test amplification.
// Targets the uncovered branches in Check (76.1%) and CheckArtifactNames
// (67.3%) identified by the cycle-281 coverage baseline (88.6% total).
// All tests are black-box (spec-derived), never reading the implementation.

import (
	"strings"
	"testing"
	"testing/fstest"
)

// TestCoherence_MalformedProfileJSON — adversarial: a JSON-malformed profile
// must return an error, not silently produce zero violations (corpus rot guard).
func TestCoherence_MalformedProfileJSON(t *testing.T) {
	agents, _ := fixtures(
		map[string]string{"evolve-widget": personaMD("widget", `tools: ["Read"]`)},
		nil,
	)
	profs := fstest.MapFS{
		"widget.json": &fstest.MapFile{Data: []byte("{not valid json")},
	}
	_, err := Check(Options{AgentsFS: agents, ProfilesFS: profs})
	if err == nil {
		t.Error("Check(malformed JSON profile) = nil error, want error (fail loudly on corpus rot)")
	}
}

// TestCoherence_MultiplePersonasMixedViolations — adversarial: two personas,
// one clean and one drifting; only the drifting persona must appear in violations.
func TestCoherence_MultiplePersonasMixedViolations(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{
			"evolve-clean":   personaMD("clean", `tools: ["Read", "Bash"]`),
			"evolve-drifted": personaMD("drifted", `tools: ["Read", "Write", "Edit"]`),
		},
		map[string]string{
			"clean":   `{"name":"clean","role":"clean","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read","Bash"]}`,
			"drifted": `{"name":"drifted","role":"drifted","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read"]}`,
		},
	)
	vs, err := Check(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	for _, v := range vs {
		if v.Persona == "clean" {
			t.Errorf("clean pair must not produce violations; got %+v", v)
		}
	}
	var driftedCount int
	for _, v := range vs {
		if v.Persona == "drifted" {
			driftedCount++
		}
	}
	if driftedCount == 0 {
		t.Error("drifted pair must produce at least one violation")
	}
}

// TestCoherence_BothDisallowedAndUndeclared — adversarial: persona declares a
// tool the profile disallows AND the profile allows a tool the persona omits;
// both violation kinds must appear.
func TestCoherence_BothDisallowedAndUndeclared(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{"evolve-widget": personaMD("widget", `tools: ["Read", "Write"]`)},
		map[string]string{"widget": `{"name":"widget","role":"widget","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":["Read","WebSearch"]}`},
	)
	vs, err := Check(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	var hasDisallowed, hasUndeclared bool
	for _, v := range vs {
		if v.Kind == "disallowed" {
			hasDisallowed = true
		}
		if v.Kind == "undeclared" {
			hasUndeclared = true
		}
	}
	if !hasDisallowed {
		t.Errorf("want disallowed violation for Write; vs=%+v", vs)
	}
	if !hasUndeclared {
		t.Errorf("want undeclared violation for WebSearch; vs=%+v", vs)
	}
}

// TestCoherence_EmptyAllowedToolsList — adversarial: profile has an empty
// allowed_tools array (not absent) → treated as no constraint → skip.
func TestCoherence_EmptyAllowedToolsList(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{"evolve-widget": personaMD("widget", `tools: ["Read", "Write"]`)},
		map[string]string{"widget": `{"name":"widget","role":"widget","cli":"claude-tmux","model_tier_default":"sonnet","allowed_tools":[]}`},
	)
	vs, err := Check(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	// An empty allowed_tools list means no constraint — same semantics as absent.
	if len(vs) != 0 {
		t.Errorf("empty allowed_tools must produce zero violations (no constraint); got %+v", vs)
	}
}

// TestArtifactCoherence_MalformedProfileJSON — adversarial: malformed JSON in
// a profile must propagate an error, not silently produce zero violations.
func TestArtifactCoherence_MalformedProfileJSON(t *testing.T) {
	agents, _ := fixtures(
		map[string]string{"evolve-builder": personaMD("builder",
			`output-format: "build-report.md"`)},
		nil,
	)
	profs := fstest.MapFS{
		"builder.json": &fstest.MapFile{Data: []byte("{{invalid json")},
	}
	_, err := CheckArtifactNames(Options{AgentsFS: agents, ProfilesFS: profs})
	if err == nil {
		t.Error("CheckArtifactNames(malformed JSON) = nil error, want error (fail loudly)")
	}
}

// TestArtifactCoherence_OutputFormatWithNoMdToken — adversarial: output-format
// line present but contains no .md token → the checker has nothing to compare,
// must produce zero violations (not a WARN).
func TestArtifactCoherence_OutputFormatWithNoMdToken(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{"evolve-probe": personaMD("probe",
			`output-format: "JSON payload — structured output only"`)},
		map[string]string{"probe": `{"name":"probe","role":"probe","cli":"claude-tmux","model_tier_default":"sonnet","output_artifact":".evolve/runs/cycle-{cycle}/probe.json"}`},
	)
	vs, err := CheckArtifactNames(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("CheckArtifactNames: %v", err)
	}
	if len(vs) != 0 {
		t.Errorf("no .md token in output-format → no artifact name to compare, want zero violations; got %+v", vs)
	}
}

// TestArtifactCoherence_MultiplePersonasOnlyOneMismatch — adversarial: five
// personas, only one mismatched → exactly one violation, naming both basenames.
func TestArtifactCoherence_MultiplePersonasOnlyOneMismatch(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{
			"evolve-scout":  personaMD("scout", `output-format: "scout-report.md"`),
			"evolve-triage": personaMD("triage", `output-format: "triage-report.md"`),
			"evolve-build":  personaMD("build", `output-format: "wrong-artifact.md"`),
			"evolve-audit":  personaMD("audit", `output-format: "audit-report.md"`),
			"evolve-ship":   personaMD("ship"),
		},
		map[string]string{
			"scout":  `{"name":"scout","role":"scout","cli":"claude-tmux","model_tier_default":"sonnet","output_artifact":".evolve/runs/cycle-{cycle}/scout-report.md"}`,
			"triage": `{"name":"triage","role":"triage","cli":"claude-tmux","model_tier_default":"sonnet","output_artifact":".evolve/runs/cycle-{cycle}/triage-report.md"}`,
			"build":  `{"name":"build","role":"build","cli":"claude-tmux","model_tier_default":"sonnet","output_artifact":".evolve/runs/cycle-{cycle}/build-report.md"}`,
			"audit":  `{"name":"audit","role":"audit","cli":"claude-tmux","model_tier_default":"sonnet","output_artifact":".evolve/runs/cycle-{cycle}/audit-report.md"}`,
			"ship":   `{"name":"ship","role":"ship","cli":"claude-tmux","model_tier_default":"sonnet","output_artifact":".evolve/runs/cycle-{cycle}/ship-report.md"}`,
		},
	)
	vs, err := CheckArtifactNames(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("CheckArtifactNames: %v", err)
	}
	if len(vs) != 1 {
		t.Fatalf("want exactly 1 violation (build mismatch), got %d: %+v", len(vs), vs)
	}
	v := vs[0]
	if v.Persona != "build" {
		t.Errorf("violation persona = %q, want %q", v.Persona, "build")
	}
	if !strings.Contains(v.Message, "wrong-artifact.md") {
		t.Errorf("Message must name the declared wrong artifact; got %q", v.Message)
	}
}

// TestArtifactCoherence_OutputFormatQuotedMdName — adversarial: output-format
// value where the .md filename is quoted with double-quotes → parser must still
// extract the first .md token.
func TestArtifactCoherence_OutputFormatQuotedMdName(t *testing.T) {
	agents, profs := fixtures(
		map[string]string{"evolve-scout": personaMD("scout",
			`output-format: "scout-report.md — Gap Analysis table, per the template"`)},
		map[string]string{"scout": `{"name":"scout","role":"scout","cli":"claude-tmux","model_tier_default":"sonnet","output_artifact":".evolve/runs/cycle-{cycle}/scout-report.md"}`},
	)
	vs, err := CheckArtifactNames(Options{AgentsFS: agents, ProfilesFS: profs})
	if err != nil {
		t.Fatalf("CheckArtifactNames: %v", err)
	}
	if len(vs) != 0 {
		t.Errorf("quoted .md name must still parse correctly and produce zero violations; got %+v", vs)
	}
}
