package policy_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

var handoffPairNames = []string{"build-audit", "intent-scout", "scout-triage", "tdd-build", "triage-tdd"}

func sortedPairNames(cfg policy.HandoffConfig) []string {
	names := make([]string, 0, len(cfg))
	for name := range cfg {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func TestHandoffPolicy_AnAbsentBlockResolvesEveryPairToShadow(t *testing.T) {
	got := policy.Policy{}.HandoffConfig()

	if !reflect.DeepEqual(sortedPairNames(got), handoffPairNames) {
		t.Fatalf("pairs = %v, want exactly %v", sortedPairNames(got), handoffPairNames)
	}
	for name, pair := range got {
		if pair.Stage != "shadow" {
			t.Errorf("pair %q stage = %q, want the compiled default shadow", name, pair.Stage)
		}
	}
}

func TestHandoffPolicy_AFileWithoutTheBlockResolvesEveryPairToShadow(t *testing.T) {
	p, err := loadPolicyText(t, `{"fleet": {"count": 4}}`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	got := p.HandoffConfig()

	if !reflect.DeepEqual(sortedPairNames(got), handoffPairNames) {
		t.Fatalf("pairs = %v, want exactly %v", sortedPairNames(got), handoffPairNames)
	}
	for name, pair := range got {
		if pair.Stage != "shadow" {
			t.Errorf("pair %q stage = %q, want shadow", name, pair.Stage)
		}
	}
}

func TestHandoffPolicy_AnEmptyBlockResolvesEveryPairToShadow(t *testing.T) {
	p, err := loadPolicyText(t, `{"handoff": {}}`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	for name, pair := range p.HandoffConfig() {
		if pair.Stage != "shadow" {
			t.Errorf("pair %q stage = %q, want shadow", name, pair.Stage)
		}
	}
}

func TestHandoffPolicy_UnknownPairRefusedAtDecode(t *testing.T) {
	for name, pair := range map[string]string{
		"a misspelt pair":         "scout-tdd",
		"a reversed pair":         "audit-build",
		"a phase name":            "build",
		"a wrong case":            "Scout-Triage",
		"a trailing space":        "scout-triage ",
		"the empty name":          "",
		"a pair the plan dropped": "audit-ship",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := loadPolicyText(t, `{"handoff": {"`+pair+`": {"stage": "shadow"}}}`)

			if err == nil {
				t.Fatalf("pair %q was accepted; an unknown pair must fail the load", pair)
			}
			if !strings.Contains(err.Error(), "handoff") {
				t.Errorf("error %q must name the handoff block", err)
			}
		})
	}
}

func TestHandoffPolicy_AnUnknownPairErrorNamesThePairAndTheKnownOnes(t *testing.T) {
	_, err := loadPolicyText(t, `{"handoff": {"scout-tdd": {"stage": "enforce"}}}`)

	if err == nil {
		t.Fatal("an unknown pair must fail the load")
	}
	for _, want := range append([]string{"scout-tdd"}, handoffPairNames...) {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q must name %q so the operator can fix the typo", err, want)
		}
	}
}

func TestHandoffPolicy_AnUnknownPairBesideAKnownOneStillRefused(t *testing.T) {
	_, err := loadPolicyText(t, `{"handoff": {"scout-triage": {"stage": "enforce"}, "scout-tdd": {"stage": "enforce"}}}`)

	if err == nil {
		t.Fatal("one unknown pair among known ones must still fail the load")
	}
}

func TestHandoffPolicy_InvalidStageRefused(t *testing.T) {
	for _, stage := range []string{"off", "advisory", "ENFORCE", "Shadow", "enforced", "shadow ", "on", "warn", "0"} {
		t.Run(stage, func(t *testing.T) {
			_, err := loadPolicyText(t, `{"handoff": {"tdd-build": {"stage": "`+stage+`"}}}`)

			if err == nil {
				t.Fatalf("stage %q was accepted; only shadow and enforce are stages", stage)
			}
			for _, want := range []string{"handoff", "tdd-build", "shadow", "enforce"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q must name %q", err, want)
				}
			}
		})
	}
}

func TestHandoffPolicy_AStageThatIsNotAStringRefused(t *testing.T) {
	for name, stage := range map[string]string{"a number": `1`, "a bool": `true`, "a list": `["shadow"]`} {
		t.Run(name, func(t *testing.T) {
			if _, err := loadPolicyText(t, `{"handoff": {"tdd-build": {"stage": `+stage+`}}}`); err == nil {
				t.Fatalf("stage %s was accepted", stage)
			}
		})
	}
}

func TestHandoffPolicy_AnUnknownKeyInsideAPairRefused(t *testing.T) {
	_, err := loadPolicyText(t, `{"handoff": {"tdd-build": {"stge": "enforce"}}}`)

	if err == nil || !strings.Contains(err.Error(), "handoff") {
		t.Fatalf("a typo inside a pair must fail the load naming the block, got %v", err)
	}
}

func TestHandoffPolicy_ABlockThatIsNotAnObjectRefused(t *testing.T) {
	for name, block := range map[string]string{"a list": `[]`, "a string": `"enforce"`, "a number": `1`} {
		t.Run(name, func(t *testing.T) {
			if _, err := loadPolicyText(t, `{"handoff": `+block+`}`); err == nil {
				t.Fatalf("handoff block %s was accepted", block)
			}
		})
	}
}

func TestHandoffPolicy_EnforceOnOnePairLeavesTheOthersAtShadow(t *testing.T) {
	for _, pair := range handoffPairNames {
		t.Run(pair, func(t *testing.T) {
			p, err := loadPolicyText(t, `{"handoff": {"`+pair+`": {"stage": "enforce"}}}`)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}

			got := p.HandoffConfig()

			if !reflect.DeepEqual(sortedPairNames(got), handoffPairNames) {
				t.Fatalf("pairs = %v, want exactly %v", sortedPairNames(got), handoffPairNames)
			}
			for name, cfg := range got {
				want := "shadow"
				if name == pair {
					want = "enforce"
				}
				if cfg.Stage != want {
					t.Errorf("pair %q stage = %q, want %q", name, cfg.Stage, want)
				}
			}
		})
	}
}

func TestHandoffPolicy_EveryPairTakesItsOwnStageAndJudgeTier(t *testing.T) {
	p, err := loadPolicyText(t, `{"handoff": {
		"scout-triage": {"stage": "shadow", "judge_tier": "fast"},
		"triage-tdd": {"stage": "enforce", "judge_tier": "balanced"},
		"tdd-build": {"stage": "enforce", "judge_tier": "deep"},
		"build-audit": {"stage": "shadow"},
		"intent-scout": {"judge_tier": "deep"}
	}}`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := policy.HandoffConfig{
		"scout-triage": {Stage: "shadow", JudgeTier: "fast"},
		"triage-tdd":   {Stage: "enforce", JudgeTier: "balanced"},
		"tdd-build":    {Stage: "enforce", JudgeTier: "deep"},
		"build-audit":  {Stage: "shadow"},
		"intent-scout": {Stage: "shadow", JudgeTier: "deep"},
	}
	if got := p.HandoffConfig(); !reflect.DeepEqual(got, want) {
		t.Fatalf("HandoffConfig() = %+v, want %+v", got, want)
	}
}

func TestHandoffPolicy_ANamedPairWithoutAStageStaysAtShadow(t *testing.T) {
	p, err := loadPolicyText(t, `{"handoff": {"build-audit": {}}}`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := p.HandoffConfig()["build-audit"]; got != (policy.HandoffPairConfig{Stage: "shadow"}) {
		t.Fatalf("build-audit = %+v, want stage shadow and no judge tier", got)
	}
}

func TestHandoffPolicy_ReturnsACopyTheCallerCannotCorrupt(t *testing.T) {
	first := policy.Policy{}.HandoffConfig()
	first["tdd-build"] = policy.HandoffPairConfig{Stage: "enforce", JudgeTier: "deep"}
	delete(first, "build-audit")

	again := policy.Policy{}.HandoffConfig()

	if again["tdd-build"].Stage != "shadow" || again["tdd-build"].JudgeTier != "" {
		t.Errorf("a caller's edit leaked into the compiled default: %+v", again["tdd-build"])
	}
	if _, ok := again["build-audit"]; !ok {
		t.Error("a caller's delete leaked into the compiled default")
	}
}

func TestHandoffPolicy_TheLoadedBlockIsNotMutatedByItsConfig(t *testing.T) {
	p, err := loadPolicyText(t, `{"handoff": {"tdd-build": {"stage": "enforce"}}}`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	p.HandoffConfig()["tdd-build"] = policy.HandoffPairConfig{Stage: "shadow"}

	if got := p.HandoffConfig()["tdd-build"].Stage; got != "enforce" {
		t.Fatalf("a caller's edit changed the loaded policy: stage = %q", got)
	}
}

func TestRuntimeReference_DocumentsTheHandoffBlock(t *testing.T) {
	doc := runtimeReference(t)

	row := ""
	for _, line := range strings.Split(doc, "\n") {
		if strings.Contains(line, "`handoff`") && strings.Contains(line, "policy.json") {
			row = line
			break
		}
	}
	if row == "" {
		t.Fatal("no runtime-reference.md line documents the handoff block in .evolve/policy.json")
	}
	for _, want := range append([]string{"stage", "shadow", "enforce", "judge_tier"}, handoffPairNames...) {
		if !strings.Contains(row, want) {
			t.Errorf("the runtime-reference.md line for the handoff block never mentions %q", want)
		}
	}
}

func TestHandoffPolicy_TheCheckedInPolicyFileCarriesNoHandoffBlock(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", ".evolve", "policy.json"))
	if err != nil {
		t.Fatalf("read the checked-in policy: %v", err)
	}
	var blocks map[string]json.RawMessage
	if err := json.Unmarshal(raw, &blocks); err != nil {
		t.Fatalf("decode the checked-in policy: %v", err)
	}

	if _, present := blocks["handoff"]; present {
		t.Fatal("the checked-in .evolve/policy.json must leave the handoff block to the compiled defaults")
	}
}
