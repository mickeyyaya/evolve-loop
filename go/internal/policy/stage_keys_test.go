package policy_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

var gateFieldsOutsideTheStageList = []string{
	"contract_gate", "eval_gate", "triage_cap_gate", "review_gate",
	"topn_gate", "report_size_gate", "manifest_gate", "repo_contract_gate",
}

var effectiveStageDefaults = map[string]string{
	"inbox.curator.stage":                        "shadow",
	"inbox.curator.trailer.stage":                "shadow",
	"gc.remote_sweep.stage":                      "shadow",
	"pr.merge_window.stage":                      "off",
	"workflow.findings_repair.code-review.stage": "shadow",
	"workflow.convergence.stage":                 "shadow",
	"workflow.retry_ladder.stage":                "shadow",
	"workflow.retry_ladder.audit_fix.stage":      "shadow",
	"workflow.repair.fixability.stage":           "shadow",
	"workflow.cycle_spend.stage":                 "shadow",
	"workflow.builder_selfcheck.stage":           "shadow",
	"workflow.supersede.stage":                   "shadow",
	"swarm.stage":                                "shadow",
	"failure_disposition.stage":                  "shadow",
	"merge_gate.stage":                           "shadow",
	"parallel_evaluate.stage":                    "off",
	"catalog.refresh_stage":                      "enforce",
	"docs_floor.stage":                           "enforce",
	"docs_floor.ste_stage":                       "shadow",
	"comment_floor.stage":                        "shadow",
	"held_proposals.stage":                       "shadow",
	"cover_changed.stage":                        "enforce",
	"retrospectives.stage":                       "shadow",
	"integrity.stage":                            "shadow",
	"fleet.budget.stage":                         "off",
	"fleet.orchestrator.stage":                   "shadow",
	"fleet.integration.stage":                    "shadow",
	"fleet.orchestrator.window.adaptive.stage":   "off",
	"audit_cascade.stage":                        "off",
	"isolation.i2.stage":                         "shadow",
	"isolation.i3.stage":                         "shadow",
	"isolation.i5.stage":                         "shadow",
	"isolation.i7.stage":                         "shadow",
	"isolation.i8.stage":                         "legacy",
	"isolation.i15.stage":                        "shadow",
	"isolation.context.stage":                    "shadow",
	"isolation.dispatch_requests.stage":          "shadow",
	"isolation.inbox_requests.stage":             "enforce",
	"healing.detector.stage":                     "shadow",
	"healing.orchestrator.stage":                 "shadow",
	"healing.validator.stage":                    "shadow",
	"healing.live_reaction.stage":                "shadow",
	"gates.scan_cache_gate":                      "shadow",
	"handoff.build-audit.stage":                  "shadow",
	"handoff.intent-scout.stage":                 "shadow",
	"handoff.scout-triage.stage":                 "shadow",
	"handoff.tdd-build.stage":                    "shadow",
	"handoff.triage-tdd.stage":                   "shadow",
}

type stageProbe struct {
	set  string
	want string
	with map[string]string
}

var effectiveStageProbes = map[string]stageProbe{
	"pr.merge_window.stage":                      {set: "on", want: "on"},
	"healing.detector.stage":                     {set: "off", want: "off"},
	"healing.orchestrator.stage":                 {set: "off", want: "off"},
	"healing.validator.stage":                    {set: "off", want: "off"},
	"audit_cascade.stage":                        {set: "shadow", want: "shadow"},
	"isolation.context.stage":                    {set: "shadow", want: "shadow"},
	"workflow.findings_repair.code-review.stage": {set: "enforce", want: "shadow"},
	"fleet.orchestrator.window.adaptive.stage":   {set: "enforce", want: "enforce", with: map[string]string{"fleet.orchestrator.stage": "enforce"}},
	"fleet.budget.stage":                         {set: "enforce", want: "enforce"},
}

func probeFor(key string) stageProbe {
	if probe, ok := effectiveStageProbes[key]; ok {
		return probe
	}
	if effectiveStageDefaults[key] == "enforce" {
		return stageProbe{set: "shadow", want: "shadow"}
	}
	return stageProbe{set: "enforce", want: "enforce"}
}

func policyWith(t *testing.T, values map[string]string) policy.Policy {
	t.Helper()
	root := map[string]any{}
	for key, value := range values {
		parts := strings.Split(key, ".")
		node := root
		for _, part := range parts[:len(parts)-1] {
			next, ok := node[part].(map[string]any)
			if !ok {
				next = map[string]any{}
				node[part] = next
			}
			node = next
		}
		node[parts[len(parts)-1]] = value
	}
	raw, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	p, err := policy.Parse(raw)
	if err != nil {
		t.Fatalf("policy %s: %v", raw, err)
	}
	return p
}

func TestStageKeys_ListEachSchemaKeyTheGateAndTheHandoffPairsOnce(t *testing.T) {
	t.Parallel()
	keys := policy.StageKeys()
	seen := map[string]bool{}
	for _, key := range keys {
		if seen[key] {
			t.Errorf("stage key %s is listed twice", key)
		}
		seen[key] = true
	}
	for _, field := range policy.StageFields() {
		if !seen[field.Path] {
			t.Errorf("schema stage field %s is not in StageKeys", field.Path)
		}
	}
	for _, pair := range handoffPairNames {
		if !seen["handoff."+pair+".stage"] {
			t.Errorf("handoff pair %s has no stage key", pair)
		}
	}
	if !seen["gates.scan_cache_gate"] {
		t.Error("gates.scan_cache_gate is not in StageKeys")
	}
	if want := len(policy.StageFields()) + 1 + len(handoffPairNames); len(keys) != want {
		t.Errorf("StageKeys has %d keys, want %d: the schema walk, one gate and %d pairs", len(keys), want, len(handoffPairNames))
	}
}

func TestStageKeysCoverGateAndHandoffFields(t *testing.T) {
	t.Parallel()
	keys := policy.StageKeys()
	gates := reflect.TypeOf(policy.GatesPolicy{})
	named := map[string]bool{}
	for i := 0; i < gates.NumField(); i++ {
		name := strings.Split(gates.Field(i).Tag.Get("json"), ",")[0]
		if !strings.HasSuffix(name, "_gate") || gates.Field(i).Type.Kind() != reflect.String {
			continue
		}
		named[name] = true
		if !slices.Contains(keys, "gates."+name) && !slices.Contains(gateFieldsOutsideTheStageList, name) {
			t.Errorf("gate field %s is in neither StageKeys nor the list of gates that need no stage key", name)
		}
	}
	for _, name := range gateFieldsOutsideTheStageList {
		if !named[name] {
			t.Errorf("gateFieldsOutsideTheStageList names %s, which is not a gate field", name)
		}
		if slices.Contains(keys, "gates."+name) {
			t.Errorf("gate field %s is both in StageKeys and outside it", name)
		}
	}
	for pair := range (policy.Policy{}).HandoffConfig() {
		if !slices.Contains(keys, "handoff."+pair+".stage") {
			t.Errorf("handoff pair %s has no stage key", pair)
		}
	}
}

func TestStageFields_NameTheGoTypeOfEachSchemaStage(t *testing.T) {
	t.Parallel()
	types := map[string]string{}
	for _, field := range policy.StageFields() {
		types[field.Path] = field.TypeName
	}
	if types["held_proposals.stage"] != "string" || types["fleet.orchestrator.window.adaptive.stage"] != "policy.AdaptiveStage" {
		t.Errorf("type names = %q and %q", types["held_proposals.stage"], types["fleet.orchestrator.window.adaptive.stage"])
	}
}

func TestEffectiveStagesMatchEachKeyDefault(t *testing.T) {
	t.Parallel()
	got, err := policy.Policy{}.EffectiveStages()
	if err != nil {
		t.Fatalf("EffectiveStages: %v", err)
	}
	for _, key := range policy.StageKeys() {
		want, ok := effectiveStageDefaults[key]
		if !ok {
			t.Errorf("stage key %s has no expected default in the test", key)
		}
		if got[key] != want {
			t.Errorf("%s defaults to %q, want %q", key, got[key], want)
		}
	}
	if len(got) != len(policy.StageKeys()) || len(effectiveStageDefaults) != len(got) {
		t.Errorf("EffectiveStages has %d keys, StageKeys %d, the default table %d", len(got), len(policy.StageKeys()), len(effectiveStageDefaults))
	}
}

func TestEffectiveStages_ReadEachKeyFromItsOwnBlockAndNoOther(t *testing.T) {
	t.Parallel()
	for _, key := range policy.StageKeys() {
		probe := probeFor(key)
		values := map[string]string{key: probe.set}
		for other, value := range probe.with {
			values[other] = value
		}
		got, err := policyWith(t, values).EffectiveStages()
		if err != nil {
			t.Fatalf("%s: EffectiveStages: %v", key, err)
		}
		if got[key] != probe.want {
			t.Errorf("%s set to %q resolves to %q, want %q", key, probe.set, got[key], probe.want)
		}
		for other, def := range effectiveStageDefaults {
			if _, moved := values[other]; !moved && got[other] != def {
				t.Errorf("setting %s moved %s to %q, want its default %q", key, other, got[other], def)
			}
		}
	}
}

func TestEffectiveStages_AnEnforcedKeyReadsEnforce(t *testing.T) {
	t.Parallel()
	got, err := policyWith(t, map[string]string{"inbox.curator.stage": "enforce", "handoff.tdd-build.stage": "enforce", "gates.scan_cache_gate": "enforce"}).EffectiveStages()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"inbox.curator.stage", "handoff.tdd-build.stage", "gates.scan_cache_gate"} {
		if got[key] != "enforce" {
			t.Errorf("%s = %q, want enforce", key, got[key])
		}
	}
}

func TestEffectiveStages_IsolationKeysReadTheValueTheirConsumerActsOn(t *testing.T) {
	t.Parallel()
	cases := []struct {
		raw          string
		wantIsolated string
		wantFlow     string
		wantQueue    string
	}{
		{"ENFORCE", "enforce", "legacy", "shadow"},
		{" shadow ", "shadow", "legacy", "shadow"},
		{"bogus", "off", "legacy", "shadow"},
		{"0", "off", "legacy", "shadow"},
		{"off", "off", "legacy", "shadow"},
		{"enforce", "enforce", "enforce", "enforce"},
	}
	for _, c := range cases {
		got, err := policyWith(t, map[string]string{
			"isolation.i2.stage": c.raw, "isolation.i3.stage": c.raw, "isolation.i5.stage": c.raw,
			"isolation.i7.stage": c.raw, "isolation.i8.stage": c.raw, "isolation.i15.stage": c.raw,
		}).EffectiveStages()
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"isolation.i2.stage", "isolation.i3.stage", "isolation.i5.stage", "isolation.i7.stage"} {
			if got[key] != c.wantIsolated {
				t.Errorf("%s set to %q resolves to %q, want %q", key, c.raw, got[key], c.wantIsolated)
			}
		}
		if got["isolation.i15.stage"] != c.wantQueue {
			t.Errorf("isolation.i15.stage set to %q resolves to %q, want %q", c.raw, got["isolation.i15.stage"], c.wantQueue)
		}
		if got["isolation.i8.stage"] != c.wantFlow {
			t.Errorf("isolation.i8.stage set to %q resolves to %q, want %q", c.raw, got["isolation.i8.stage"], c.wantFlow)
		}
	}
}

func TestResolveFlowStage_KnownWordsPassAndAnythingElseIsLegacyAndUnknown(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		stage string
		known bool
	}{
		"": {"legacy", true}, "legacy": {"legacy", true}, "shadow": {"shadow", true}, "enforce": {"enforce", true},
		"Enforce": {"legacy", false}, "off": {"legacy", false},
	}
	for raw, want := range cases {
		stage, known := policy.ResolveFlowStage(raw)
		if stage != want.stage || known != want.known {
			t.Errorf("ResolveFlowStage(%q) = %q, %v; want %q, %v", raw, stage, known, want.stage, want.known)
		}
	}
}

func TestResolveIsolationStage_EmptyIsShadowZeroAndUnknownAreOffAndWordsFold(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"": "shadow", "0": "off", "off": "off", "shadow": "shadow", "enforce": "enforce",
		" Enforce\n": "enforce", "OFF": "off", "bogus": "off", "on": "off",
	}
	for raw, want := range cases {
		if got := policy.ResolveIsolationStage(raw); got != want {
			t.Errorf("ResolveIsolationStage(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestResolveQueueStage_OnlyTheExactWordEnforceEnforces(t *testing.T) {
	t.Parallel()
	cases := map[string]string{"enforce": "enforce", "": "shadow", "shadow": "shadow", "off": "shadow", "ENFORCE": "shadow", " enforce": "shadow"}
	for raw, want := range cases {
		if got := policy.ResolveQueueStage(raw); got != want {
			t.Errorf("ResolveQueueStage(%q) = %q, want %q", raw, got, want)
		}
	}
}
