//go:build acs

package cycle1659

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var (
	evolveBin      string
	evolveBuildErr error
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "acs-cycle1659-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "acs/cycle1659: tempdir:", err)
		os.Exit(1)
	}
	evolveBin = filepath.Join(dir, "evolve")
	root, err := repoRootFromCwd()
	if err != nil {
		evolveBuildErr = err
	} else {
		out, err := exec.Command("go", "-C", filepath.Join(root, "go"), "build", "-o", evolveBin, "./cmd/evolve").CombinedOutput()
		if err != nil {
			evolveBuildErr = fmt.Errorf("go build ./cmd/evolve: %v\n%s", err, out)
		}
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func repoRootFromCwd() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	out, err := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %v", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	if evolveBuildErr != nil {
		t.Fatalf("RED (harness): %v", evolveBuildErr)
	}
	return evolveBin
}

func assertBoundTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	cmd := exec.Command("go", "test", "-count=1", "-v", "-run", pattern, pkg)
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	out, _ := cmd.CombinedOutput()
	combined := string(out)
	if strings.Contains(combined, "[build failed]") {
		t.Fatalf("RED: %s does not compile — the frozen contract's types/signatures are missing:\n%s", pkg, trimTail(combined))
	}
	for _, n := range names {
		if !strings.Contains(combined, "--- PASS: "+n) {
			t.Errorf("RED: binding test %s did NOT pass in %s (renamed, missing, or failing):\n%s", n, pkg, trimTail(combined))
		}
	}
	if strings.Contains(combined, "--- FAIL:") {
		t.Errorf("RED: a bound test failed in %s:\n%s", pkg, trimTail(combined))
	}
}

func trimTail(s string) string {
	const max = 6000
	if len(s) <= max {
		return s
	}
	return "…" + s[len(s)-max:]
}

const corePkg = "./internal/core"

func TestC1659_001_ClaimableInboxWithEmptyCommitmentStopsBeforeImplementationOnBothRoots(t *testing.T) {
	assertBoundTestsPass(t, corePkg,
		"TestRunCycle_EmptyTriageClaimableWorkStopsBeforeImplementation",
		"TestRunCycleFromPhase_ResumedEmptyTriageWithClaimableInboxStopsBeforeImplementation")
}

func TestC1659_002_LegitimateEmptyInboxStaysPlannedNoWorkSkipped(t *testing.T) {
	assertBoundTestsPass(t, corePkg, "TestRunCycle_EmptyInboxIsPlannedNoWork")
}

func TestC1659_003_CommittedWorkBesideAPopulatedInboxStillAdvances(t *testing.T) {
	assertBoundTestsPass(t, corePkg, "TestRunCycle_CommittedWorkWithClaimableInboxStillDispatchesImplementation")
}

const dossierCommitmentSignal = "dossier_commitment"

var cycle1623Phases = []string{"scout", "triage", "tdd", "build", "audit", "tdd", "build", "audit", "tdd", "build", "audit", "ship"}

func writeDossierFixture(t *testing.T, cycle int, tasks []string, phases []string) (root, workspace string) {
	t.Helper()
	root = t.TempDir()
	workspace = filepath.Join(root, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "knowledge-base", "cycles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	d := map[string]any{
		"cycle":         cycle,
		"run_id":        "01JHISTORICALRUN000000000",
		"goal":          "historical record under test",
		"final_verdict": "PASS",
	}
	if tasks != nil {
		d["tasks"] = tasks
	}
	var recs []map[string]any
	for _, p := range phases {
		recs = append(recs, map[string]any{"name": p, "verdict": "PASS", "duration_ms": 1})
	}
	d["phases"] = recs
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("cycle-%d.json", cycle)), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return root, workspace
}

func runCycleHealth(t *testing.T, root, workspace string, cycle int) (stdout string, code int) {
	t.Helper()
	cmd := exec.Command(evolveBinary(t), "cycle-health", fmt.Sprintf("%d", cycle), workspace)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "EVOLVE_PROJECT_ROOT="+root)
	var sout, serr strings.Builder
	cmd.Stdout, cmd.Stderr = &sout, &serr
	err := cmd.Run()
	code = 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("evolve cycle-health did not run: %v\n%s", err, serr.String())
	}
	if code == 10 {
		t.Fatalf("evolve cycle-health rejected its arguments (exit 10): %s", serr.String())
	}
	return sout.String(), code
}

func commitmentAnomalyLines(stdout string) []string {
	var lines []string
	for _, line := range strings.Split(stdout, "\n") {
		if strings.Contains(line, dossierCommitmentSignal+":") {
			lines = append(lines, line)
		}
	}
	return lines
}

func TestC1659_004_HistoricalEmptyCommitmentDossierWithImplementationIsACyclehealthAnomaly(t *testing.T) {
	cases := []struct {
		name   string
		phases []string
	}{
		{"cycle-1623-twelve-phases", cycle1623Phases},
		{"single-build-after-empty-commitment", []string{"scout", "triage", "build"}},
		{"ship-only-after-empty-commitment", []string{"scout", "triage", "ship"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, ws := writeDossierFixture(t, 1623, []string{}, tc.phases)
			stdout, _ := runCycleHealth(t, root, ws, 1623)
			lines := commitmentAnomalyLines(stdout)
			if len(lines) == 0 {
				t.Fatalf("RED: `evolve cycle-health` raised no %q anomaly for an empty commitment that ran %v; stdout:\n%s", dossierCommitmentSignal, tc.phases, stdout)
			}
			named := false
			for _, l := range lines {
				for _, impl := range []string{"tdd", "build", "audit", "ship"} {
					if strings.Contains(l, impl) {
						named = true
					}
				}
			}
			if !named {
				t.Errorf("RED: the %s anomaly must name the implementation phase(s) that ran; got %q", dossierCommitmentSignal, lines)
			}
			raw, err := os.ReadFile(filepath.Join(ws, "cycle-health.json"))
			if err != nil {
				t.Fatalf("cycle-health.json not written: %v", err)
			}
			var report struct {
				SignalsRun []string `json:"signals_run"`
				Anomalies  []struct {
					Signal   string `json:"signal"`
					Severity string `json:"severity"`
					Message  string `json:"message"`
				} `json:"anomalies"`
			}
			if err := json.Unmarshal(raw, &report); err != nil {
				t.Fatalf("cycle-health.json unparseable: %v", err)
			}
			listed := false
			for _, s := range report.SignalsRun {
				if s == dossierCommitmentSignal {
					listed = true
				}
			}
			if !listed {
				t.Errorf("RED: signals_run %v does not list %q — the report cannot show the check ran", report.SignalsRun, dossierCommitmentSignal)
			}
			recorded := false
			for _, a := range report.Anomalies {
				if a.Signal == dossierCommitmentSignal && (a.Severity == "warn" || a.Severity == "fatal") {
					recorded = true
				}
			}
			if !recorded {
				t.Errorf("RED: cycle-health.json carries no %q anomaly with a valid severity; anomalies=%+v", dossierCommitmentSignal, report.Anomalies)
			}
		})
	}
}

func TestC1659_005_HealthyDossierShapesRaiseNoCommitmentAnomaly(t *testing.T) {
	cases := []struct {
		name    string
		tasks   []string
		phases  []string
		dossier bool
	}{
		{"empty-commitment-ended-at-triage", []string{}, []string{"scout", "triage"}, true},
		{"empty-commitment-resumed-at-triage", []string{}, []string{"triage"}, true},
		{"legacy-record-without-tasks-field", nil, cycle1623Phases, true},
		{"committed-work-with-full-spine", []string{"a-task"}, cycle1623Phases, true},
		{"empty-commitment-then-retro-only", []string{}, []string{"scout", "triage", "retro"}, true},
		{"no-dossier-on-disk", nil, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var root, ws string
			if tc.dossier {
				root, ws = writeDossierFixture(t, 1630, tc.tasks, tc.phases)
			} else {
				root = t.TempDir()
				ws = filepath.Join(root, ".evolve", "runs", "cycle-1630")
				if err := os.MkdirAll(ws, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			stdout, _ := runCycleHealth(t, root, ws, 1630)
			if lines := commitmentAnomalyLines(stdout); len(lines) != 0 {
				t.Errorf("%s anomaly raised on a healthy shape: %q", dossierCommitmentSignal, lines)
			}
		})
	}
}

var producerParamsFields = []string{
	"ProjectRoot", "WorkspacePath", "Cycle", "Goal", "RunID", "Outcome",
	"SkippedPhases", "VerdictsNotAdopted", "SpineFailOpens", "PhaseTimings",
}

func parseCoreFile(t *testing.T, name string) (*token.FileSet, *ast.File) {
	t.Helper()
	path := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "core", name)
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return fset, f
}

func TestC1659_006_WriteCycleDossierTakesAtMostThreeParametersWithEveryFieldNamed(t *testing.T) {
	_, f := parseCoreFile(t, "dossier_producer.go")
	var fn *ast.FuncDecl
	fields := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		switch d := n.(type) {
		case *ast.FuncDecl:
			if d.Name.Name == "writeCycleDossier" && d.Recv == nil {
				fn = d
			}
		case *ast.TypeSpec:
			if d.Name.Name == "cycleDossierParams" {
				st, ok := d.Type.(*ast.StructType)
				if !ok {
					t.Fatalf("RED: cycleDossierParams is not a struct type")
				}
				for _, fl := range st.Fields.List {
					for _, name := range fl.Names {
						fields[name.Name] = true
					}
				}
			}
		}
		return true
	})
	if fn == nil {
		t.Fatalf("RED: writeCycleDossier not declared in internal/core/dossier_producer.go")
	}
	params := 0
	for _, p := range fn.Type.Params.List {
		if len(p.Names) == 0 {
			params++
			continue
		}
		params += len(p.Names)
	}
	if params > 3 {
		t.Errorf("RED: writeCycleDossier takes %d parameters, want ≤ 3 (lock, params value, optional context) — the positional payload is still in place", params)
	}
	if len(fields) == 0 {
		t.Fatalf("RED: no struct type cycleDossierParams declared in dossier_producer.go (fields found: none)")
	}
	for _, want := range producerParamsFields {
		if !fields[want] {
			t.Errorf("RED: cycleDossierParams lacks field %s — a current producer input was not preserved by name", want)
		}
	}
	usesParams := false
	for _, p := range fn.Type.Params.List {
		if id, ok := p.Type.(*ast.Ident); ok && id.Name == "cycleDossierParams" {
			usesParams = true
		}
	}
	if !usesParams {
		t.Errorf("RED: writeCycleDossier does not take a cycleDossierParams value")
	}
}

func TestC1659_007_BothProductionCallersPassKeyedParams(t *testing.T) {
	root := acsassert.RepoRoot(t)
	coreDir := filepath.Join(root, "go", "internal", "core")
	entries, err := os.ReadDir(coreDir)
	if err != nil {
		t.Fatal(err)
	}
	callers := map[string]int{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		_, f := parseCoreFile(t, name)
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := call.Fun.(*ast.Ident)
			if !ok || id.Name != "writeCycleDossier" {
				return true
			}
			callers[name]++
			if len(call.Args) > 3 {
				t.Errorf("RED: %s calls writeCycleDossier with %d positional arguments (want ≤ 3)", name, len(call.Args))
			}
			for _, arg := range call.Args {
				lit, ok := arg.(*ast.CompositeLit)
				if !ok {
					continue
				}
				for _, elt := range lit.Elts {
					if _, keyed := elt.(*ast.KeyValueExpr); !keyed {
						t.Errorf("RED: %s builds the params value with a POSITIONAL element — a field addition would still churn this caller", name)
						break
					}
				}
			}
			return true
		})
	}
	for _, want := range []string{"cycle_closeout.go", "cyclerun_epilogue.go"} {
		if callers[want] == 0 {
			t.Errorf("RED: %s no longer calls writeCycleDossier — the %s closeout path lost its dossier", want, map[string]string{"cycle_closeout.go": "normal", "cyclerun_epilogue.go": "abnormal"}[want])
		}
	}
	total := 0
	for _, n := range callers {
		total += n
	}
	if total != 2 {
		t.Errorf("RED: %d production writeCycleDossier call sites (%v), want exactly 2 (normal + abnormal closeout)", total, callers)
	}
}

func TestC1659_008_RefactoredProducerPreservesPreRefactorBytes(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range []string{
		"go/internal/core/testdata/dossierparams/cycle-4242.golden.json",
		"go/internal/core/testdata/dossierparams/cycle-4242.golden.md",
	} {
		if !acsassert.FileExists(t, filepath.Join(root, rel)) {
			t.Fatalf("RED: golden %s missing on disk", rel)
		}
		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", rel); code == 0 {
			t.Errorf("RED: golden %s is gitignored — it would be dropped at ship", rel)
		}
	}
	assertBoundTestsPass(t, corePkg,
		"TestWriteCycleDossier_ParamsStructPreservesFixedInputBytes",
		"TestWriteCycleDossier_ParamsAreKeyedAndOptional")
}

func TestC1659_009_VetAndDossierSuitesStayGreenThroughTheRefactor(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	vet := exec.Command("go", "vet", corePkg)
	vet.Dir = goDir
	if out, err := vet.CombinedOutput(); err != nil {
		t.Errorf("RED: go vet %s: %v\n%s", corePkg, err, trimTail(string(out)))
	}
	dossier := exec.Command("go", "test", "-count=1", "./internal/dossier")
	dossier.Dir = goDir
	if out, err := dossier.CombinedOutput(); err != nil {
		t.Errorf("RED: go test ./internal/dossier: %v\n%s", err, trimTail(string(out)))
	}
	assertBoundTestsPass(t, corePkg,
		"TestWriteCycleDossier_WritesValidArtifact",
		"TestWriteCycleDossier_FailOutcomeRecordsDefect",
		"TestWriteCycleDossier_LeavesCleanTree",
		"TestDossierVerdict_MapsCycleOutcomes")
}
