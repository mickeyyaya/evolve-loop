//go:build acs

// Package cycle1659 materializes the acceptance criteria of the two inbox items
// this fleet lane committed (lane-scope.json todo_ids; triage-report.md
// ## top_n) — and nothing else (R9.3: the deferred fleet-scope-excluded
// carryovers get ZERO predicates):
//
// Provenance: this contract was RED-authored in cycle 1652 (go/acs/cycle1652,
// never shipped — that cycle's audit stalled on an artifact timeout) and rode
// the ADR-0076 continuation chain 1652 → 1657 → 1659 with its GREEN build in
// the salvage snapshot. Cycle 1659 rebinds the package to its own number so
// `evolve acs suite --cycle 1659` runs it; the predicate bodies are unchanged
// except for the binding-assert wording the phantom-binding classifier pins.
//
//	triage-empty-commitment-still-dispatches-spine  (P1, pipeline-integrity)
//	dossier-producer-params-struct                  (low, techdebt)
//
// Both inbox records were read verbatim from the worktree's .evolve/inbox/
// (the harness's Task Contract block could not open them at the project root,
// so the records' own acceptance arrays are the authority here).
//
// AC map (1:1 with test-report.md ## AC-Materialization):
//
//	T1-AC1 claimable inbox + top_n [] ⇒ no tdd/build/audit dispatch, a named
//	       non-PASS reason                                    → 001
//	T1-AC2 legitimate empty inbox stays SKIPPED via
//	       recordPlannedNoWorkOutcome (regression pin)        → 002
//	T1-AC3 1623-shaped dossier: impossible from the fixed gate
//	       (001's zero-dispatch) + cyclehealth anomaly        → 004, 005
//	T1 anti-no-op: committed work beside a populated inbox
//	       must still advance                                 → 003
//	T2-AC1 writeCycleDossier ≤ 3 params, every field by name   → 006, 008
//	T2-AC2 a new field compiles without editing call sites
//	       (keyed params at both production callers)         → 007, 008
//	T2-AC3 vet + core/dossier suites green; fixed-input bytes
//	       unchanged                                          → 008, 009
//
// Adversarial axes (skills/adversarial-testing §6). NEGATIVE: 002 (the
// legitimate no-work disposition must survive — a gate that fails EVERY empty
// commitment passes 001 and breaks this), 003 (a gate keyed on "inbox
// non-empty" bricks every productive cycle), 005 (six healthy dossier shapes
// must raise NO commitment anomaly — legacy records without the tasks field,
// an empty commitment that honestly ended at triage, a missing dossier).
// EDGE/OOD: 001's FAIL-verdict row, 005's resumed-at-triage and retro-only
// rows, 008's omitted-evidence row (no fabricated fields). SEMANTIC: dispatch
// (001), disposition (002), anti-no-op (003), historical detection (004),
// quiet-on-healthy (005), API arity (006), call-site shape (007), byte
// preservation (008), suite health (009) — nine distinct behaviours.
//
// No grep-only predicates (cycle-85 ban): 001/002/003/008 run the composed
// orchestrator (RunCycle / RunCycleFromPhase) through the Builder-frozen
// in-package tests, bound by their `--- PASS:` markers in ONE named package
// with -run narrowing (the cycle-976/1587/1648 shape — internal/core is the
// known-slow suite, so the narrowing is what keeps these cheap); 004/005 run
// the REAL `evolve` binary (built once in TestMain) through its dispatcher —
// the production caller of cyclehealth.Check; 006/007 parse the producer's
// Go AST for the criterion that IS a source-shape property (parameter arity,
// keyed call sites) and are paired with 008, which executes the refactored
// producer against the pre-refactor golden bytes; 009 runs vet and the two
// named packages the inbox record demands.
//
// Reachability probe (cycle-644 rule): this package imports only pkg/acsassert
// and the standard library; acs/cycle1659 is a leaf. The fix's own import
// shapes were compiler-probed at RED time: internal/cyclehealth → internal/
// dossier builds (dossier does not depend on cyclehealth), and internal/core
// already imports internal/inboxbatch (task_contract.go).
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

// ---------------------------------------------------------------------------
// Harness: the real CLI, built once (the cycle-1648 TestMain shape).
// ---------------------------------------------------------------------------

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

// repoRootFromCwd mirrors acsassert.RepoRoot for TestMain (no *testing.T yet).
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

// assertBoundTestsPass runs ONE named package narrowed to exactly the bound
// test names (`-run ^(A|B)$`) from the module root and requires each
// `--- PASS:` marker — the cycle-976/1587/1648 binding shape. A renamed,
// missing, failing, or non-compiling test is classified, never silently green.
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

// trimTail bounds a pasted `go test -v` transcript so a failing predicate's
// message stays readable in the audit log.
func trimTail(s string) string {
	const max = 6000
	if len(s) <= max {
		return s
	}
	return "…" + s[len(s)-max:]
}

const corePkg = "./internal/core"

// ---------------------------------------------------------------------------
// Task 1 — triage-empty-commitment-still-dispatches-spine
// ---------------------------------------------------------------------------

// TestC1659_001 — T1-AC1 on BOTH dispatch roots. The composed cycle: a
// claimable lane item in <ProjectRoot>/.evolve/inbox, triage writes top_n []
// (three rows: silent claim race, cycle-1623's narrated deferral, a FAIL
// verdict over the same evidence), and the orchestrator must dispatch no
// tdd/build/audit/ship, record a NAMED reason distinct from
// triage-empty-commitment, never PASS, never IsTriageNoWorkResult, and
// re-dispatch triage at most once. The resumed root (RunCycleFromPhase) must
// agree — the cycle-1639 fresh/resume divergence lesson.
func TestC1659_001_ClaimableInboxWithEmptyCommitmentStopsBeforeImplementationOnBothRoots(t *testing.T) {
	assertBoundTestsPass(t, corePkg,
		"TestRunCycle_EmptyTriageClaimableWorkStopsBeforeImplementation",
		"TestRunCycleFromPhase_ResumedEmptyTriageWithClaimableInboxStopsBeforeImplementation")
}

// TestC1659_002 — T1-AC2, the regression pin (NEGATIVE against an
// over-broad fix). Empty inbox dir, absent inbox dir, and an inbox holding
// only a console-routed (operator-owned, unclaimable) item: all three remain
// the legitimate planned no-work — SKIPPED via recordPlannedNoWorkOutcome,
// triage-empty-commitment, worktree cleaned, IsTriageNoWorkResult true.
func TestC1659_002_LegitimateEmptyInboxStaysPlannedNoWorkSkipped(t *testing.T) {
	assertBoundTestsPass(t, corePkg, "TestRunCycle_EmptyInboxIsPlannedNoWork")
}

// TestC1659_003 — ANTI-NO-OP. Two claimable items queued, triage commits one:
// tdd and build MUST dispatch and the cycle PASSes. A gate that keys on the
// inbox being non-empty (instead of empty-commitment ∧ claimable) satisfies
// 001 and halts every productive cycle.
func TestC1659_003_CommittedWorkBesideAPopulatedInboxStillAdvances(t *testing.T) {
	assertBoundTestsPass(t, corePkg, "TestRunCycle_CommittedWorkWithClaimableInboxStillDispatchesImplementation")
}

// dossierCommitmentSignal is the cyclehealth signal name pinned for the
// historical-record anomaly (the Builder adds it to Check + signalNames).
const dossierCommitmentSignal = "dossier_commitment"

var cycle1623Phases = []string{"scout", "triage", "tdd", "build", "audit", "tdd", "build", "audit", "tdd", "build", "audit", "ship"}

// writeDossierFixture lays out a project root with
// <root>/knowledge-base/cycles/cycle-<n>.json (tasks nil ⇒ field omitted:
// a legacy pre-Tasks record) and <root>/.evolve/runs/cycle-<n>/ as the
// workspace, which the CLI's fallback root derivation resolves back to root.
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

// runCycleHealth drives the REAL binary: `evolve cycle-health <N> <workspace>`
// with EVOLVE_PROJECT_ROOT pinned to the fixture root.
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

// TestC1659_004 — T1-AC3 (the historical half) + house rule 2 REACHABILITY.
// A cycle-1623-shaped dossier — tasks [] beside twelve phases — must be
// classified as an anomaly by the PRODUCTION cyclehealth caller (`evolve
// cycle-health`), the anomaly must name an implementation phase, and the
// written cycle-health.json must list the signal in signals_run (so a Scout
// reading the report sees the check ran, not merely that it stayed quiet).
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

// TestC1659_005 — NEGATIVE / EDGE for T1-AC3. Healthy history must stay quiet:
// an empty commitment that honestly ended at triage (fresh and resumed), a
// legacy record with no tasks field (unknown ≠ empty), committed work with a
// full spine, an empty commitment followed only by retro, and no dossier at
// all. A check that pages on any of these would be worse than none.
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

// ---------------------------------------------------------------------------
// Task 2 — dossier-producer-params-struct
// ---------------------------------------------------------------------------

// producerParamsFields are the ten inputs writeCycleDossier takes today, as the
// named fields the params value must carry (BuildOpts' spellings where
// BuildOpts has the field; Outcome keeps the producer's current name because it
// is the RAW cycle outcome dossierVerdict maps).
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

// TestC1659_006 — T2-AC1. writeCycleDossier's declaration has at most three
// parameters (lock, the params value, optionally a context) and the params
// type cycleDossierParams exists with every current input preserved by name.
// Arity is a source-shape criterion, so the AST is the system under test;
// 008 executes the resulting producer.
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

// TestC1659_007 — T2-AC2 at the two production call sites. Each
// writeCycleDossier call in cycle_closeout.go (normal closeout) and
// cyclerun_epilogue.go (abnormal epilogue) passes ≤ 3 arguments, and any
// composite literal among them is FULLY KEYED — the property that lets a
// twelfth field land without editing either caller. Exactly those two
// production callers remain (no third path was opened, none was dropped).
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

// TestC1659_008 — T2-AC1 + T2-AC3 executed. The refactored producer, fed the
// golden's fixed input through cycleDossierParams, writes the JSON and
// Markdown the eleven-argument producer wrote (golden captured at HEAD
// 287aa81c through the OLD signature); a caller naming only the fields it has
// compiles, runs, and fabricates nothing. The golden files must not be
// gitignored (cycle-93: a testdata file dropped at ship is a silently green
// contract).
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

// TestC1659_009 — T2-AC3 suite health as the inbox record words it: go vet on
// the producer's package, the internal/dossier suite (one named, fast
// package), and the pre-existing producer tests in internal/core narrowed by
// name (the whole core suite is the regression lane's job, never a cycle
// predicate's — flaky-shape rule). Also proves the normal + abnormal closeout
// behaviour is unchanged: the pre-existing lock/failure/not-adopted producer
// tests still pass through the new signature.
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
