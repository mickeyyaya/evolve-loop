//go:build acs

// Package cycle2552 holds the predicates of the T7 structure-pin replacement contract.
package cycle2552

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/testaudit"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	modulePath   = "github.com/mickeyyaya/evolve-loop/go"
	corePkg      = modulePath + "/internal/core"
	panetrustPkg = modulePath + "/internal/panetrust"
)

var pinRules = map[string]bool{
	"single-file-source-pin": true,
	"single-file-ast-pin":    true,
	"literal-field-count":    true,
}

var runSlots = make(chan struct{}, 3)

type edit struct {
	file string
	old  string
	new  string
	all  bool
}

func internalPkgDir(t *testing.T, pkg string) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go", "internal", pkg)
}

func coreDir(t *testing.T) string {
	t.Helper()
	return internalPkgDir(t, "core")
}

func testFuncNames(t *testing.T, dir string, globs ...string) []string {
	t.Helper()
	seen := map[string]bool{}
	for _, glob := range globs {
		files, err := filepath.Glob(filepath.Join(dir, glob))
		if err != nil {
			t.Fatalf("glob %s: %v", glob, err)
		}
		for _, path := range files {
			if !strings.HasSuffix(path, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if ok && fn.Recv == nil && strings.HasPrefix(fn.Name.Name, "Test") && len(fn.Type.Params.List) == 1 {
					seen[fn.Name.Name] = true
				}
			}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func writeOverlay(t *testing.T, edits []edit) (string, map[string]string) {
	t.Helper()
	tmp := t.TempDir()
	sources := map[string]string{}
	order := []string{}
	for _, e := range edits {
		if _, ok := sources[e.file]; !ok {
			raw, err := os.ReadFile(e.file)
			if err != nil {
				t.Fatalf("read %s: %v", e.file, err)
			}
			sources[e.file] = string(raw)
			order = append(order, e.file)
		}
		src := sources[e.file]
		switch {
		case e.old == "":
			src += e.new
		case e.all:
			if strings.Count(src, e.old) == 0 {
				t.Fatalf("mutation anchor missing in %s: %q", filepath.Base(e.file), e.old)
			}
			src = strings.ReplaceAll(src, e.old, e.new)
		default:
			if n := strings.Count(src, e.old); n != 1 {
				t.Fatalf("mutation anchor must occur once in %s, found %d: %q", filepath.Base(e.file), n, e.old)
			}
			src = strings.Replace(src, e.old, e.new, 1)
		}
		sources[e.file] = src
	}
	replace := map[string]string{}
	for i, file := range order {
		out := filepath.Join(tmp, fmt.Sprintf("%d_%s", i, filepath.Base(file)))
		if err := os.WriteFile(out, []byte(sources[file]), 0o644); err != nil {
			t.Fatal(err)
		}
		replace[file] = out
	}
	raw, err := json.Marshal(map[string]map[string]string{"Replace": replace})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(tmp, "overlay.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path, replace
}

func goTest(t *testing.T, overlay, pkg string, names []string) (string, int) {
	t.Helper()
	runSlots <- struct{}{}
	defer func() { <-runSlots }()
	args := []string{"test", "-count=1", "-timeout", "10m"}
	if overlay != "" {
		args = append(args, "-overlay="+overlay)
	}
	if len(names) > 0 {
		args = append(args, "-run", "^("+strings.Join(names, "|")+")$")
	}
	args = append(args, pkg)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", args...)
	if code == -1 {
		t.Fatalf("go test failed to launch: %v", err)
	}
	return stdout + stderr, code
}

func failures(out string) string {
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "--- FAIL") || strings.Contains(line, ".go:") {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	return lastBytes(strings.Join(lines, "\n"), 2500)
}

func lastBytes(text string, keep int) string {
	if len(text) > keep {
		text = text[len(text)-keep:]
	}
	return text
}

func requireDetectors(t *testing.T, dir string, globs ...string) []string {
	t.Helper()
	names := testFuncNames(t, dir, globs...)
	if len(names) == 0 {
		t.Fatalf("RED: no test function remains in %v — the claim of the removed pin has no test", globs)
	}
	return names
}

func assertMutantKilled(t *testing.T, edits []edit, pkg string, names []string) {
	t.Helper()
	t.Parallel()
	overlay, _ := writeOverlay(t, edits)
	out, code := goTest(t, overlay, pkg, names)
	if strings.Contains(out, "[build failed]") || strings.Contains(out, "[setup failed]") {
		t.Fatalf("the behavior mutant did not compile:\n%s", lastBytes(out, 1500))
	}
	if code == 0 {
		t.Errorf("RED: no test of %v failed on the behavior mutant — the claim is not covered by a behavior test", names)
	}
}

func assertRenameSurvived(t *testing.T, edits []edit, runs map[string][]string) {
	t.Helper()
	t.Parallel()
	overlay, replace := writeOverlay(t, edits)
	root := acsassert.RepoRoot(t)
	leaves := map[string]bool{}
	for orig := range replace {
		leaves[filepath.Dir(orig)] = true
	}
	pkgs := make([]string, 0, len(runs))
	for pkg := range runs {
		pkgs = append(pkgs, pkg)
		leaves[pkgDir(root, pkg)] = true
	}
	sort.Strings(pkgs)
	shadow := t.TempDir()
	mirrorTree(t, root, shadow, leaves, replace)
	for _, pkg := range pkgs {
		bin := filepath.Join(t.TempDir(), "pkg.test")
		out, code := goBuildTest(t, overlay, pkg, bin)
		if code != 0 {
			t.Fatalf("the rename mutant did not compile:\n%s", lastBytes(out, 1500))
		}
		rel, err := filepath.Rel(root, pkgDir(root, pkg))
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(bin, runArgs(runs[pkg])...)
		cmd.Dir = filepath.Join(shadow, rel)
		raw, runErr := cmd.CombinedOutput()
		if runErr != nil {
			t.Errorf("RED: %s failed on a behavior-preserving rename — a test reads source text instead of running code:\n%s", pkg, failures(string(raw)))
		}
	}
}

func runArgs(names []string) []string {
	args := []string{"-test.count=1", "-test.timeout=10m"}
	if len(names) > 0 {
		args = append(args, "-test.run", "^("+strings.Join(names, "|")+")$")
	}
	return args
}

func pkgDir(root, pkg string) string {
	return filepath.Join(root, "go", strings.TrimPrefix(pkg, modulePath+"/"))
}

func goBuildTest(t *testing.T, overlay, pkg, out string) (string, int) {
	t.Helper()
	runSlots <- struct{}{}
	defer func() { <-runSlots }()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-c", "-overlay="+overlay, "-o", out, pkg)
	if code == -1 {
		t.Fatalf("go test -c failed to launch: %v", err)
	}
	return stdout + stderr, code
}

func mirrorTree(t *testing.T, src, dst string, leaves map[string]bool, replace map[string]string) {
	t.Helper()
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		from, to := filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())
		switch {
		case replace[from] != "":
			raw, err := os.ReadFile(replace[from])
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(to, raw, 0o644); err != nil {
				t.Fatal(err)
			}
		case entry.IsDir() && hasLeafUnder(from, leaves):
			mirrorTree(t, from, to, leaves, replace)
		default:
			if err := os.Symlink(from, to); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func hasLeafUnder(dir string, leaves map[string]bool) bool {
	for leaf := range leaves {
		if leaf == dir || strings.HasPrefix(leaf, dir+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func TestC2552_001_BaselineHoldsNoStructurePinRows(t *testing.T) {
	path := filepath.Join(coreDir(t), "testdata", "testaudit-baseline.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var shard struct {
		Rows []testaudit.Finding `json:"rows"`
	}
	if err := json.Unmarshal(raw, &shard); err != nil {
		t.Fatalf("decode baseline: %v", err)
	}
	for _, row := range shard.Rows {
		if pinRules[row.Rule] {
			t.Errorf("RED: baseline still holds %s %s", row.Rule, row.Test)
		}
	}
}

func TestC2552_002_AnalyzerFindsNoStructurePinInCoreTests(t *testing.T) {
	cfg, err := testaudit.DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Clean(coreDir(t))
	found, err := testaudit.Analyze(dir, cfg)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	for _, f := range found {
		if f.Package == dir && pinRules[f.Rule] {
			t.Errorf("RED: analyzer reports %s on %s", f.Rule, f.Test)
		}
	}
}

func TestC2552_003_TheLadderGuardStillRunsItsShapeCases(t *testing.T) {
	names := []string{"TestNoDecisionLiteralOutsideDecider", "TestDecisionGuardSeesEveryShapeAndAlias", "TestGuardCoversTheModuleRootNotOnlyInternal"}
	have := map[string]bool{}
	for _, name := range testFuncNames(t, coreDir(t), "retry_ladder_decider_test.go") {
		have[name] = true
	}
	for _, name := range names {
		if !have[name] {
			t.Errorf("RED: %s was removed — the AST pin must be rewritten, not deleted, because no other test runs the guard on aliased and shaped decision builds", name)
		}
	}
	out, code := goTest(t, "", corePkg, names)
	if code != 0 {
		t.Errorf("RED: the ladder guard tests failed:\n%s", lastBytes(out, 1500))
	}
}

func TestC2552_004_RedactionLossKillsATestInPanetrust(t *testing.T) {
	edits := []edit{{
		file: filepath.Join(internalPkgDir(t, "panetrust"), "panetrust.go"),
		old:  "func redactSecrets(s string) string { return redact.Secrets(s) }",
		new:  "func redactSecrets(s string) string { _ = redact.Secrets; return s }",
	}}
	assertMutantKilled(t, edits, panetrustPkg, nil)
}

func TestC2552_005_RenamingTheRedactionSeamBreaksNoTest(t *testing.T) {
	edits := []edit{{
		file: filepath.Join(internalPkgDir(t, "panetrust"), "panetrust.go"),
		old:  "redactSecrets",
		new:  "mutScrubSecrets",
		all:  true,
	}}
	runs := map[string][]string{panetrustPkg: nil}
	if names := testFuncNames(t, coreDir(t), "seams_test.go"); len(names) > 0 {
		runs[corePkg] = names
	}
	assertRenameSurvived(t, edits, runs)
}

func TestC2552_006_ADriftedRepairPrefixKillsARepairTest(t *testing.T) {
	names := requireDetectors(t, coreDir(t), "audit_repair_*_test.go", "audit_gate_remedy_test.go")
	edits := []edit{{
		file: filepath.Join(coreDir(t), "audit_fail_decision.go"),
		old:  "auditRepairReasonPrefix + string(action)",
		new:  `"audit-repair:" + string(action)`,
	}}
	assertMutantKilled(t, edits, corePkg, names)
}

func TestC2552_007_RoutingTheRepairPrefixThroughAnotherConstantBreaksNoTest(t *testing.T) {
	names := requireDetectors(t, coreDir(t), "audit_repair_*_test.go", "audit_gate_remedy_test.go")
	edits := []edit{
		{
			file: filepath.Join(coreDir(t), "audit_fail_decision.go"),
			old:  "auditRepairReasonPrefix + string(action)",
			new:  "mutRepairPrefix + string(action)",
		},
		{
			file: filepath.Join(coreDir(t), "audit_fail_decision.go"),
			new:  "\nconst mutRepairPrefix = \"audit-repair: \"\n",
		},
	}
	assertRenameSurvived(t, edits, map[string][]string{corePkg: names})
}

func TestC2552_008_DroppingTheLessonOnTheFreshPathKillsALessonTest(t *testing.T) {
	names := requireDetectors(t, coreDir(t), "judgment_lesson*_test.go")
	edits := []edit{{
		file: filepath.Join(coreDir(t), "phase_completion.go"),
		old:  "\t\tr.orchestrator.recordJudgmentLesson(r.ctx, r.cycle, cs.WorkspacePath, r.phase, r.state, r.response.Diagnostics)\n",
		new:  "",
	}}
	assertMutantKilled(t, edits, corePkg, names)
}

func TestC2552_009_DroppingTheLessonOnTheResumePathKillsATest(t *testing.T) {
	names := requireDetectors(t, coreDir(t), "judgment_lesson*_test.go", "resume_*_test.go")
	edits := []edit{{
		file: filepath.Join(coreDir(t), "resume_execution.go"),
		old:  "\t\tif err := completion.persist(); err != nil {\n\t\t\treturn result, err\n\t\t}\n\t\tcursor.advance(next, resp.Verdict)\n",
		new: "\t\tif resp.Verdict == VerdictFAIL && !o.isAuthoritativePhase(next) {\n\t\t\tcompletion.state = nil\n\t\t}\n" +
			"\t\tif err := completion.persist(); err != nil {\n\t\t\treturn result, err\n\t\t}\n\t\tcursor.advance(next, resp.Verdict)\n",
	}}
	assertMutantKilled(t, edits, corePkg, names)
}

func TestC2552_010_RenamingTheCompletionBoundaryBreaksNoLessonTest(t *testing.T) {
	names := requireDetectors(t, coreDir(t), "judgment_lesson*_test.go")
	edits := []edit{
		{
			file: filepath.Join(coreDir(t), "phase_completion.go"),
			old:  "\t\tr.orchestrator.recordJudgmentLesson(r.ctx,",
			new:  "\t\tmutLesson := r.orchestrator.recordJudgmentLesson\n\t\tmutLesson(r.ctx,",
		},
		{
			file: filepath.Join(coreDir(t), "resume_execution.go"),
			old:  "completion := phaseCompletionRecord{",
			new:  "completion := mutCompletion{",
		},
		{
			file: filepath.Join(coreDir(t), "resume_execution.go"),
			new:  "\ntype mutCompletion = phaseCompletionRecord\n",
		},
	}
	assertRenameSurvived(t, edits, map[string][]string{corePkg: names})
}

func TestC2552_011_ResumeExitWithoutRunnerRecordsItsOutcome(t *testing.T) {
	assertResumeExitMutantKilled(t, edit{
		old: "\t\t\to.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath,\n\t\t\t\tphaseOutcomeFrom(next, PhaseResponse{}, 0, noRunnerErr.Error(), \"\"))\n",
	})
}

func TestC2552_012_ResumeExitOnNonCanonicalVerdictRecordsItsOutcome(t *testing.T) {
	assertResumeExitMutantKilled(t, edit{
		old: "\t\t\to.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempts, ferr.Error(), cs.PhaseStartedAt))\n",
	})
}

func TestC2552_013_ResumeExitAfterDeliverableReviewRecordsItsOutcome(t *testing.T) {
	assertResumeExitMutantKilled(t, edit{
		old: "\t\t\to.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempts, err.Error(), cs.PhaseStartedAt))\n",
	})
}

func TestC2552_014_ResumeExitAfterBuildExplanationRefreshRecordsItsOutcome(t *testing.T) {
	assertResumeExitMutantKilled(t, edit{
		old: "\t\t\t\to.recordPhaseOutcome(&result, &phaseTimings, cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempts, phaseErr.Error(), cs.PhaseStartedAt))\n\t\t\t\treturn result, phaseErr\n",
		new: "\t\t\t\treturn result, phaseErr\n",
	})
}

func TestC2552_015_ResumeIterationLimitRecordsTheChokepointEscape(t *testing.T) {
	assertResumeExitMutantKilled(t, edit{
		old: "timingOwner.recordChokepointEscape(",
		new: "mutNoEscape(",
	}, edit{new: "\nfunc mutNoEscape(string) {}\n"})
}

func TestC2552_016_RenamingTheResumeRecordingCallsBreaksNoTest(t *testing.T) {
	file := filepath.Join(coreDir(t), "resume_execution.go")
	names := resumeDetectors(t)
	edits := []edit{
		{file: file, old: "o.recordPhaseOutcome(", new: "mutNoteOutcome(o, ", all: true},
		{file: file, old: "phaseOutcomeFrom(", new: "mutOutcomeOf(", all: true},
		{file: file, old: "timingOwner.recordChokepointEscape(", new: "mutNoteEscape(timingOwner, ", all: true},
		{file: file, new: "\nvar mutNoteOutcome = (*Orchestrator).recordPhaseOutcome\nvar mutOutcomeOf = phaseOutcomeFrom\nvar mutNoteEscape = (*cycleRun).recordChokepointEscape\n"},
	}
	assertRenameSurvived(t, edits, map[string][]string{corePkg: names})
}

func resumeDetectors(t *testing.T) []string {
	t.Helper()
	return requireDetectors(t, coreDir(t), "resume_*_test.go", "orchestrator_phaseoutcome_test.go")
}

func assertResumeExitMutantKilled(t *testing.T, mutation edit, extra ...edit) {
	t.Helper()
	mutation.file = filepath.Join(coreDir(t), "resume_execution.go")
	edits := []edit{mutation}
	for _, e := range extra {
		e.file = mutation.file
		edits = append(edits, e)
	}
	assertMutantKilled(t, edits, corePkg, resumeDetectors(t))
}
