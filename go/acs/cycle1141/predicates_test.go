//go:build acs

package cycle1141

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/docsfloor"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func acsRepoRoot(t *testing.T) string {
	t.Helper()
	return acsassert.RepoRoot(t)
}

func acsSubprocess(t *testing.T, name string, args ...string) (stdout, stderr string, code int, err error) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = filepath.Join(acsRepoRoot(t), "go")
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	if ee, ok := runErr.(*exec.ExitError); ok {
		return out.String(), errBuf.String(), ee.ExitCode(), nil
	}
	if runErr != nil {
		return out.String(), errBuf.String(), -1, runErr
	}
	return out.String(), errBuf.String(), 0, nil
}

func TestC1141_001_dossier_fail_defect_derives_audit_artifact_name(t *testing.T) {
	auditContract, ok := phasecontract.For("audit")
	if !ok {
		t.Fatalf("registry has no audit contract — cannot derive expected artifact name")
	}
	want := auditContract.ArtifactName
	if want == "" {
		t.Fatalf("registry audit contract has empty ArtifactName")
	}

	d, err := dossier.Build(1141, dossier.BuildOpts{
		WorkspacePath: t.TempDir(),
		Goal:          "cycle-1141 acs probe",
		FinalVerdict:  "FAIL",
	})
	if err != nil {
		t.Fatalf("dossier.Build returned error: %v", err)
	}
	if len(d.Defects) == 0 {
		t.Fatalf("FAIL verdict produced no defects — cannot verify artifact-name derivation")
	}
	if !strings.Contains(d.Defects[0].Summary, want) {
		t.Errorf("defect summary %q does not reference the registry audit artifact %q",
			d.Defects[0].Summary, want)
	}

	src := filepath.Join(acsRepoRoot(t), "go", "internal", "dossier", "build.go")
	assertNoRawLiteral(t, src, `"audit-report.md"`)
	assertReferencesRegistry(t, src)
}

func TestC1141_002_gc_discover_markers_track_registry_artifacts(t *testing.T) {
	required := phasecontract.RequiredArtifacts()
	if len(required) == 0 {
		t.Fatalf("registry RequiredArtifacts() is empty — nothing to bind")
	}

	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	runsDir := filepath.Join(evolveDir, "runs")
	for i, name := range required {
		dir := filepath.Join(runsDir, "run-"+itoa(i))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("write marker %s: %v", name, err)
		}
	}
	decoy := filepath.Join(runsDir, "run-decoy")
	if err := os.MkdirAll(decoy, 0o755); err != nil {
		t.Fatalf("mkdir decoy: %v", err)
	}
	if err := os.WriteFile(filepath.Join(decoy, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write decoy: %v", err)
	}

	found, err := gc.Discover(evolveDir, gc.DiscoverOptions{})
	if err != nil {
		t.Fatalf("gc.Discover returned error: %v", err)
	}
	seen := map[string]bool{}
	for _, r := range found {
		seen[filepath.Base(r.Path)] = true
	}
	for i, name := range required {
		if !seen["run-"+itoa(i)] {
			t.Errorf("run dir evidenced only by registry artifact %q was NOT discovered — marker set is not derived from the registry", name)
		}
	}
	if seen["run-decoy"] {
		t.Errorf("run-decoy (no marker file) was discovered — Discover is not discriminating")
	}

	src := filepath.Join(acsRepoRoot(t), "go", "internal", "gc", "discover.go")
	for _, lit := range []string{`"scout-report.md"`, `"build-report.md"`, `"audit-report.md"`} {
		assertNoRawLiteral(t, src, lit)
	}
	assertReferencesRegistry(t, src)
}

func TestC1141_003_lanescope_scout_report_name_derived(t *testing.T) {
	src := filepath.Join(acsRepoRoot(t), "go", "internal", "core", "lanescope.go")
	assertNoRawLiteral(t, src, `"scout-report.md"`)
	assertReferencesRegistry(t, src)

	stdout, stderr, code, err := acsSubprocess(t, "go", "test", "-count=1",
		"-run", "TestNormalizeScoutGoalHash", "./internal/core/")
	if err != nil {
		t.Fatalf("running core lanescope tests: %v", err)
	}
	if code != 0 {
		t.Errorf("go test -run TestNormalizeScoutGoalHash ./internal/core/ exited %d after the SSOT refactor\nstdout:\n%s\nstderr:\n%s",
			code, stdout, stderr)
	}
}

func TestC1141_004_routing_mandatory_no_regression(t *testing.T) {
	cfg, _ := config.Load(filepath.Join(t.TempDir(), "absent-registry.json"), map[string]string{})
	if len(cfg.Mandatory) == 0 {
		t.Fatalf("routing config Mandatory is empty — routing completeness floor lost")
	}
	have := map[string]bool{}
	for _, p := range cfg.Mandatory {
		have[p] = true
	}
	for _, role := range phasecontract.RequiredRoles() {
		phase := phaseForRole(t, role)
		if !have[phase] {
			t.Errorf("registry-required role %q (phase %q) is missing from routing Mandatory %v",
				role, phase, cfg.Mandatory)
		}
	}
	if !have["ship"] {
		t.Errorf(`routing Mandatory %v dropped "ship" — the routing-only phase the registry does not cover`, cfg.Mandatory)
	}
	for _, p := range cfg.Mandatory {
		if _, ok := phasecontract.For(p); !ok {
			t.Errorf("routing Mandatory names %q which is not a registered phase — vocabulary drift", p)
		}
	}
}

func TestC1141_005_mandatory_divergence_derived_or_documented(t *testing.T) {
	src := filepath.Join(acsRepoRoot(t), "go", "internal", "config", "config.go")
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read config.go: %v", err)
	}
	text := string(b)

	derived := strings.Contains(text, "phasecontract")
	documented := hasDivergenceComment(text)
	if !derived && !documented {
		t.Errorf("config.go Mandatory neither derives from phasecontract nor carries an explicit divergence comment naming the registry — the audit produced no recorded decision")
	}
}

func TestC1141_006_docsfloor_warns_on_undocumented_architecture_change(t *testing.T) {
	v := docsfloor.Evaluate(
		docsfloor.Config{Stage: "enforce"},
		docsfloor.Input{
			ArchitectureLabeled: true,
			ChangedFiles:        []string{"go/internal/core/cyclerun.go", "go/internal/policy/policy.go"},
		},
	)
	if v.Status != docsfloor.StatusWarn {
		t.Errorf("architecture-labeled change with no docs touch: got status %q, want %q",
			v.Status, docsfloor.StatusWarn)
	}
	if strings.TrimSpace(v.Reason) == "" {
		t.Errorf("WARN verdict carries an empty Reason — an unexplained warning is unactionable")
	}
}

func TestC1141_007_docsfloor_decision_table(t *testing.T) {
	cases := []struct {
		name string
		cfg  docsfloor.Config
		in   docsfloor.Input
		want string
	}{
		{
			name: "architecture change touching docs passes",
			cfg:  docsfloor.Config{Stage: "enforce"},
			in: docsfloor.Input{ArchitectureLabeled: true, ChangedFiles: []string{
				"go/internal/core/cyclerun.go", "docs/architecture/adr-0077-docs-floor.md"}},
			want: docsfloor.StatusPass,
		},
		{
			name: "architecture change touching any docs file passes",
			cfg:  docsfloor.Config{Stage: "enforce"},
			in: docsfloor.Input{ArchitectureLabeled: true, ChangedFiles: []string{
				"go/internal/policy/policy.go", "docs/operations/operating-policy.md"}},
			want: docsfloor.StatusPass,
		},
		{
			name: "non-architecture change is skipped",
			cfg:  docsfloor.Config{Stage: "enforce"},
			in: docsfloor.Input{ArchitectureLabeled: false, ChangedFiles: []string{
				"go/internal/core/cyclerun.go"}},
			want: docsfloor.StatusSkip,
		},
		{
			name: "stage off never fires",
			cfg:  docsfloor.Config{Stage: "off"},
			in: docsfloor.Input{ArchitectureLabeled: true, ChangedFiles: []string{
				"go/internal/core/cyclerun.go"}},
			want: docsfloor.StatusSkip,
		},
		{
			name: "empty change set is not judged",
			cfg:  docsfloor.Config{Stage: "enforce"},
			in:   docsfloor.Input{ArchitectureLabeled: true, ChangedFiles: nil},
			want: docsfloor.StatusSkip,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := docsfloor.Evaluate(tc.cfg, tc.in)
			if got.Status != tc.want {
				t.Errorf("got status %q, want %q (reason %q)", got.Status, tc.want, got.Reason)
			}
		})
	}
}

func TestC1141_008_docsfloor_config_injected_and_wired(t *testing.T) {
	dir := t.TempDir()
	bare := filepath.Join(dir, "bare-policy.json")
	if err := os.WriteFile(bare, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write bare policy: %v", err)
	}
	p, err := policy.Load(bare)
	if err != nil {
		t.Fatalf("policy.Load(bare): %v", err)
	}
	if got := p.DocsFloorConfig().Stage; got != "enforce" {
		t.Errorf("absent docs_floor block: compiled default stage = %q, want %q", got, "enforce")
	}

	overridden := filepath.Join(dir, "override-policy.json")
	if err := os.WriteFile(overridden, []byte(`{"docs_floor":{"stage":"off"}}`), 0o644); err != nil {
		t.Fatalf("write override policy: %v", err)
	}
	p2, err := policy.Load(overridden)
	if err != nil {
		t.Fatalf("policy.Load(override): %v", err)
	}
	if got := p2.DocsFloorConfig().Stage; got != "off" {
		t.Errorf("policy.json docs_floor.stage override did not reach the gate: got %q, want %q", got, "off")
	}

	if !hasProductionCaller(t, "docsfloor.Evaluate") {
		t.Errorf("no production (non-test) file outside internal/docsfloor calls docsfloor.Evaluate — the gate is inert")
	}
}

var docPrefixes = []string{"docs/"}

func assertNoRawLiteral(t *testing.T, path, literal string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if strings.Contains(string(b), literal) {
		t.Errorf("%s still hardcodes %s — not derived from phasecontract", path, literal)
	}
}

func assertReferencesRegistry(t *testing.T, path string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(b), "phasecontract") {
		t.Errorf("%s does not reference phasecontract — the name is not derived from the registry SSOT", path)
	}
}

func hasDivergenceComment(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "//") {
			continue
		}
		low := strings.ToLower(trimmed)
		if !strings.Contains(low, "phasecontract") && !strings.Contains(low, "requiredroles") {
			continue
		}
		if strings.Contains(low, "mandatory") || strings.Contains(low, "diverg") || strings.Contains(low, "deliberately") {
			return true
		}
	}
	return false
}

func phaseForRole(t *testing.T, role string) string {
	t.Helper()
	for _, c := range phasecontract.Contracts() {
		if c.AgentName == role {
			return c.Phase
		}
	}
	t.Fatalf("no registry contract has AgentName %q", role)
	return ""
}

func hasProductionCaller(t *testing.T, call string) bool {
	t.Helper()
	stdout, _, _, err := acsSubprocess(t, "grep", "-rl", "--include=*.go", call,
		filepath.Join(acsRepoRoot(t), "go", "internal"),
		filepath.Join(acsRepoRoot(t), "go", "cmd"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasSuffix(line, "_test.go") {
			continue
		}
		if strings.Contains(line, string(filepath.Separator)+"docsfloor"+string(filepath.Separator)) {
			continue
		}
		return true
	}
	return false
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	out := ""
	for i > 0 {
		out = string(rune('0'+i%10)) + out
		i /= 10
	}
	return out
}

var _ = docPrefixes
