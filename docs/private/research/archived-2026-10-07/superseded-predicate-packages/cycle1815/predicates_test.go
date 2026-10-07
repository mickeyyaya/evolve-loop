//go:build acs

package cycle1815

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
	"github.com/mickeyyaya/evolve-loop/go/internal/setup"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const workflowBlockSettingEverySliceAndMap = `{"workflow":{
  "phase_enables":{"tdd":"on","memo":"off"},
  "remediable_phases":["coverage-gate","build-floor"],
  "universal_fallback_exclude":["agy","ollama"],
  "interactive_policies":{"scout":"recommended_or_first"},
  "size_budget_multipliers":{"medium":2}
}}`

func loadPolicyFromFile(t *testing.T, body string) policy.Policy {
	t.Helper()
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write policy fixture: %v", err)
	}
	pol, err := policy.Load(path)
	if err != nil {
		t.Fatalf("policy.Load(%s): %v", path, err)
	}
	return pol
}

func jsonSnapshot(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("snapshot %T: %v", v, err)
	}
	return string(raw)
}

func mutateEverySliceAndMapField(cfg *policy.WorkflowConfig) []string {
	var mutated []string
	v := reflect.ValueOf(cfg).Elem()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		name := v.Type().Field(i).Name
		switch field.Kind() {
		case reflect.Slice:
			if field.Len() == 0 {
				continue
			}
			field.Index(0).Set(reflect.Zero(field.Type().Elem()))
			mutated = append(mutated, name)
		case reflect.Map:
			if field.Len() == 0 {
				continue
			}
			for _, key := range field.MapKeys() {
				field.SetMapIndex(key, reflect.Zero(field.Type().Elem()))
			}
			field.SetMapIndex(reflect.ValueOf("acs-injected-key"), reflect.Zero(field.Type().Elem()))
			mutated = append(mutated, name)
		}
	}
	return mutated
}

func TestC1815_001_MutatingResolvedPhaseEnablesLeavesTheNextResolveUnchanged(t *testing.T) {
	pol := loadPolicyFromFile(t, workflowBlockSettingEverySliceAndMap)
	want := map[string]string{"tdd": "on", "memo": "off"}

	first := pol.WorkflowConfig()
	if !reflect.DeepEqual(first.PhaseEnables, want) {
		t.Fatalf("fixture phase_enables did not resolve: got %v, want %v", first.PhaseEnables, want)
	}
	first.PhaseEnables["tdd"] = "mutated-by-caller"
	first.PhaseEnables["acs-injected-key"] = "on"

	second := pol.WorkflowConfig()
	if !reflect.DeepEqual(second.PhaseEnables, want) {
		t.Errorf("RED: a caller's write to WorkflowConfig().PhaseEnables leaked into the next resolve: got %v, want %v", second.PhaseEnables, want)
	}
	if !reflect.DeepEqual(pol.Workflow.PhaseEnables, want) {
		t.Errorf("RED: a caller's write to WorkflowConfig().PhaseEnables altered the loaded policy: got %v, want %v", pol.Workflow.PhaseEnables, want)
	}
}

func TestC1815_002_MutatingResolvedRemediablePhasesLeavesTheNextResolveUnchanged(t *testing.T) {
	pol := loadPolicyFromFile(t, workflowBlockSettingEverySliceAndMap)
	want := []string{"coverage-gate", "build-floor"}

	first := pol.WorkflowConfig()
	if !reflect.DeepEqual(first.RemediablePhases, want) {
		t.Fatalf("fixture remediable_phases did not resolve: got %v, want %v", first.RemediablePhases, want)
	}
	first.RemediablePhases[0] = "audit"

	second := pol.WorkflowConfig()
	if !reflect.DeepEqual(second.RemediablePhases, want) {
		t.Errorf("RED: a caller's write to WorkflowConfig().RemediablePhases leaked into the next resolve: got %v, want %v", second.RemediablePhases, want)
	}
	if !reflect.DeepEqual(pol.Workflow.RemediablePhases, want) {
		t.Errorf("RED: a caller's write to WorkflowConfig().RemediablePhases altered the loaded policy: got %v, want %v", pol.Workflow.RemediablePhases, want)
	}
}

func TestC1815_003_MutatingEverySliceAndMapOfWorkflowConfigLeavesTheNextResolveUnchanged(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{"policy sets every slice and map", workflowBlockSettingEverySliceAndMap},
		{"absent workflow block", `{}`},
		{"empty workflow block", `{"workflow":{}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pol := loadPolicyFromFile(t, tc.body)
			resolvedBefore := jsonSnapshot(t, pol.WorkflowConfig())
			loadedBefore := jsonSnapshot(t, pol.Workflow)

			victim := pol.WorkflowConfig()
			mutated := mutateEverySliceAndMapField(&victim)
			if len(mutated) == 0 {
				t.Fatalf("the sweep mutated no field; the probe is vacuous")
			}

			if after := jsonSnapshot(t, pol.WorkflowConfig()); after != resolvedBefore {
				t.Errorf("RED: mutating %v of one WorkflowConfig() changed the next resolve:\nbefore %s\nafter  %s", mutated, resolvedBefore, after)
			}
			if after := jsonSnapshot(t, pol.Workflow); after != loadedBefore {
				t.Errorf("RED: mutating %v of one WorkflowConfig() changed the loaded policy:\nbefore %s\nafter  %s", mutated, loadedBefore, after)
			}
		})
	}

	probe := loadPolicyFromFile(t, workflowBlockSettingEverySliceAndMap).WorkflowConfig()
	sweptFields := mutateEverySliceAndMapField(&probe)
	for _, required := range []string{"PhaseEnables", "RemediablePhases"} {
		if !slices.Contains(sweptFields, required) {
			t.Errorf("the sweep never probed %s (swept %v); the fixture no longer exercises the aliased field", required, sweptFields)
		}
	}
}

func TestC1815_004_NegativeChronicleOverridesResolveToTheDefaults(t *testing.T) {
	defaults := policy.Policy{}.ChronicleConfig()
	if defaults.DigestTokens <= 0 || defaults.DigestCycles <= 0 {
		t.Fatalf("compiled chronicle defaults are not positive: %+v", defaults)
	}

	for _, tc := range []struct {
		name string
		body string
	}{
		{"minus one", `{"chronicle":{"digest_tokens":-1,"digest_cycles":-1}}`},
		{"large negative", `{"chronicle":{"digest_tokens":-1200,"digest_cycles":-100000}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := loadPolicyFromFile(t, tc.body).ChronicleConfig()
			if got.DigestTokens != defaults.DigestTokens {
				t.Errorf("RED: negative digest_tokens resolved to %d, want the default %d", got.DigestTokens, defaults.DigestTokens)
			}
			if got.DigestCycles != defaults.DigestCycles {
				t.Errorf("RED: negative digest_cycles resolved to %d, want the default %d", got.DigestCycles, defaults.DigestCycles)
			}
		})
	}

	positive := loadPolicyFromFile(t, `{"chronicle":{"digest_tokens":300,"digest_cycles":4}}`).ChronicleConfig()
	if positive.DigestTokens != 300 || positive.DigestCycles != 4 {
		t.Errorf("a positive chronicle override must still win: got tokens=%d cycles=%d, want 300 and 4", positive.DigestTokens, positive.DigestCycles)
	}
	zero := loadPolicyFromFile(t, `{"chronicle":{"digest_tokens":0,"digest_cycles":0}}`).ChronicleConfig()
	if zero.DigestTokens != defaults.DigestTokens || zero.DigestCycles != defaults.DigestCycles {
		t.Errorf("a zero chronicle override must resolve to the defaults: got %+v, want %+v", zero, defaults)
	}
}

func TestC1815_005_NegativeFailureDispositionOverridesResolveToTheEscalationDefaults(t *testing.T) {
	escalationDefaults := recurrence.DefaultEscalationPolicy()
	defaults := policy.Policy{}.FailureDispositionConfig()
	if defaults.Threshold != escalationDefaults.Threshold || defaults.Step != escalationDefaults.Step || defaults.Cap != escalationDefaults.Cap {
		t.Fatalf("policy defaults %+v drifted from recurrence.DefaultEscalationPolicy %+v", defaults, escalationDefaults)
	}

	for _, tc := range []struct {
		name string
		body string
	}{
		{"every knob negative", `{"failure_disposition":{"stage":"shadow","threshold":-1,"step":-0.03,"cap":-0.5}}`},
		{"large negatives", `{"failure_disposition":{"threshold":-100000,"step":-1,"cap":-99}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := loadPolicyFromFile(t, tc.body).FailureDispositionConfig()
			if got.Threshold != escalationDefaults.Threshold {
				t.Errorf("RED: negative threshold resolved to %d, want the default %d", got.Threshold, escalationDefaults.Threshold)
			}
			if got.Step != escalationDefaults.Step {
				t.Errorf("RED: negative step resolved to %v, want the default %v", got.Step, escalationDefaults.Step)
			}
			if got.Cap != escalationDefaults.Cap {
				t.Errorf("RED: negative cap resolved to %v, want the default %v", got.Cap, escalationDefaults.Cap)
			}
			applied := recurrence.EscalationPolicy{Threshold: got.Threshold, Step: got.Step, Cap: got.Cap}
			if w, want := applied.Target(0.5, 3), escalationDefaults.Target(0.5, 3); w != want {
				t.Errorf("RED: the escalation boundary would weight a 3x recurrence at %v under the negative override, want %v", w, want)
			}
		})
	}

	positive := loadPolicyFromFile(t, `{"failure_disposition":{"threshold":4,"step":0.05,"cap":0.9}}`).FailureDispositionConfig()
	if positive.Threshold != 4 || positive.Step != 0.05 || positive.Cap != 0.9 {
		t.Errorf("a positive failure_disposition override must still win: got %+v", positive)
	}
}

func readyCLIs(families ...string) []setup.CLIStatus {
	out := make([]setup.CLIStatus, 0, len(families))
	for _, f := range families {
		out = append(out, setup.CLIStatus{CLI: f, BinaryPresent: true, AuthConfigured: true, Verdict: "ready"})
	}
	return out
}

func assignmentFor(t *testing.T, rr setup.RecommendReport, role string) setup.Assignment {
	t.Helper()
	if len(rr.Presets) == 0 {
		t.Fatalf("Recommend returned no presets")
	}
	for _, a := range rr.Presets[0].Assignments {
		if a.Role == role {
			return a
		}
	}
	t.Fatalf("no %s assignment in %+v", role, rr.Presets[0].Assignments)
	return setup.Assignment{}
}

func TestC1815_006_SetupRecommendNormalizesDriverSuffixesLikePolicyBaseCLI(t *testing.T) {
	const chainedDriverName = "codex-tmux-p"
	if got := policy.BaseCLI(chainedDriverName); got != "codex" {
		t.Fatalf("policy.BaseCLI(%q) = %q, want codex", chainedDriverName, got)
	}

	cfg := setup.PresetConfig{Default: "acs", Presets: []setup.PresetSpec{{Name: "acs"}}}
	rep := setup.DetectReport{
		CLIs: readyCLIs("claude", "codex"),
		Phases: []setup.PhaseStatus{
			{Role: "scout", DefaultCLI: chainedDriverName, AllowedCLIs: []string{"claude", "codex"}},
			{Role: "memo", DefaultCLI: "claude", AllowedCLIs: []string{chainedDriverName}},
			{Role: "builder", DefaultCLI: chainedDriverName, AllowedCLIs: []string{"claude", "codex"}},
			{Role: "auditor", DefaultCLI: "claude-tmux-p", AllowedCLIs: []string{"claude", "codex"}},
		},
	}
	rr := setup.Recommend(rep, cfg)

	scout := assignmentFor(t, rr, "scout")
	if scout.CLI != "codex" || scout.CLIFallback {
		t.Errorf("RED: scout default %q must resolve to its own family codex without fallback; got cli=%q fallback=%v", chainedDriverName, scout.CLI, scout.CLIFallback)
	}
	memo := assignmentFor(t, rr, "memo")
	if memo.CLI != "codex" || memo.Warning != "" {
		t.Errorf("RED: allowed_clis [%q] must admit the codex family; got cli=%q warning=%q", chainedDriverName, memo.CLI, memo.Warning)
	}
	builder := assignmentFor(t, rr, "builder")
	auditor := assignmentFor(t, rr, "auditor")
	if builder.CLI != "codex" || builder.CLIFallback || auditor.CLI != "claude" || auditor.CLIFallback {
		t.Errorf("RED: the cross-family pair must keep each default family (builder codex, auditor claude); got builder=%q/%v auditor=%q/%v",
			builder.CLI, builder.CLIFallback, auditor.CLI, auditor.CLIFallback)
	}
}

func TestC1815_007_SetupDetectCollapsesDoctorRowsToThePolicyBaseCLIFamily(t *testing.T) {
	project := t.TempDir()
	evolveDir := filepath.Join(project, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", evolveDir, err)
	}
	doctorWithChainedDriverRows := func(context.Context) bridge.DoctorReport {
		return bridge.DoctorReport{Results: []bridge.DoctorResult{
			{CLI: "codex-tmux", Binary: bridge.BinaryInfo{Present: true, Path: "/usr/local/bin/codex"}, Auth: bridge.AuthInfo{Configured: true}, Verdict: "ready"},
			{CLI: "codex-tmux-p", Binary: bridge.BinaryInfo{Present: true, Path: "/usr/local/bin/codex"}, Auth: bridge.AuthInfo{Configured: true}, Verdict: "ready"},
			{CLI: "claude-tmux-p", Binary: bridge.BinaryInfo{Present: true, Path: "/usr/local/bin/claude"}, Auth: bridge.AuthInfo{Configured: true}, Verdict: "ready"},
		}}
	}
	rep := setup.Detect(context.Background(), setup.DetectOptions{
		ProjectRoot: project,
		EvolveDir:   evolveDir,
		Env:         func(string) string { return "" },
		Now:         func() time.Time { return time.Unix(0, 0).UTC() },
		Doctor:      doctorWithChainedDriverRows,
		CapTier:     func(string) string { return "full" },
	})

	var families []string
	for _, c := range rep.CLIs {
		families = append(families, c.CLI)
	}
	want := []string{"claude", "codex"}
	if !reflect.DeepEqual(families, want) {
		t.Errorf("RED: Detect must report one row per policy.BaseCLI family; got %v, want %v", families, want)
	}
}

// acs-predicate: source-structure
func TestC1815_008_SetupDeclaresNoBaseCLIOfItsOwn(t *testing.T) {
	root := acsassert.RepoRoot(t)
	setupDir := filepath.Join(root, "go", "internal", "setup")
	entries, err := os.ReadDir(setupDir)
	if err != nil {
		t.Fatalf("read %s: %v", setupDir, err)
	}
	fset := token.NewFileSet()
	parsed := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(setupDir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		parsed++
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && strings.EqualFold(fn.Name.Name, "baseCLI") {
				t.Errorf("RED: %s declares %s at %s; setup must call policy.BaseCLI", name, fn.Name.Name, fset.Position(fn.Pos()))
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if ok && lit.Kind == token.STRING && (lit.Value == `"-tmux"` || lit.Value == `"-p"`) {
				t.Errorf("RED: %s strips the driver suffix %s itself at %s; the only implementation is profiles.BaseCLI behind policy.BaseCLI", name, lit.Value, fset.Position(lit.Pos()))
			}
			return true
		})
	}
	if parsed == 0 {
		t.Fatalf("no non-test Go files parsed under %s", setupDir)
	}
}

// acs-predicate: source-structure
func TestC1815_009_WorkflowConfigAssignsEachRemediationKnobOnce(t *testing.T) {
	root := acsassert.RepoRoot(t)
	workflowSource := filepath.Join(root, "go", "internal", "policy", "workflow.go")
	for _, guard := range []string{
		"p.Workflow.RemediationRounds != nil",
		"p.Workflow.RemediablePhases != nil",
		"p.Workflow.BuildFloor != nil",
	} {
		n, err := acsassert.CountInGoFunc(workflowSource, "WorkflowConfig", guard)
		if err != nil {
			t.Fatalf("count %q in WorkflowConfig: %v", guard, err)
		}
		if n != 1 {
			t.Errorf("RED: WorkflowConfig checks %q %d times, want exactly 1 (the duplicate tail block must go)", guard, n)
		}
	}

	pol := loadPolicyFromFile(t, `{"workflow":{"remediation_rounds":0,"remediable_phases":[],"build_floor":false}}`)
	got := pol.WorkflowConfig()
	if got.RemediationRounds != 0 || len(got.RemediablePhases) != 0 || got.BuildFloorEnforced {
		t.Errorf("explicit remediation_rounds=0, remediable_phases=[], build_floor=false must each win once: got rounds=%d phases=%v floor=%v",
			got.RemediationRounds, got.RemediablePhases, got.BuildFloorEnforced)
	}
	defaults := policy.Policy{}.WorkflowConfig()
	if defaults.RemediationRounds != 1 || !reflect.DeepEqual(defaults.RemediablePhases, []string{"coverage-gate"}) || !defaults.BuildFloorEnforced {
		t.Errorf("compiled remediation defaults changed: rounds=%d phases=%v floor=%v", defaults.RemediationRounds, defaults.RemediablePhases, defaults.BuildFloorEnforced)
	}
}

// acs-predicate: config-check
func TestC1815_010_EveryADRNumberNamesExactlyOneFile(t *testing.T) {
	root := acsassert.RepoRoot(t)
	adrDir := filepath.Join(root, "docs", "architecture", "adr")
	entries, err := os.ReadDir(adrDir)
	if err != nil {
		t.Fatalf("read %s: %v", adrDir, err)
	}
	adrNumber := regexp.MustCompile(`^(\d{4})-.+\.md$`)
	byNumber := map[string][]string{}
	for _, e := range entries {
		if m := adrNumber.FindStringSubmatch(e.Name()); m != nil {
			byNumber[m[1]] = append(byNumber[m[1]], e.Name())
		}
	}
	if len(byNumber) == 0 {
		t.Fatalf("no NNNN-*.md ADR files under %s", adrDir)
	}
	for number, files := range byNumber {
		if len(files) > 1 {
			t.Errorf("ADR-%s names %d files %v; a `See ADR-%s.` pointer is ambiguous", number, len(files), files, number)
		}
	}
}

func TestC1815_011_PolicyPackageCarriesTheMutationAndNegativeOverrideUnitTests(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goModuleDir := filepath.Join(root, "go")
	required := []string{
		"TestWorkflowConfig_MutatingResultDoesNotChangeNextResolve",
		"TestChronicleConfig_NegativeOverridesResolveToDefaults",
		"TestFailureDispositionConfig_NegativeOverridesResolveToDefaults",
	}
	exact := "^(" + strings.Join(required, "|") + ")$"

	listOut, listErr, code, err := acsassert.SubprocessOutput("go", "-C", goModuleDir, "test", "-count=1", "-list", exact, "./internal/policy")
	if err != nil || code != 0 {
		t.Fatalf("go test -list ./internal/policy exit=%d err=%v\n%s%s", code, err, listOut, listErr)
	}
	listed := map[string]bool{}
	for _, line := range strings.Split(listOut, "\n") {
		listed[strings.TrimSpace(line)] = true
	}
	var missing []string
	for _, name := range required {
		if !listed[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("RED: internal/policy lacks the permanent regression tests %v", missing)
	}

	runOut, runErr, code, err := acsassert.SubprocessOutput("go", "-C", goModuleDir, "test", "-count=1", "-run", exact, "./internal/policy")
	if err != nil || code != 0 {
		t.Errorf("RED: the regression tests fail: exit=%d err=%v\n%s%s", code, err, runOut, runErr)
	}
}
