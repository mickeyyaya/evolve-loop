//go:build acs

package cycle2139

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func isolateCatalog(t *testing.T) {
	t.Helper()
	empty := t.TempDir()
	bridge.SetModelCatalogDirFn(func() string { return empty })
}

func agyManifest(t *testing.T, cli string) bridge.Manifest {
	t.Helper()
	isolateCatalog(t)
	m, err := bridge.LoadManifest(cli)
	if err != nil {
		t.Fatalf("LoadManifest(%s): %v", cli, err)
	}
	if m.ModelTierMap["fast"] == "" {
		t.Fatalf("%s declares no fast model", cli)
	}
	return m
}

func modelValue(flags []string) (string, bool) {
	i := slices.Index(flags, "--model")
	if i < 0 || i+1 >= len(flags) {
		return "", false
	}
	return flags[i+1], true
}

func requireModel(t *testing.T, what string, flags []string, want string) {
	t.Helper()
	got, ok := modelValue(flags)
	if !ok {
		t.Errorf("%s: launch flags %v carry no --model, want %q", what, flags, want)
		return
	}
	if got != want {
		t.Errorf("%s: --model %q, want %q", what, got, want)
	}
}

func goTestBridgeRun(t *testing.T, run string) {
	t.Helper()
	moduleDir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "-count=1", "-run", run, "./internal/bridge")
	cmd.Dir = moduleDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("go test ./internal/bridge -run %s fails: %v\n%s", run, err, acsassert.OutputTail(string(out), 25))
	}
}

func TestC2139_001_DispatchWithNoTierCarriesTheAgyFastModel(t *testing.T) {
	for _, cli := range []string{"agy-tmux", "agy-claude-tmux"} {
		m := agyManifest(t, cli)
		r := bridge.Realize(m, bridge.LaunchIntent{ModelTier: "auto"})
		requireModel(t, cli+" tier auto", r.LaunchFlags, m.ModelTierMap["fast"])
	}
}

func TestC2139_002_ProductionLaunchLineOfAnUntieredDispatchCarriesTheFastModel(t *testing.T) {
	goTestBridgeRun(t, "^TestAgyEveryLaunch_ProductionLaunchLineCarriesTheManifestFastModel$")
}

func TestC2139_003_UnresolvedTierTokenCarriesTheAgyFastModel(t *testing.T) {
	m := agyManifest(t, "agy-tmux")
	tiers := maps.Clone(m.ModelTierMap)
	delete(tiers, "deep")
	delete(tiers, "top")
	m.ModelTierMap = tiers
	for _, tier := range []string{"deep", "top"} {
		r := bridge.Realize(m, bridge.LaunchIntent{ModelTier: tier})
		requireModel(t, "unresolved tier "+tier, r.LaunchFlags, tiers["fast"])
	}
}

func TestC2139_004_FallbackModelIsReadFromTheManifestMapNotALiteral(t *testing.T) {
	const syntheticFast = "Synthetic Fast (Test)"
	m := agyManifest(t, "agy-tmux")
	tiers := maps.Clone(m.ModelTierMap)
	tiers["fast"] = syntheticFast
	m.ModelTierMap = tiers
	r := bridge.Realize(m, bridge.LaunchIntent{ModelTier: "auto"})
	requireModel(t, "synthetic manifest tier auto", r.LaunchFlags, syntheticFast)
}

func TestC2139_005_ResolvedTiersAndRawModelsAreUnchanged(t *testing.T) {
	m := agyManifest(t, "agy-tmux")
	for _, tier := range []string{"balanced", "deep", "top"} {
		r := bridge.Realize(m, bridge.LaunchIntent{ModelTier: tier})
		requireModel(t, "resolved tier "+tier, r.LaunchFlags, m.ModelTierMap[tier])
	}
	r := bridge.Realize(m, bridge.LaunchIntent{ModelTier: "Gemini 3.1 Pro (High)"})
	requireModel(t, "raw model", r.LaunchFlags, "Gemini 3.1 Pro (High)")
}

func TestC2139_006_OtherCLIsStillOmitTheModelForAnUnresolvedToken(t *testing.T) {
	isolateCatalog(t)
	for _, cli := range []string{"claude-tmux", "codex-tmux"} {
		r := bridge.RealizeFor(cli, bridge.LaunchIntent{ModelTier: "auto"})
		if slices.Contains(r.LaunchFlags, "--model") || slices.Contains(r.LaunchFlags, "auto") {
			t.Errorf("%s: tier auto must still omit the model, flags = %v", cli, r.LaunchFlags)
		}
	}
}

func TestC2139_007_DoctorProbeCarriesTheAgyFastModel(t *testing.T) {
	m := agyManifest(t, "agy")
	d, ok := bridge.LookupDriver("agy")
	if !ok {
		t.Fatal("agy is not a registered driver")
	}
	prober, ok := d.(bridge.DeepProber)
	if !ok {
		t.Fatal("agy driver does not implement DeepProber")
	}
	requireModel(t, "agy -p probe", prober.DeepProbeArgs(m), m.ModelTierMap["fast"])
}

func TestC2139_008_NoNonTestGoLiteralNamesAnAgyModel(t *testing.T) {
	moduleDir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
	if err != nil {
		t.Fatal(err)
	}
	files, err := acsassert.WalkModule(moduleDir, func(rel string) bool {
		return strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go") && !strings.HasPrefix(rel, "acs/")
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 100 {
		t.Fatalf("scanned %d Go files, want the whole module (guard would be vacuous)", len(files))
	}
	agyModel := regexp.MustCompile(`(?i)gemini[ -]?[0-9]|argon \(|\b(flash|pro|sonnet|opus) [0-9.]*\s*\((low|medium|high|thinking)\)`)
	fset := token.NewFileSet()
	for _, rel := range files {
		f, err := parser.ParseFile(fset, filepath.Join(moduleDir, rel), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			val, err := strconv.Unquote(lit.Value)
			if err == nil && agyModel.MatchString(val) {
				t.Errorf("%s:%d names an agy model in a Go literal: %q", rel, fset.Position(lit.Pos()).Line, val)
			}
			return true
		})
	}
}

func TestC2139_009_TheAutoTierTestPinsTheFastModelAndTheEmptyTierTestPinsNoModel(t *testing.T) {
	goTestBridgeRun(t, "^(TestAgyModelTierAutoSentinelCarriesFastModel|TestAgyModelTierUnknownTierPassthrough)$")
}

func TestC2139_010_AnEmptyTierKeepsTheAgyOwnDefaultModel(t *testing.T) {
	m := agyManifest(t, "agy-tmux")
	for what, flags := range map[string][]string{
		"Realize":    bridge.Realize(m, bridge.LaunchIntent{ModelTier: ""}).LaunchFlags,
		"RealizeFor": bridge.RealizeFor("agy-tmux", bridge.LaunchIntent{ModelTier: ""}).LaunchFlags,
	} {
		if got, ok := modelValue(flags); ok {
			t.Errorf("%s: an empty tier is a model-less launch (the CLI-health canary), yet flags carry --model %q", what, got)
		}
	}
}

func TestC2139_011_TheCanaryStillLaunchesAgyOnItsOwnDefaultModel(t *testing.T) {
	moduleDir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "-count=1", "-run", "^TestDefaultLiveProbe_TheCanaryLaunchesTheCLIsOwnDefaultModelAndNeverAChosenTier$", "./cmd/evolve")
	cmd.Dir = moduleDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("go test ./cmd/evolve canary contract fails: %v\n%s", err, acsassert.OutputTail(string(out), 25))
	}
}

func modelTierSpecJSON(t *testing.T, m bridge.Manifest) map[string]any {
	t.Helper()
	raw, err := json.Marshal(m.Params["model_tier"])
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	return fields
}

func withModelTierSpec(t *testing.T, m bridge.Manifest, fields map[string]any) bridge.Manifest {
	t.Helper()
	raw, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	var spec bridge.ParamSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	params := maps.Clone(m.Params)
	params["model_tier"] = spec
	m.Params = params
	return m
}

func TestC2139_012_TheUnresolvedFallbackIsManifestDataAndNotAgySpecific(t *testing.T) {
	m := agyManifest(t, "agy-tmux")
	m.CLI, m.Binary = "plain-test-cli", "plainbin"
	m.ModelTierMap = map[string]string{"fast": "Plain Fast Model"}
	m = withModelTierSpec(t, m, map[string]any{
		"channel": "flag", "flag": "--model", "from": "model_tier_map", "unresolved": "fast",
	})
	r := bridge.Realize(m, bridge.LaunchIntent{ModelTier: "auto"})
	requireModel(t, "non-agy manifest declaring unresolved=fast", r.LaunchFlags, "Plain Fast Model")
}

func TestC2139_013_AnAgyManifestWithoutTheDataStillOmitsTheModel(t *testing.T) {
	m := agyManifest(t, "agy-tmux")
	fields := modelTierSpecJSON(t, m)
	if _, declared := fields["unresolved"]; !declared {
		t.Fatalf("agy-tmux model_tier declares no unresolved fallback in manifest data: %v", fields)
	}
	delete(fields, "unresolved")
	delete(fields, "default")
	m = withModelTierSpec(t, m, fields)
	for _, tier := range []string{"", "auto"} {
		r := bridge.Realize(m, bridge.LaunchIntent{ModelTier: tier})
		if got, ok := modelValue(r.LaunchFlags); ok {
			t.Errorf("tier %q: an agy manifest with no fallback data must not invent --model %q from its binary name", tier, got)
		}
	}
}
