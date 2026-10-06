//go:build acs

package cycle1812

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/phasecmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const registryRel = "docs/architecture/phase-registry.json"

const declaredRoot = "custom-phases"

const declaredPhase = "fixture-sweep"

var warnCodeRE = regexp.MustCompile(`\b[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+\b`)

var failedSubtestRE = regexp.MustCompile(`--- FAIL: (TestValidateUserSpec/\S+)`)

type cliRun struct {
	code   int
	stdout string
	stderr string
}

func (r cliRun) String() string {
	return "exit=" + strconv.Itoa(r.code) + "\n--- stdout ---\n" + r.stdout + "\n--- stderr ---\n" + r.stderr
}

func newPhaseProject(t *testing.T, policyBody *string) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	registry, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(registryRel)))
	if err != nil {
		t.Fatalf("fixture: read the repo phase registry: %v", err)
	}
	project := t.TempDir()
	writeFile(t, filepath.Join(project, filepath.FromSlash(registryRel)), registry)
	writeFile(t, filepath.Join(project, declaredRoot, declaredPhase, "phase.json"),
		[]byte(`{"name":"`+declaredPhase+`","optional":true,"kind":"llm"}`))
	if policyBody != nil {
		writeFile(t, filepath.Join(project, ".evolve", "policy.json"), []byte(*policyBody))
	}
	return project
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("fixture: mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("fixture: write %s: %v", path, err)
	}
}

func runPhasesList(t *testing.T, project string) cliRun {
	t.Helper()
	t.Setenv("EVOLVE_PROJECT_ROOT", project)
	var stdout, stderr bytes.Buffer
	code := phasecmd.RunPhases([]string{"list"}, strings.NewReader(""), &stdout, &stderr)
	return cliRun{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func policyWarnLines(r cliRun) []string {
	var hits []string
	for _, line := range strings.Split(r.stdout+"\n"+r.stderr, "\n") {
		if strings.Contains(line, "policy.json") {
			hits = append(hits, line)
		}
	}
	return hits
}

func TestC1812_001_MalformedPolicyYieldsCodedWarnOnPhasesList(t *testing.T) {
	malformedPolicyDeclaringRoots := `{"paths": {"phase_roots": "` + declaredRoot + `"},`
	project := newPhaseProject(t, &malformedPolicyDeclaringRoots)

	r := runPhasesList(t, project)

	if r.code != 0 {
		t.Fatalf("evolve phases list must still list the catalog (exit 0) when policy.json is malformed — a load failure would make the preflight consumer fail open and hide the problem again\n%s", r)
	}
	if !strings.Contains(r.stdout, "builtin") {
		t.Errorf("evolve phases list printed no built-in phases; the built-in catalog must survive a malformed policy.json\n%s", r)
	}
	var coded []string
	for _, line := range policyWarnLines(r) {
		withoutFixturePath := strings.ReplaceAll(line, project, "")
		if strings.Contains(withoutFixturePath, "WARN") && warnCodeRE.MatchString(withoutFixturePath) {
			coded = append(coded, line)
		}
	}
	if len(coded) == 0 {
		t.Errorf("a malformed .evolve/policy.json silently fell back to the default phase root: evolve phases list emitted no coded WARN line (an UPPER_SNAKE code plus the policy.json path), so the operator-declared root %q vanished without notice\n%s", declaredRoot, r)
	}
}

func TestC1812_002_MissingOrValidPolicyYieldsNoPolicyWarn(t *testing.T) {
	validPolicyDeclaringRoots := `{"paths": {"phase_roots": "` + declaredRoot + `"}}`
	cases := []struct {
		name            string
		policy          *string
		wantUserPhase   bool
		wantDeclaredDir bool
	}{
		{name: "missing policy.json", policy: nil, wantUserPhase: false},
		{name: "valid policy.json declaring a root", policy: &validPolicyDeclaringRoots, wantUserPhase: true, wantDeclaredDir: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			project := newPhaseProject(t, tc.policy)

			r := runPhasesList(t, project)

			if r.code != 0 {
				t.Fatalf("evolve phases list exit %d, want 0\n%s", r.code, r)
			}
			if hits := policyWarnLines(r); len(hits) != 0 {
				t.Errorf("%s must not produce a policy warning; got %q\n%s", tc.name, hits, r)
			}
			if got := strings.Contains(r.stdout, declaredPhase); got != tc.wantUserPhase {
				t.Errorf("user phase %s listed=%t, want %t (the declared root %q is honored only when policy.json declares it)\n%s",
					declaredPhase, got, tc.wantUserPhase, declaredRoot, r)
			}
			if tc.wantDeclaredDir && !strings.Contains(r.stdout, declaredRoot) {
				t.Errorf("the ROOT column does not name the declared root %q\n%s", declaredRoot, r)
			}
		})
	}
}

func goModuleRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func goTestPhasespec(t *testing.T, runPattern string, replacements map[string][]byte) (string, int) {
	t.Helper()
	goRoot := goModuleRoot(t)
	args := []string{"test", "-count=1", "-run", runPattern}
	if len(replacements) > 0 {
		dir := t.TempDir()
		overlay := map[string]map[string]string{"Replace": {}}
		names := make([]string, 0, len(replacements))
		for original := range replacements {
			names = append(names, original)
		}
		sort.Strings(names)
		for i, original := range names {
			mutant := filepath.Join(dir, "mutant"+strconv.Itoa(i)+"_"+filepath.Base(original))
			writeFile(t, mutant, replacements[original])
			overlay["Replace"][original] = mutant
		}
		overlayJSON, err := json.Marshal(overlay)
		if err != nil {
			t.Fatalf("fixture: marshal overlay: %v", err)
		}
		overlayPath := filepath.Join(dir, "overlay.json")
		writeFile(t, overlayPath, overlayJSON)
		args = append(args, "-overlay", overlayPath)
	}
	args = append(args, "./internal/phasespec")
	cmd := exec.Command("go", args...)
	cmd.Dir = goRoot
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("fixture: run go %s: %v", strings.Join(args, " "), err)
		}
		code = exitErr.ExitCode()
	}
	return string(out), code
}

func readSource(t *testing.T, rel string) (string, []byte) {
	t.Helper()
	path := filepath.Join(goModuleRoot(t), filepath.FromSlash(rel))
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("fixture: read %s: %v", path, err)
	}
	return path, src
}

func TestC1812_003_ValidateUserSpecTestKillsReservedKindRuleMutant(t *testing.T) {
	const reservedKindClause = `case "command":`
	validatePath, src := readSource(t, "internal/phasespec/validate.go")
	if n := strings.Count(string(src), reservedKindClause); n != 1 {
		t.Fatalf("fixture: expected exactly one %q clause (the reserved-kind rule) in %s, found %d", reservedKindClause, validatePath, n)
	}
	reservedKindRuleRemoved := []byte(strings.Replace(string(src), reservedKindClause, `case "command-rule-removed-by-mutation":`, 1))

	controlOut, controlCode := goTestPhasespec(t, "^TestValidateUserSpec$", nil)
	if controlCode != 0 {
		t.Fatalf("control: TestValidateUserSpec must pass on the unmutated tree before a mutation verdict means anything\n%s", controlOut)
	}

	mutantOut, mutantCode := goTestPhasespec(t, "^TestValidateUserSpec$", map[string][]byte{validatePath: reservedKindRuleRemoved})

	killers := failedSubtestRE.FindAllStringSubmatch(mutantOut, -1)
	if mutantCode == 0 || len(killers) == 0 {
		t.Errorf("TestValidateUserSpec survives the removal of the reserved-kind rule (validate.go %s): no case pins that rule. The reserved case must use a valid multi-word name with Kind \"command\" and a wantSub that the \"unknown kind\" fallback does not also produce\nmutant exit=%d\n%s", reservedKindClause, mutantCode, mutantOut)
	}
	if strings.Contains(mutantOut, "single-word names are reserved") {
		t.Errorf("the case that pins the reserved-kind rule also trips the single-word name rule: its fixture must use a valid multi-word name so only the rule it names fires\n%s", mutantOut)
	}
}

func TestC1812_004_ReservedKindIsCommandAndNativeIsExecutable(t *testing.T) {
	nativeWithValidName := phasespec.PhaseSpec{Name: "fixture-check", Optional: true, Kind: "native"}
	if v := phasespec.ValidateUserSpec(nativeWithValidName); len(v) != 0 {
		t.Errorf("kind native is executable and must validate clean with a valid multi-word name; got %q", v)
	}

	commandWithValidName := phasespec.PhaseSpec{Name: "fixture-check", Optional: true, Kind: "command"}
	v := phasespec.ValidateUserSpec(commandWithValidName)
	if len(v) != 1 || !strings.Contains(v[0], "reserved") {
		t.Errorf("kind command with a valid name must trip exactly one violation, the reserved-kind rule; got %q", v)
	}

	singleWordName := phasespec.PhaseSpec{Name: "x", Optional: true, Kind: "native"}
	if v := phasespec.ValidateUserSpec(singleWordName); len(v) != 1 || strings.Contains(v[0], "kind") {
		t.Errorf("a single-word name with an executable kind must trip only the name rule; got %q", v)
	}
}

func injectFailIfSignalIntoWalk(t *testing.T, path string, src []byte) []byte {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		t.Fatalf("fixture: parse %s: %v", path, err)
	}
	var walk *ast.FuncDecl
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == "eachTrackedPhaseClassify" {
			walk = fd
		}
	}
	if walk == nil || walk.Body == nil {
		t.Fatalf("fixture: %s no longer defines the shared walk eachTrackedPhaseClassify", path)
	}
	callback := ""
	for _, field := range walk.Type.Params.List {
		if _, isFunc := field.Type.(*ast.FuncType); isFunc && len(field.Names) == 1 {
			callback = field.Names[0].Name
		}
	}
	if callback == "" {
		t.Fatalf("fixture: eachTrackedPhaseClassify has no named func-typed callback parameter")
	}
	at := fset.Position(walk.Body.Lbrace).Offset + 1
	injected := "\n\t" + callback + `("mutant-walk/phase.json", &ClassifyRules{FailIfSignal: map[string]string{"mutant-signal": "x"}})` + "\n"
	out := make([]byte, 0, len(src)+len(injected))
	out = append(out, src[:at]...)
	out = append(out, injected...)
	return append(out, src[at:]...)
}

func TestC1812_005_NoInertFailIfSignalVerdictDependsOnSharedWalk(t *testing.T) {
	const target = "^TestRepoPhaseCatalog_NoInertFailIfSignal$"
	walkPath, src := readSource(t, "internal/phasespec/repo_phaseconfigs_test.go")
	walkFeedsAFailIfSignal := injectFailIfSignalIntoWalk(t, walkPath, src)

	controlOut, controlCode := goTestPhasespec(t, target, nil)
	if controlCode != 0 {
		t.Fatalf("control: TestRepoPhaseCatalog_NoInertFailIfSignal must pass on the unmutated tree\n%s", controlOut)
	}

	mutantOut, mutantCode := goTestPhasespec(t, target, map[string][]byte{walkPath: walkFeedsAFailIfSignal})

	if mutantCode == 0 || !strings.Contains(mutantOut, "--- FAIL: TestRepoPhaseCatalog_NoInertFailIfSignal") || !strings.Contains(mutantOut, "mutant-walk/phase.json") {
		t.Errorf("TestRepoPhaseCatalog_NoInertFailIfSignal ignored a fail_if_signal phase delivered by eachTrackedPhaseClassify: it still walks .evolve/phases itself instead of reusing the shared walk\nmutant exit=%d\n%s", mutantCode, mutantOut)
	}
}

func jsonTag(t *testing.T, typ reflect.Type, field string) string {
	t.Helper()
	f, ok := typ.FieldByName(field)
	if !ok {
		t.Fatalf("fixture: %s has no field %s", typ, field)
	}
	return strings.Split(f.Tag.Get("json"), ",")[0]
}

func section(t *testing.T, doc, heading string) string {
	t.Helper()
	start := strings.Index(doc, "\n"+heading)
	if start < 0 {
		t.Fatalf("fixture: heading %q not found", heading)
	}
	body := doc[start+1+len(heading):]
	for _, next := range []string{"\n### ", "\n## "} {
		if end := strings.Index(body, next); end >= 0 {
			body = body[:end]
		}
	}
	return body
}

// acs-predicate: config-check
func TestC1812_006_PluginDocSection32DescribesPolicyPhaseRoots(t *testing.T) {
	policyKey := jsonTag(t, reflect.TypeOf(policy.Policy{}), "Paths") + "." + jsonTag(t, reflect.TypeOf(policy.PathsConfig{}), "PhaseRoots")
	docPath := filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "phase-plugin-system.md")
	raw, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read %s: %v", docPath, err)
	}
	s32 := section(t, string(raw), "### 3.2")

	if strings.Contains(s32, "EVOLVE_PHASE_ROOTS") {
		t.Errorf("§3.2 still says discovery roots come from EVOLVE_PHASE_ROOTS; no production code reads that variable")
	}
	if !strings.Contains(s32, policyKey) {
		t.Errorf("§3.2 does not name %s, the .evolve/policy.json key policy.PathsConfig decodes the roots from", policyKey)
	}
	if !strings.Contains(s32, "policy.json") {
		t.Errorf("§3.2 does not say the roots are read from .evolve/policy.json")
	}
	if !strings.Contains(s32, "phasespec.Roots") {
		t.Errorf("§3.2 does not name phasespec.Roots, the seam that resolves the roots")
	}
}
