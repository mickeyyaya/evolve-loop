package config

// loader_pins_test.go — ADR-0103 unit 08 step 1: the pre-move pins and the
// goldens captured on 8e8f080f before any code moved. Every test here was
// green on the unsplit config.go and proven red against its named mutant;
// they replay byte-for-byte through the Loader after the split.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// kitchenSinkEnv is the env half of the kitchen-sink fixture (the registry half
// is testdata/kitchen_sink_registry.json): every env arm that can warn does,
// EVOLVE_USE_PHASE_REGISTRY=false keeps the registry ON (Q1), and the spine is
// weakened so validateSpine fires.
func kitchenSinkEnv() map[string]string {
	return map[string]string{
		"EVOLVE_SANDBOX":                 "bogus",
		"EVOLVE_MAX_OPTIONAL_INSERTIONS": "x",
		"EVOLVE_COMMIT_EVIDENCE":         "advisory",
		"EVOLVE_MANDATORY_PHASES":        "scout",
		"EVOLVE_DYNAMIC_ROUTING":         "off",
		"EVOLVE_USE_PHASE_REGISTRY":      "false",
	}
}

func kitchenSinkRegistry() string { return filepath.Join("testdata", "kitchen_sink_registry.json") }

func readGolden(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func marshalIndented(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(b, '\n')
}

func codesOf(ws []Warning) []string {
	out := make([]string, 0, len(ws))
	for _, w := range ws {
		out = append(out, w.Code)
	}
	return out
}

// Test 1 — the pipeline order registry → env → spine → inert is what the range
// stamper depends on; the literal messages pin each producer.
func TestLoad_WarningOrderIsRegistryThenEnvThenSpineThenInert(t *testing.T) {
	reg := writeRegistry(t, `{"config":{"dynamic_routing":"bogus"},"phases":[{"name":"plan-review","enabled":"on"}]}`)
	_, ws := Load(reg, map[string]string{
		"EVOLVE_PHASE_IO":         "bogus",
		"EVOLVE_DYNAMIC_ROUTING":  "off",
		"EVOLVE_MANDATORY_PHASES": "scout,build",
	})
	want := []string{
		`dynamic_routing="bogus" unknown, defaulting to off`,
		`EVOLVE_PHASE_IO="bogus" unknown, defaulting to off`,
		`mandatory_phases omits audit+ship — audit-before-ship guarantee weakened`,
		`phase "plan-review" is force-enabled but the router is off/shadow (dynamic_routing<advisory) and it is not in the static state machine — the enable is inert; set dynamic_routing>=advisory or remove the enable`,
	}
	wantCodes := []string{"unknown-value", "unknown-value", "weak-spine", "inert-phase-enable"}
	if got := codesOf(ws); !reflect.DeepEqual(got, wantCodes) {
		t.Fatalf("codes %v, want %v", got, wantCodes)
	}
	for i, w := range ws {
		if w.Message != want[i] {
			t.Errorf("warning %d message %q, want %q", i, w.Message, want[i])
		}
	}
}

// Test 2 — the inert-enable validator's first POSITIVE pin: force-enabled
// non-spine phases warn in sorted order below advisory, never at advisory.
func TestValidateInertEnables_FiresForForceEnabledNonSpinePhasesInSortedOrder(t *testing.T) {
	reg := writeRegistry(t, `{"config":{"mandatory_phases":["scout","build","audit","ship","custom-mandatory"]},
		"phases":[{"name":"zz-custom","enabled":"on"},{"name":"plan-review","enabled":"on"},{"name":"triage","enabled":"on"},{"name":"custom-mandatory","enabled":"on"}]}`)
	_, ws := Load(reg, map[string]string{"EVOLVE_DYNAMIC_ROUTING": "shadow"})
	var got []string
	for _, w := range ws {
		if w.Code == "inert-phase-enable" {
			got = append(got, w.Message)
		}
	}
	want := []string{
		`phase "plan-review" is force-enabled but the router is off/shadow (dynamic_routing<advisory) and it is not in the static state machine — the enable is inert; set dynamic_routing>=advisory or remove the enable`,
		`phase "zz-custom" is force-enabled but the router is off/shadow (dynamic_routing<advisory) and it is not in the static state machine — the enable is inert; set dynamic_routing>=advisory or remove the enable`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("inert warnings %q, want %q (sorted; the spine phase, the mandatory phase and the Off phases silent)", got, want)
	}
	if _, ws := Load(reg, map[string]string{"EVOLVE_DYNAMIC_ROUTING": "advisory"}); hasWarning(ws, "inert-phase-enable") {
		t.Errorf("at advisory the router consults PhaseEnable — nothing is inert: %+v", ws)
	}
}

// Test 3 — a malformed EVOLVE_CONDITIONAL_MANDATORY expression warns and keeps
// the compiled rule (covers the :766 branch).
func TestApplyEnv_ConditionalMandatoryParseErrorWarns(t *testing.T) {
	cfg, ws := Load("/nonexistent/phase-registry.json", map[string]string{"EVOLVE_CONDITIONAL_MANDATORY": "tdd:nonsense"})
	if got := codesOf(ws); !reflect.DeepEqual(got, []string{"unknown-value"}) {
		t.Fatalf("codes %v, want exactly one unknown-value", got)
	}
	if want := `EVOLVE_CONDITIONAL_MANDATORY="tdd:nonsense": no comparison operator in "nonsense"`; ws[0].Message != want {
		t.Errorf("message %q, want %q", ws[0].Message, want)
	}
	if !reflect.DeepEqual(cfg.Conditional["tdd"], DefaultTddRule()) {
		t.Errorf("Conditional[tdd] = %+v, want the compiled default kept", cfg.Conditional["tdd"])
	}
}

// Test 4 — an unknown routing_mode warns and defaults to llm (covers :872).
func TestParseMode_UnknownWarnsAndDefaultsToLLM(t *testing.T) {
	cfg, ws := Load("/nonexistent/phase-registry.json", map[string]string{"EVOLVE_ROUTING_MODE": "bogus"})
	if cfg.Mode != ModeDynamicLLM {
		t.Errorf("Mode = %v, want ModeDynamicLLM", cfg.Mode)
	}
	if len(ws) != 1 || ws[0].Code != "unknown-value" || ws[0].Message != `routing_mode="bogus" unknown, defaulting to llm` {
		t.Errorf("warnings %+v, want the one routing_mode unknown-value", ws)
	}
}

// Test 5 — a phases[] entry without a name is skipped whole (covers :715).
func TestApplyRegistry_PhaseWithoutNameIsSkipped(t *testing.T) {
	reg := writeRegistry(t, `{"config":{},"phases":[{"name":"","enabled":"on"},{"name":"scout"}]}`)
	cfg, _ := Load(reg, map[string]string{})
	if !reflect.DeepEqual(cfg.Order, []string{"scout"}) {
		t.Errorf("Order = %v, want [scout]", cfg.Order)
	}
	if _, ok := cfg.PhaseEnable[""]; ok {
		t.Errorf("PhaseEnable must not carry the empty name: %+v", cfg.PhaseEnable)
	}
}

// Test 6 — registry conditional rules MERGE over the compiled tdd default; the
// live registry's empty-RHS rule is accepted.
func TestLoad_RegistryConditionalMergesOverTheTddDefault(t *testing.T) {
	reg := writeRegistry(t, `{"config":{"conditional_mandatory":{"plan-review":"triage.unified_size!="}},"phases":[]}`)
	cfg, ws := Load(reg, map[string]string{})
	if len(ws) != 0 {
		t.Fatalf("unexpected warnings: %+v", ws)
	}
	if !reflect.DeepEqual(cfg.Conditional["tdd"], DefaultTddRule()) {
		t.Errorf("Conditional[tdd] = %+v, want the compiled default kept beside the registry rule", cfg.Conditional["tdd"])
	}
	if got := cfg.Conditional["plan-review"]; got.Field != "triage.unified_size" || got.Op != "!=" || got.Value != "" || got.And != nil {
		t.Errorf("Conditional[plan-review] = %+v, want {triage.unified_size != \"\"}", got)
	}
}

// Test 7 — characterisation: a malformed registry runs on the compiled baseline
// (the same value an absent registry yields). Step 2 adds the WARN (D2).
func TestLoad_MalformedRegistryFallsBackToTheCompiledBaseline(t *testing.T) {
	reg := writeRegistry(t, `{`)
	cfg, _ := Load(reg, map[string]string{})
	baseline, _ := Load("/nonexistent/phase-registry.json", map[string]string{})
	if !reflect.DeepEqual(cfg, baseline) {
		t.Errorf("a malformed registry must resolve to the compiled baseline:\n got %+v\nwant %+v", cfg, baseline)
	}
	if !reflect.DeepEqual(cfg.Mandatory, []string{"scout", "build", "audit", "ship"}) || cfg.Order != nil || cfg.DeliverableKinds != nil {
		t.Errorf("the baseline omits triage, the registry order and the deliverable kinds: %+v", cfg)
	}
}

// Test 8 — golden G1: the resolved value for the FROZEN registry copy
// (testdata/frozen_registry.json, the live file at 8e8f080f) is byte-identical
// to the golden captured before the split. The live file is guarded by the
// property-style TestLoad_RealRegistry; cycles edit it, so it is never golden'd.
func TestLoad_RealRegistry_MatchesTheGolden(t *testing.T) {
	cfg, ws := Load(filepath.Join("testdata", "frozen_registry.json"), map[string]string{})
	if len(ws) != 0 {
		t.Fatalf("the frozen registry loads clean: %+v", ws)
	}
	if got, want := marshalIndented(t, cfg), readGolden(t, "real_registry_config.golden.json"); string(got) != string(want) {
		t.Fatalf("the resolved RoutingConfig drifted from the pre-split golden:\n%s", diffLines(string(want), string(got)))
	}
}

// Test 9 — golden G2: the compiled defaults through the DEFAULT reader on a real
// absent path — zero warnings (absence is the ordinary silent case).
func TestLoad_BuiltinDefaults_MatchTheGoldenThroughTheDefaultReader(t *testing.T) {
	cfg, ws := Load("/nonexistent/phase-registry.json", nil)
	if len(ws) != 0 {
		t.Fatalf("an absent registry is silent: %+v", ws)
	}
	if got, want := marshalIndented(t, cfg), readGolden(t, "defaults.golden.json"); string(got) != string(want) {
		t.Fatalf("the compiled defaults drifted from the pre-split golden:\n%s", diffLines(string(want), string(got)))
	}
}

// Test 10 — golden G3: the kitchen sink's twelve {Code, Message} pairs, in
// order. A projection, not the Warning itself: step 2 adds Fields.
func TestLoad_KitchenSink_WarningsMatchTheGolden(t *testing.T) {
	_, ws := Load(kitchenSinkRegistry(), kitchenSinkEnv())
	type cm struct{ Code, Message string }
	got := make([]cm, 0, len(ws))
	for _, w := range ws {
		got = append(got, cm{w.Code, w.Message})
	}
	if len(got) != 12 {
		t.Fatalf("the kitchen sink yields twelve warnings, got %d: %+v", len(got), got)
	}
	if g, want := marshalIndented(t, got), readGolden(t, "kitchen_sink_warnings.golden.json"); string(g) != string(want) {
		t.Fatalf("the warning sequence drifted from the pre-split golden:\n%s", diffLines(string(want), string(g)))
	}
}

// Test 11 — a domain.json that exists but cannot be read is an error, not
// absence (covers domain.go's non-ENOENT branch).
func TestLoadDomain_ReadErrorOtherThanAbsenceIsReturned(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".evolve", "domain.json")
	if err := os.MkdirAll(path, 0o755); err != nil { // a directory at the path: a read fault, not absence
		t.Fatal(err)
	}
	_, ok, err := LoadDomain(root)
	if ok || err == nil || !strings.HasPrefix(err.Error(), "read "+path+":") {
		t.Fatalf("ok=%v err=%v, want ok=false and a `read <path>:` error", ok, err)
	}
}

// Test 12 — a deliverable-kind hole warns but the spec is still installed
// (the registry is the SSOT; the consumer sees the hole, never a default).
func TestApplyRegistry_DeliverableKindHoleWarnsButKeepsTheSpec(t *testing.T) {
	reg := writeRegistry(t, `{"config":{"deliverable_kinds":{"document":{"root":"","min_options":0}}},"phases":[]}`)
	cfg, ws := Load(reg, map[string]string{})
	if len(ws) != 1 || ws[0].Message != `deliverable_kinds[document]: root and min_options must be set (root="", min_options=0)` {
		t.Fatalf("warnings %+v, want the one hole warning", ws)
	}
	if _, ok := cfg.DeliverableKinds["document"]; !ok {
		t.Errorf("the spec is assigned before it is validated: %+v", cfg.DeliverableKinds)
	}
}

// Test 13 — Q1: only the literal "0" disables the registry read.
func TestLoad_UsePhaseRegistry_OnlyTheLiteralZeroDisables(t *testing.T) {
	reg := writeRegistry(t, `{"config":{},"phases":[{"name":"scout"}]}`)
	if cfg, _ := Load(reg, map[string]string{"EVOLVE_USE_PHASE_REGISTRY": "0"}); len(cfg.Order) != 0 {
		t.Errorf(`"0" disables the registry: Order = %v`, cfg.Order)
	}
	for _, v := range []string{"false", "off", ""} {
		if cfg, _ := Load(reg, map[string]string{"EVOLVE_USE_PHASE_REGISTRY": v}); len(cfg.Order) != 1 {
			t.Errorf("%q keeps the registry on (Q1): Order = %v", v, cfg.Order)
		}
	}
}

// diffLines renders the first differing line of two texts for a failure message.
func diffLines(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(w) && i < len(g); i++ {
		if w[i] != g[i] {
			return "line " + itoa(i+1) + ":\n want " + w[i] + "\n  got " + g[i]
		}
	}
	return "length differs: want " + itoa(len(w)) + " lines, got " + itoa(len(g))
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
