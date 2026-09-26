package policy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func writePolicy(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, "policy.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad_AbsentIsEmptyNoError(t *testing.T) {
	p, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("absent policy must not error: %v", err)
	}
	if len(p.MandatoryPhases) != 0 || len(p.Pins) != 0 {
		t.Errorf("absent policy must be empty, got %+v", p)
	}
}

func TestLoad_MalformedIsError(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(writePolicy(t, dir, "{ not json")); err == nil {
		t.Fatal("malformed policy must error (fail loudly, not silently disable)")
	}
}

func TestLoad_ParsesMandatoryAndPins(t *testing.T) {
	dir := t.TempDir()
	path := writePolicy(t, dir, `{
		"mandatory_phases": ["scout","build","audit","ship"],
		"pins": { "audit": {"cli":"claude-tmux","model":"claude-opus-4-8"} }
	}`)
	p, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.MandatoryPhases) != 4 || p.MandatoryPhases[2] != "audit" {
		t.Errorf("MandatoryPhases=%v", p.MandatoryPhases)
	}
	pin, ok := p.PinFor("audit")
	if !ok || pin.CLI != "claude-tmux" || pin.Model != "claude-opus-4-8" {
		t.Errorf("PinFor(audit)=%+v ok=%v", pin, ok)
	}
}

func TestLoad_ReadErrorNotMissing(t *testing.T) {
	dir := t.TempDir() // exists, but os.ReadFile fails on a directory
	p, err := Load(dir)
	if err == nil {
		t.Fatalf("reading a directory as policy must error, got policy %+v", p)
	}
	if !strings.Contains(err.Error(), "policy: read "+dir) {
		t.Errorf("error %q must name the read failure on %q", err, dir)
	}
	if len(p.MandatoryPhases) != 0 || len(p.Pins) != 0 {
		t.Errorf("on error Load must return zero Policy, got %+v", p)
	}
}

func TestTierRank(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"fast", 1},
		{"balanced", 2},
		{"deep", 3},
		{"top", 4},
		{"haiku", 1},
		{"sonnet", 2},
		{"opus", 3},
		{"  FAST  ", 1},
		{"Balanced", 2},
		{"DEEP", 3},
		{"claude-haiku-4-5", 1},
		{"claude-sonnet-4-6", 2},
		{"claude-opus-4-8", 3},
		{"gpt-5.5", 0},
		{"", 0},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := TierRank(c.in); got != c.want {
				t.Errorf("TierRank(%q)=%d, want %d", c.in, got, c.want)
			}
		})
	}
}

func TestMergeMandatory(t *testing.T) {
	cases := []struct {
		name string
		pol  Policy
		base []string
		want []string
	}{
		{"empty-policy-returns-base", Policy{}, []string{"scout", "build"}, []string{"scout", "build"}},
		{"adds-new", Policy{MandatoryPhases: []string{"security-scan"}}, []string{"scout", "audit"}, []string{"scout", "audit", "security-scan"}},
		{"dedups-existing", Policy{MandatoryPhases: []string{"audit", "scout"}}, []string{"scout", "audit"}, []string{"scout", "audit"}},
		{"skips-empty", Policy{MandatoryPhases: []string{"", "ship"}}, []string{"scout"}, []string{"scout", "ship"}},
		{"empty-base", Policy{MandatoryPhases: []string{"audit"}}, nil, []string{"audit"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.pol.MergeMandatory(c.base)
			if !slices.Equal(got, c.want) {
				t.Errorf("MergeMandatory(%v over %v)=%v, want %v", c.pol.MandatoryPhases, c.base, got, c.want)
			}
		})
	}
}

func TestMergeMandatory_DoesNotMutateBase(t *testing.T) {
	base := []string{"scout", "audit"}
	pol := Policy{MandatoryPhases: []string{"security-scan", "scout"}}
	got := pol.MergeMandatory(base)
	if !slices.Equal(base, []string{"scout", "audit"}) {
		t.Errorf("base was mutated: %v", base)
	}
	if !slices.Equal(got, []string{"scout", "audit", "security-scan"}) {
		t.Errorf("got=%v, want [scout audit security-scan]", got)
	}
}

func TestPinFor_EmptyPinIsAbsent(t *testing.T) {
	p := Policy{Pins: map[string]Pin{"audit": {}}}
	if _, ok := p.PinFor("audit"); ok {
		t.Error("an all-empty pin must report absent")
	}
	if _, ok := p.PinFor("missing"); ok {
		t.Error("missing phase must report absent")
	}
}

func TestValidatePin_NilProfileOK(t *testing.T) {
	if err := ValidatePin("audit", Pin{CLI: "codex-tmux"}, nil); err != nil {
		t.Errorf("nil profile must validate ok, got %v", err)
	}
}

func TestValidatePin_CLIOutsideAllowed(t *testing.T) {
	prof := &profiles.Profile{AllowedCLIs: []string{"claude", "agy"}}
	if err := ValidatePin("audit", Pin{CLI: "codex-tmux"}, prof); err == nil {
		t.Error("codex pin must be rejected when allowed_clis=[claude,agy]")
	}
	if err := ValidatePin("audit", Pin{CLI: "claude-tmux"}, prof); err != nil {
		t.Errorf("claude pin must pass allowed_clis=[claude,agy]: %v", err)
	}
}

func TestValidatePin_AllowedAllPasses(t *testing.T) {
	prof := &profiles.Profile{AllowedCLIs: []string{"all"}}
	if err := ValidatePin("audit", Pin{CLI: "codex-tmux"}, prof); err != nil {
		t.Errorf("allowed_clis=[all] must accept any cli: %v", err)
	}
}

func TestValidatePin_ModelTierWithinEnvelope(t *testing.T) {
	prof := &profiles.Profile{ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "deep", Max: "deep"}}
	if err := ValidatePin("audit", Pin{Model: "claude-opus-4-8"}, prof); err != nil {
		t.Errorf("opus pin must sit within deep..deep: %v", err)
	}
}

func TestValidatePin_ModelTierOutsideEnvelope(t *testing.T) {
	prof := &profiles.Profile{ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "deep", Max: "deep"}}
	if err := ValidatePin("audit", Pin{Model: "claude-haiku-4-5"}, prof); err == nil {
		t.Error("haiku pin must be rejected when envelope=deep..deep")
	}
}

func TestValidatePin_UnclassifiableModelSkipsEnvelope(t *testing.T) {
	prof := &profiles.Profile{ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "deep", Max: "deep"}}
	if err := ValidatePin("audit", Pin{Model: "gpt-5.5"}, prof); err != nil {
		t.Errorf("unclassifiable model must skip envelope check, got %v", err)
	}
}

func TestValidatePin_TopTierOutsideDeepEnvelope(t *testing.T) {
	prof := &profiles.Profile{ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "deep", Max: "deep"}}
	if err := ValidatePin("audit", Pin{Model: "top"}, prof); err == nil {
		t.Error("top pin must be rejected when envelope max=deep")
	}
}

func TestPolicy_QuotaResetConfig(t *testing.T) {
	if got := (Policy{}).QuotaResetConfig(); got != (QuotaResetConfig{}) {
		t.Errorf("absent QuotaReset must yield zero config, got %+v", got)
	}
	want := QuotaResetConfig{ResetAt: "2026-06-20T12:00:00Z", DefaultHours: 5.5}
	if got := (Policy{QuotaReset: &want}).QuotaResetConfig(); got != want {
		t.Errorf("QuotaResetConfig() = %+v, want %+v", got, want)
	}
}

func TestPolicy_ConfigAccessors(t *testing.T) {
	// The typed var declarations name each *Policy type for apicover.
	var bridge BridgePolicy = (Policy{}).BridgeConfig()
	if !reflect.DeepEqual(bridge, BridgePolicy{}) {
		t.Errorf("absent Bridge must yield zero BridgePolicy, got %+v", bridge)
	}

	var fanout FanoutPolicy = (Policy{}).FanoutConfig()
	if fanout.Concurrency != 2 {
		t.Errorf("default FanoutPolicy.Concurrency = %d, want 2", fanout.Concurrency)
	}
	if fanout.TrackWorkers == nil || !*fanout.TrackWorkers {
		t.Error("default FanoutPolicy.TrackWorkers must be true")
	}

	var observer ObserverPolicy = (Policy{}).ObserverConfig()
	if observer.Autospawn == nil || !*observer.Autospawn {
		t.Error("default ObserverPolicy.Autospawn must be true")
	}
	if observer.StallS == nil || *observer.StallS != 600 {
		t.Error("default ObserverPolicy.StallS must be 600")
	}
}

func TestWorkflowConfig(t *testing.T) {
	disabled := false
	got := (Policy{Workflow: &WorkflowPolicy{
		MaxConsecutiveFails:   3,
		MaxCyclesCap:          8,
		AutoPrune:             &disabled,
		DiffComplexityDisable: true,
		AuditorTierOverride:   "deep",
	}}).WorkflowConfig()
	if got.MaxConsecutiveFails != 3 || got.MaxCyclesCap != 8 || got.AutoPrune ||
		!got.DiffComplexityDisable || got.AuditorTierOverride != "deep" {
		t.Fatalf("WorkflowConfig() = %+v, want configured values", got)
	}

	defaults := (Policy{Workflow: &WorkflowPolicy{}}).WorkflowConfig()
	if defaults.MaxConsecutiveFails != 1 || defaults.MaxCyclesCap != 25 || !defaults.AutoPrune {
		t.Fatalf("WorkflowConfig() defaults = %+v, want max fails=1 max cycles=25 auto prune=true", defaults)
	}

	if defaults.CycleBudget != "enforce" {
		t.Errorf("default CycleBudget = %q, want enforce (advisor decides when --cycles omitted)", defaults.CycleBudget)
	}
	if nilWF := (Policy{}).WorkflowConfig(); nilWF.CycleBudget != "enforce" {
		t.Errorf("nil-workflow CycleBudget = %q, want enforce", nilWF.CycleBudget)
	}
	if off := (Policy{Workflow: &WorkflowPolicy{CycleBudget: "off"}}).WorkflowConfig(); off.CycleBudget != "off" {
		t.Errorf("explicit CycleBudget=off was not honored, got %q", off.CycleBudget)
	}
}

func TestGatesConfig(t *testing.T) {
	got := (Policy{Gates: &GatesPolicy{
		ContractGate: "shadow",
		ReviewGate:   "enforce",
	}}).GatesConfig()

	if got.ContractGate != "shadow" || got.EvalGate != "enforce" ||
		got.TriageCapGate != "enforce" || got.ReviewGate != "enforce" {
		t.Fatalf("GatesConfig() = %+v, want partial overrides with defaults retained", got)
	}
}

func TestRetryConfig(t *testing.T) {
	defaults := (Policy{}).RetryConfig()
	if defaults.PhaseMaxAttempts != 2 || defaults.RetryBackoffBaseS != 5 ||
		defaults.PhaseLatencyCeilingS != 900 || defaults.ContractCorrectionRetries != 2 {
		t.Fatalf("RetryConfig() defaults = %+v", defaults)
	}

	got := (Policy{Retry: &RetryPolicy{
		PhaseMaxAttempts:          9,
		RetryBackoffBaseS:         3,
		PhaseLatencyCeilingS:      120,
		ContractCorrectionRetries: 9,
	}}).RetryConfig()
	if got.PhaseMaxAttempts != 5 || got.RetryBackoffBaseS != 3 ||
		got.PhaseLatencyCeilingS != 120 || got.ContractCorrectionRetries != 5 {
		t.Fatalf("RetryConfig() configured = %+v", got)
	}

	var loaded Policy
	if err := json.Unmarshal([]byte(`{"retry":{"retry_backoff_base_s":0,"contract_correction_retries":0}}`), &loaded); err != nil {
		t.Fatal(err)
	}
	disabled := loaded.RetryConfig()
	if disabled.RetryBackoffBaseS != 0 || disabled.ContractCorrectionRetries != 0 {
		t.Fatalf("RetryConfig() explicit zero = %+v", disabled)
	}
}

func TestCatalogConfig_AllowedFamilies(t *testing.T) {
	if got := (Policy{}).CatalogConfig(); got.AllowedFamilies != nil {
		t.Errorf("absent catalog policy must yield nil AllowedFamilies (no constraint), got %+v", got.AllowedFamilies)
	}

	if got := (Policy{Catalog: &CatalogPolicy{}}).CatalogConfig(); got.AllowedFamilies != nil {
		t.Errorf("catalog policy with no allowed_families must yield nil (no constraint), got %+v", got.AllowedFamilies)
	}

	want := map[string][]string{"agy": {"gemini"}, "claude": {"claude"}}
	got := (Policy{Catalog: &CatalogPolicy{AllowedFamilies: want}}).CatalogConfig()
	if !reflect.DeepEqual(got.AllowedFamilies, want) {
		t.Errorf("CatalogConfig().AllowedFamilies = %+v, want %+v", got.AllowedFamilies, want)
	}
}

func TestLoad_ParsesCatalogAllowedFamilies(t *testing.T) {
	dir := t.TempDir()
	path := writePolicy(t, dir, `{
		"catalog": { "allowed_families": { "agy": ["gemini"], "codex": ["gpt"] } }
	}`)
	p, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	got := p.CatalogConfig().AllowedFamilies
	if !reflect.DeepEqual(got["agy"], []string{"gemini"}) {
		t.Errorf("agy allowed_families = %v, want [gemini]", got["agy"])
	}
	if !reflect.DeepEqual(got["codex"], []string{"gpt"}) {
		t.Errorf("codex allowed_families = %v, want [gpt]", got["codex"])
	}
}

func TestWorkflowConfig_UniversalFallbackExcludeDefaultsToAgy(t *testing.T) {
	if got := (Policy{}).WorkflowConfig().UniversalFallbackExclude; len(got) != 1 || got[0] != "agy" {
		t.Fatalf("default = %v, want [agy]", got)
	}
	lifted := Policy{Workflow: &WorkflowPolicy{UniversalFallbackExclude: []string{}}}
	if got := lifted.WorkflowConfig().UniversalFallbackExclude; len(got) != 0 {
		t.Fatalf("an explicit empty list lifts the ban: %v", got)
	}
	custom := Policy{Workflow: &WorkflowPolicy{UniversalFallbackExclude: []string{"agy", "ollama"}}}
	if got := custom.WorkflowConfig().UniversalFallbackExclude; len(got) != 2 {
		t.Fatalf("custom = %v", got)
	}
}
