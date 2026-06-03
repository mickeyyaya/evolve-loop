package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeRegistry writes a phase-registry.json into a temp dir and returns its path.
func writeRegistry(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "phase-registry.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	return p
}

func hasWarning(ws []Warning, code string) bool {
	for _, w := range ws {
		if w.Code == code {
			return true
		}
	}
	return false
}

func TestLoad_DefaultsWhenRegistryMissing(t *testing.T) {
	cfg, ws := Load(filepath.Join(t.TempDir(), "absent.json"), map[string]string{})

	if cfg.Stage != StageAdvisory {
		t.Errorf("Stage = %v, want StageAdvisory (default-on since Component #7)", cfg.Stage)
	}
	if cfg.Mode != ModeDynamicLLM {
		t.Errorf("Mode = %v, want ModeDynamicLLM (locked decision: LLM default)", cfg.Mode)
	}
	if cfg.MaxInsertions != 4 {
		t.Errorf("MaxInsertions = %d, want 4", cfg.MaxInsertions)
	}
	wantMandatory := []string{"scout", "build", "audit", "ship"}
	if got := cfg.Mandatory; len(got) != 4 || got[0] != "scout" || got[3] != "ship" {
		t.Errorf("Mandatory = %v, want %v", got, wantMandatory)
	}
	rule, ok := cfg.Conditional["tdd"]
	if !ok {
		t.Fatalf("Conditional missing tdd pin")
	}
	if rule.Field != "cycle_size" || rule.Op != "!=" || rule.Value != "trivial" {
		t.Errorf("tdd CondRule = %+v, want {cycle_size != trivial}", rule)
	}
	if hasWarning(ws, "weak-spine") {
		t.Errorf("unexpected weak-spine warning on default spine")
	}
}

func TestLoad_RegistryConfigBlockParsed(t *testing.T) {
	reg := writeRegistry(t, `{
      "schema_version": 3,
      "config": {
        "dynamic_routing": "shadow",
        "routing_mode": "static",
        "mandatory_phases": ["scout","build","audit","ship"],
        "conditional_mandatory": {"tdd": "cycle_size!=trivial"},
        "max_optional_insertions": 7
      },
      "phases": []
    }`)

	cfg, ws := Load(reg, map[string]string{})

	if cfg.Stage != StageShadow {
		t.Errorf("Stage = %v, want StageShadow", cfg.Stage)
	}
	if cfg.Mode != ModeStaticPreset {
		t.Errorf("Mode = %v, want ModeStaticPreset", cfg.Mode)
	}
	if cfg.MaxInsertions != 7 {
		t.Errorf("MaxInsertions = %d, want 7", cfg.MaxInsertions)
	}
	if len(ws) != 0 {
		t.Errorf("unexpected warnings: %+v", ws)
	}
}

func TestLoad_EnvOverridesRegistryAndDefault(t *testing.T) {
	reg := writeRegistry(t, `{"schema_version":3,"config":{"dynamic_routing":"shadow"},"phases":[]}`)

	cfg, ws := Load(reg, map[string]string{
		"EVOLVE_DYNAMIC_ROUTING":         "enforce",
		"EVOLVE_ROUTING_MODE":            "static",
		"EVOLVE_MAX_OPTIONAL_INSERTIONS": "2",
		"EVOLVE_MANDATORY_PHASES":        "scout,build", // drops audit+ship
	})

	if cfg.Stage != StageEnforce {
		t.Errorf("Stage = %v, want StageEnforce (env beats registry)", cfg.Stage)
	}
	if cfg.Mode != ModeStaticPreset {
		t.Errorf("Mode = %v, want ModeStaticPreset", cfg.Mode)
	}
	if cfg.MaxInsertions != 2 {
		t.Errorf("MaxInsertions = %d, want 2", cfg.MaxInsertions)
	}
	if len(cfg.Mandatory) != 2 {
		t.Errorf("Mandatory = %v, want [scout build]", cfg.Mandatory)
	}
	if !hasWarning(ws, "weak-spine") {
		t.Errorf("expected weak-spine warning when audit/ship dropped from mandatory")
	}
}

func TestLoad_EvalGateStage(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "absent.json")

	// Default (no env): enforce — the structural eval gates are on by default.
	if cfg, _ := Load(absent, map[string]string{}); cfg.EvalGate != StageEnforce {
		t.Errorf("default EvalGate = %v, want StageEnforce", cfg.EvalGate)
	}

	for v, want := range map[string]Stage{"off": StageOff, "0": StageOff, "shadow": StageShadow, "enforce": StageEnforce} {
		if cfg, _ := Load(absent, map[string]string{"EVOLVE_EVAL_GATE": v}); cfg.EvalGate != want {
			t.Errorf("EVOLVE_EVAL_GATE=%q → %v, want %v", v, cfg.EvalGate, want)
		}
	}

	// A typo defaults to off (never silently keeps a kill-path) and warns.
	cfg, ws := Load(absent, map[string]string{"EVOLVE_EVAL_GATE": "banana"})
	if cfg.EvalGate != StageOff {
		t.Errorf("typo EVOLVE_EVAL_GATE → %v, want StageOff", cfg.EvalGate)
	}
	if !hasWarning(ws, "unknown-value") {
		t.Error("typo EVOLVE_EVAL_GATE should warn unknown-value")
	}
}

func TestLoad_ContractGateStage(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "absent.json")

	// Default (no env): enforce — the deliverable-contract gate is on by default (ADR-0034).
	if cfg, _ := Load(absent, map[string]string{}); cfg.ContractGate != StageEnforce {
		t.Errorf("default ContractGate = %v, want StageEnforce", cfg.ContractGate)
	}
	for v, want := range map[string]Stage{"off": StageOff, "0": StageOff, "shadow": StageShadow, "enforce": StageEnforce} {
		if cfg, _ := Load(absent, map[string]string{"EVOLVE_CONTRACT_GATE": v}); cfg.ContractGate != want {
			t.Errorf("EVOLVE_CONTRACT_GATE=%q → %v, want %v", v, cfg.ContractGate, want)
		}
	}
	// A typo defaults to off (never silently keeps a kill-path) and warns.
	cfg, ws := Load(absent, map[string]string{"EVOLVE_CONTRACT_GATE": "banana"})
	if cfg.ContractGate != StageOff {
		t.Errorf("typo EVOLVE_CONTRACT_GATE → %v, want StageOff", cfg.ContractGate)
	}
	if !hasWarning(ws, "unknown-value") {
		t.Error("typo EVOLVE_CONTRACT_GATE should warn unknown-value")
	}
}

func TestLoad_UnknownStageDefaultsSafe(t *testing.T) {
	cfg, ws := Load(filepath.Join(t.TempDir(), "absent.json"), map[string]string{
		"EVOLVE_DYNAMIC_ROUTING": "banana",
	})
	if cfg.Stage != StageOff {
		t.Errorf("Stage = %v, want StageOff for unknown value (protect autonomy)", cfg.Stage)
	}
	if !hasWarning(ws, "unknown-value") {
		t.Errorf("expected unknown-value warning for bogus stage")
	}
}

func TestLoad_UseRegistryDisabled(t *testing.T) {
	reg := writeRegistry(t, `{"schema_version":3,"config":{"max_optional_insertions":99},"phases":[]}`)
	cfg, _ := Load(reg, map[string]string{"EVOLVE_USE_PHASE_REGISTRY": "0"})
	if cfg.MaxInsertions != 4 {
		t.Errorf("MaxInsertions = %d, want default 4 when registry disabled", cfg.MaxInsertions)
	}
}

func TestLoad_PerPhaseEnabledAndTriggers(t *testing.T) {
	reg := writeRegistry(t, `{
      "schema_version": 3,
      "config": {},
      "phases": [
        { "name": "tester", "optional": true, "enabled": "content",
          "routing": { "insert_when": [ {"field":"build.acs_red","op":"gt","value":0} ] } },
        { "name": "triage", "optional": true, "enabled": "on" }
      ]
    }`)

	cfg, _ := Load(reg, map[string]string{})

	if cfg.PhaseEnable["tester"] != EnableContent {
		t.Errorf("tester enable = %v, want EnableContent", cfg.PhaseEnable["tester"])
	}
	if cfg.PhaseEnable["triage"] != EnableOn {
		t.Errorf("triage enable = %v, want EnableOn", cfg.PhaseEnable["triage"])
	}
	tb, ok := cfg.Triggers["tester"]
	if !ok || len(tb.InsertWhen) != 1 {
		t.Fatalf("tester triggers = %+v, want one insert_when", tb)
	}
	c := tb.InsertWhen[0]
	if c.Field != "build.acs_red" || c.Op != "gt" {
		t.Errorf("tester condition = %+v, want build.acs_red gt 0", c)
	}
}

func TestLoad_LegacyFlagAbsorption(t *testing.T) {
	cfg, _ := Load(filepath.Join(t.TempDir(), "absent.json"), map[string]string{
		"EVOLVE_TRIAGE_DISABLE": "1",
		"EVOLVE_REQUIRE_INTENT": "1",
	})
	if cfg.PhaseEnable["triage"] != EnableOff {
		t.Errorf("triage enable = %v, want EnableOff (EVOLVE_TRIAGE_DISABLE=1)", cfg.PhaseEnable["triage"])
	}
	if cfg.PhaseEnable["intent"] != EnableOn {
		t.Errorf("intent enable = %v, want EnableOn (EVOLVE_REQUIRE_INTENT=1)", cfg.PhaseEnable["intent"])
	}
}

func TestParseCondRule(t *testing.T) {
	cases := []struct {
		in              string
		field, op, want string
	}{
		{"cycle_size!=trivial", "cycle_size", "!=", "trivial"},
		{"cycle_size==large", "cycle_size", "==", "large"},
		{"verdict != PASS", "verdict", "!=", "PASS"}, // tolerate spaces
	}
	for _, tc := range cases {
		got, err := parseCondRule(tc.in)
		if err != nil {
			t.Fatalf("parseCondRule(%q) error: %v", tc.in, err)
		}
		if got.Field != tc.field || got.Op != tc.op || got.Value != tc.want {
			t.Errorf("parseCondRule(%q) = %+v, want {%s %s %s}", tc.in, got, tc.field, tc.op, tc.want)
		}
	}
}

func TestStageAndModeString(t *testing.T) {
	if StageEnforce.String() != "enforce" {
		t.Errorf("StageEnforce.String() = %q", StageEnforce.String())
	}
	if ModeDynamicLLM.String() != "llm" {
		t.Errorf("ModeDynamicLLM.String() = %q", ModeDynamicLLM.String())
	}
}
