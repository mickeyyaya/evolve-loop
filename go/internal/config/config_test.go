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

func TestLoad_GateStagesUseDefaults(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "absent.json")

	cfg, ws := Load(absent, map[string]string{
		"EVOLVE_EVAL_GATE":       "off",
		"EVOLVE_CONTRACT_GATE":   "shadow",
		"EVOLVE_TRIAGE_CAP_GATE": "off",
	})
	if cfg.EvalGate != StageEnforce {
		t.Errorf("default EvalGate = %v, want StageEnforce", cfg.EvalGate)
	}
	if cfg.ContractGate != StageEnforce {
		t.Errorf("default ContractGate = %v, want StageEnforce", cfg.ContractGate)
	}
	if cfg.TriageCapGate != StageEnforce {
		t.Errorf("default TriageCapGate = %v, want StageEnforce", cfg.TriageCapGate)
	}
	if hasWarning(ws, "unknown-value") {
		t.Errorf("removed gate env keys should be ignored without warnings: %+v", ws)
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

func TestLoad_PhaseRecoveryStage(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "absent.json")

	// Default (no env): SHADOW — ADR-0044's behavior-neutral first ship. Note
	// the R8.5 spine-floor flip deliberately did NOT move this dial: it is
	// overloaded (bidirectional channel + failure-adviser promotion), so the
	// floor got its own SpineFloor dial instead (TestLoad_SpineFloorStage).
	if cfg, _ := Load(absent, map[string]string{}); cfg.PhaseRecovery != StageShadow {
		t.Errorf("default PhaseRecovery = %v, want StageShadow", cfg.PhaseRecovery)
	}
	// EVOLVE_PHASE_RECOVERY is retired (cycle-12 flag retirement). The env var
	// is ignored by applyEnv; the dial is now policy-driven (policy.RecoveryConfig).
	// Passing the env var has no effect — PhaseRecovery stays at the default StageShadow.
	for _, v := range []string{"off", "0", "shadow", "enforce", "banana"} {
		cfg, _ := Load(absent, map[string]string{"EVOLVE_PHASE_RECOVERY": v})
		if cfg.PhaseRecovery != StageShadow {
			t.Errorf("EVOLVE_PHASE_RECOVERY=%q should be ignored (retired flag); got %v, want StageShadow", v, cfg.PhaseRecovery)
		}
	}
}

// TestLoad_SpineFloorStage pins the R8.5 flip: the artifact-backed spine floor
// defaults ENFORCE on its OWN dial (decoupled from the overloaded PhaseRecovery
// — see the SpineFloor field doc), with no env-var override (policy-only:
// `recovery.spine_floor`).
func TestLoad_SpineFloorStage(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "absent.json")
	if cfg, _ := Load(absent, map[string]string{}); cfg.SpineFloor != StageEnforce {
		t.Errorf("default SpineFloor = %v, want StageEnforce (R8.5 flip)", cfg.SpineFloor)
	}
	// No env vocabulary exists for it — any EVOLVE_SPINE_FLOOR value is inert.
	for _, v := range []string{"off", "shadow", "banana"} {
		cfg, _ := Load(absent, map[string]string{"EVOLVE_SPINE_FLOOR": v})
		if cfg.SpineFloor != StageEnforce {
			t.Errorf("EVOLVE_SPINE_FLOOR=%q must be inert (policy-only dial); got %v, want StageEnforce", v, cfg.SpineFloor)
		}
	}
}

// TestPhaseIOStage pins the EVOLVE_PHASE_IO dial (ADR-0050 Phase 3): the unified
// phase I/O rollout uses the FULL off→shadow→advisory→enforce ladder (4-value,
// unlike the 3-value gate dials), defaults ENFORCE as of the 3.10 cutover (the
// typed envelope is now authoritative; set EVOLVE_PHASE_IO=off to roll back), and
// a typo falls back to off with a warning (never silently leaving the dial in an
// unintended state). Covers DefaultEnforce / Off / Shadow / Advisory / Enforce / TypoDefaultsOff.
func TestPhaseIOStage(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "absent.json")

	// DefaultEnforce: no env ⇒ the typed envelope is authoritative (3.10 cutover).
	if cfg, _ := Load(absent, map[string]string{}); cfg.PhaseIO != StageEnforce {
		t.Errorf("default PhaseIO = %v, want StageEnforce", cfg.PhaseIO)
	}
	// The full 4-value ladder (advisory is the middle state the gates omit).
	for v, want := range map[string]Stage{
		"off": StageOff, "0": StageOff, "shadow": StageShadow,
		"advisory": StageAdvisory, "enforce": StageEnforce,
	} {
		if cfg, _ := Load(absent, map[string]string{"EVOLVE_PHASE_IO": v}); cfg.PhaseIO != want {
			t.Errorf("EVOLVE_PHASE_IO=%q → %v, want %v", v, cfg.PhaseIO, want)
		}
	}
	// TypoDefaultsOff: an unknown value never silently enables the envelope.
	cfg, ws := Load(absent, map[string]string{"EVOLVE_PHASE_IO": "banana"})
	if cfg.PhaseIO != StageOff {
		t.Errorf("typo EVOLVE_PHASE_IO → %v, want StageOff", cfg.PhaseIO)
	}
	if !hasWarning(ws, "unknown-value") {
		t.Error("typo EVOLVE_PHASE_IO should warn unknown-value")
	}
}

// TestDefaults_PhaseIO_Enforce pins the 3.10 cutover at the struct-default level:
// the baked-in RolloutStages default is StageEnforce, so the typed envelope is
// authoritative on every production path that doesn't explicitly override the dial.
// This is the byte-behavior flip — guard it explicitly so a future default edit
// can't silently roll the cutover back to off.
func TestDefaults_PhaseIO_Enforce(t *testing.T) {
	if got := defaults().RolloutStages.PhaseIO; got != StageEnforce {
		t.Errorf("default RolloutStages.PhaseIO = %v, want StageEnforce (3.10 cutover)", got)
	}
}
