package setup

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func applyFixture(t *testing.T) (DetectReport, *profiles.Loader) {
	t.Helper()
	project, evolveDir := fixtureRepo(t)
	rep := Detect(context.Background(), DetectOptions{
		ProjectRoot: project, EvolveDir: evolveDir,
		Env:     func(string) string { return "" },
		Now:     func() time.Time { return time.Unix(0, 0).UTC() },
		Doctor:  fakeDoctor,
		CapTier: func(string) string { return "full" },
	})
	return rep, profiles.NewFromDir(filepath.Join(evolveDir, "profiles"))
}

func fakeDoctorAllBlocked(ctx context.Context) bridge.DoctorReport {
	return bridge.DoctorReport{
		Results: []bridge.DoctorResult{
			{CLI: "claude-tmux", Binary: bridge.BinaryInfo{Present: false}, Verdict: "blocked"},
			{CLI: "codex-tmux", Binary: bridge.BinaryInfo{Present: false}, Verdict: "blocked"},
		},
	}
}

func parsePins(t *testing.T, b []byte) map[string]policy.Pin {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(b, &obj); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	pins := map[string]policy.Pin{}
	if raw, ok := obj["pins"]; ok {
		if err := json.Unmarshal(raw, &pins); err != nil {
			t.Fatalf("output pins not valid: %v", err)
		}
	}
	return pins
}

func TestApply_UnknownPreset_Errors(t *testing.T) {
	rep, loader := applyFixture(t)
	out, err := Apply(rep, builtinPresets, "turbo", nil, loader)
	if err == nil {
		t.Fatal("unknown preset should error")
	}
	if out != nil {
		t.Error("no bytes should be returned on error")
	}
}

func TestApply_EmptyPolicy_WritesOnlyDifferingPins(t *testing.T) {
	rep, loader := applyFixture(t)
	out, err := Apply(rep, builtinPresets, "recommended", nil, loader)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	pins := parsePins(t, out)
	if _, ok := pins["builder"]; !ok {
		t.Error("builder differs from default (agy→claude) → should be pinned")
	}
	if _, ok := pins["auditor"]; !ok {
		t.Error("auditor differs from default (tier→deep) → should be pinned")
	}
	if _, ok := pins["scout"]; ok {
		t.Error("scout matches its profile default → must NOT be pinned")
	}
}

func TestApply_LosslessMerge_PreservesForeignKeys(t *testing.T) {
	rep, loader := applyFixture(t)
	existing := `{
	  "version": 1,
	  "floor": [{"id":"dossier-closeout"}],
	  "cli_health": {"proactive_probe": true}
	}`
	out, err := Apply(rep, builtinPresets, "recommended", []byte(existing), loader)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"version", "floor", "cli_health", "pins"} {
		if _, ok := obj[k]; !ok {
			t.Errorf("lossless merge dropped %q", k)
		}
	}
}

func TestApply_PreservesForeignPins(t *testing.T) {
	rep, loader := applyFixture(t)
	existing := `{"pins":{"deploy":{"cli":"claude","model":"deep"}}}`
	out, err := Apply(rep, builtinPresets, "recommended", []byte(existing), loader)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	pins := parsePins(t, out)
	if d, ok := pins["deploy"]; !ok || d.CLI != "claude" || d.Model != "deep" {
		t.Errorf("foreign pin 'deploy' should survive, got %+v (present=%v)", d, ok)
	}
}

func TestApply_ReapplyRecommendedClearsUpgrade(t *testing.T) {
	rep, loader := applyFixture(t)
	hi, err := Apply(rep, builtinPresets, "max-quality", nil, loader)
	if err != nil {
		t.Fatalf("apply max-quality: %v", err)
	}
	if _, ok := parsePins(t, hi)["scout"]; !ok {
		t.Fatal("precondition: max-quality should pin scout (upgraded to deep)")
	}
	lo, err := Apply(rep, builtinPresets, "recommended", hi, loader)
	if err != nil {
		t.Fatalf("re-apply recommended: %v", err)
	}
	if _, ok := parsePins(t, lo)["scout"]; ok {
		t.Error("re-applying recommended should clear the stale scout upgrade pin")
	}
	if b := parsePins(t, lo)["builder"]; b.Model != "balanced" {
		t.Errorf("builder tier after re-apply = %q, want balanced", b.Model)
	}
}

func TestApply_MalformedExisting_Refuses(t *testing.T) {
	rep, loader := applyFixture(t)
	out, err := Apply(rep, builtinPresets, "recommended", []byte(`{not json`), loader)
	if err == nil || out != nil {
		t.Errorf("malformed existing should refuse with no bytes; err=%v bytes=%v", err, out != nil)
	}
	if err != nil && !strings.Contains(err.Error(), "clobber") {
		t.Errorf("error should say it refuses to clobber, got %v", err)
	}
}

func TestApply_EmittedPinsPassValidatePin(t *testing.T) {
	rep, loader := applyFixture(t)
	out, err := Apply(rep, builtinPresets, "max-quality", nil, loader)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	for role, pin := range parsePins(t, out) {
		prof, perr := loader.Get(role)
		if perr != nil {
			t.Errorf("emitted pin for %q has no profile", role)
			continue
		}
		if verr := policy.ValidatePin(role, pin, &prof); verr != nil {
			t.Errorf("emitted pin %q=%+v breaches floor: %v", role, pin, verr)
		}
	}
}

func TestApply_DegradedPreset_Refuses(t *testing.T) {
	project, evolveDir := fixtureRepo(t)
	rep := Detect(context.Background(), DetectOptions{
		ProjectRoot: project, EvolveDir: evolveDir,
		Env: func(string) string { return "" }, Now: func() time.Time { return time.Unix(0, 0).UTC() },
		Doctor: fakeDoctorAllBlocked, CapTier: func(string) string { return "n/a" },
	})
	loader := profiles.NewFromDir(filepath.Join(evolveDir, "profiles"))
	out, err := Apply(rep, builtinPresets, "recommended", nil, loader)
	if err == nil || out != nil {
		t.Errorf("degraded preset should refuse; err=%v bytes=%v", err, out != nil)
	}
}

func TestApply_PinStoresTierNotNativeModel(t *testing.T) {
	rep, loader := applyFixture(t)
	out, err := Apply(rep, builtinPresets, "recommended", nil, loader)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	a := parsePins(t, out)["auditor"]
	if a.Model != "deep" {
		t.Errorf("auditor pin model = %q, want the tier 'deep' (not a native model id)", a.Model)
	}
}

func TestApply_CrossFamilyPinsLegal(t *testing.T) {
	rep, loader := applyFixture(t)
	out, err := Apply(rep, builtinPresets, "recommended", nil, loader)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	pins := parsePins(t, out)
	if pins["builder"].CLI == pins["auditor"].CLI {
		t.Errorf("builder/auditor should be cross-family, both %q", pins["builder"].CLI)
	}
}

func TestApply_ADeclaredTableRefusesEveryPreset(t *testing.T) {
	rep, loader := applyFixture(t)
	rep.RoutingTableDeclared = true
	existing := []byte(`{"cli_routing": {"clis": ["agy", "claude"], "default": ["agy", "claude"]}}`)

	out, err := Apply(rep, builtinPresets, "recommended", existing, loader)

	if out != nil || !errors.Is(err, ErrRoutingTableDeclared) || !strings.Contains(err.Error(), "cli-routing set agents.<role>") {
		t.Fatalf("out=%s err=%v: a declared table owns every route, so a preset is refused with the pointer to the table's own writer", out, err)
	}
}

func TestApply_ATablePolicyThatFailsToLoadIsStillRefused(t *testing.T) {
	project, evolveDir := fixtureRepo(t)
	existing := []byte(`{"cli_routing": {"clis": ["claude"], "default": ["claude"]}, "fleet": {"count": "three"}}`)
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), existing, 0o644); err != nil {
		t.Fatal(err)
	}
	rep := Detect(context.Background(), DetectOptions{ProjectRoot: project, EvolveDir: evolveDir, Env: func(string) string { return "" }, Doctor: fakeDoctor, CapTier: func(string) string { return "full" }})

	out, err := Apply(rep, builtinPresets, "recommended", existing, profiles.NewFromDir(filepath.Join(evolveDir, "profiles")))

	if rep.PolicyError == "" || !rep.RoutingTableDeclared {
		t.Errorf("policy_error %q, routing_table_declared %v: detect reads the cli_routing key from the raw file even when the policy does not load", rep.PolicyError, rep.RoutingTableDeclared)
	}
	if out != nil || !errors.Is(err, ErrRoutingTableDeclared) {
		t.Fatalf("out=%s err=%v: apply never writes pins beside a cli_routing key, whatever else in the file fails to load", out, err)
	}
}

func TestApply_TheFileItPatchesDecidesTheRefusalNotTheReport(t *testing.T) {
	for _, key := range []string{"cli_routing", "CLI_Routing"} {
		t.Run(key, func(t *testing.T) {
			rep, loader := applyFixture(t)
			rep.RoutingTableDeclared = false
			existing := []byte(`{"` + key + `": {"clis": ["claude"], "default": ["claude"]}}`)
			if p, err := policy.Parse(existing); err != nil || p.CLIRouting == nil {
				t.Fatalf("policy.Parse(%s) = (%+v, %v): the policy loader reads this key as a live routing table", existing, p, err)
			}

			out, err := Apply(rep, builtinPresets, "recommended", existing, loader)

			if out != nil || !errors.Is(err, ErrRoutingTableDeclared) {
				t.Fatalf("out=%s err=%v: a %s key the policy loader reads as the routing table refuses the preset", out, err, key)
			}
		})
	}
}

func TestDetect_ReportsWhetherTheRoutingTableIsDeclared(t *testing.T) {
	for name, tc := range map[string]struct {
		policy string
		want   bool
	}{
		"a declared table":            {`{"cli_routing": {"clis": ["claude"], "default": ["claude"]}}`, true},
		"a table key in another case": {`{"Cli_Routing": {"clis": ["claude"], "default": ["claude"]}}`, true},
		"pins only":                   {`{"pins": {"builder": {"cli": "claude-tmux"}}}`, false},
		"a malformed policy":          {`{"cli_routing": `, false},
		"no policy file":              {"", false},
	} {
		project, evolveDir := fixtureRepo(t)
		policyPath := filepath.Join(evolveDir, "policy.json")
		if err := os.Remove(policyPath); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if tc.policy != "" {
			if err := os.WriteFile(policyPath, []byte(tc.policy), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		rep := Detect(context.Background(), DetectOptions{ProjectRoot: project, EvolveDir: evolveDir, Env: func(string) string { return "" }, Doctor: fakeDoctor, CapTier: func(string) string { return "full" }})
		if rep.RoutingTableDeclared != tc.want {
			t.Errorf("%s: RoutingTableDeclared = %v, want %v", name, rep.RoutingTableDeclared, tc.want)
		}
	}
}
