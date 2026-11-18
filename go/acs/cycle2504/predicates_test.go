//go:build acs

package cycle2504

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/testaudit"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	bridgePkg      = "internal/bridge"
	goldenFileName = "launch-outcome.golden.json"
	baselineShard  = "/testdata/testaudit-baseline.json"
)

var tautologyRules = []string{"no-assertion", "log-only", "asserts-own-setup", "both-branches-pass"}

var flaggedBridgeTests = []string{
	"TestClaudeCloudDriver_ReadsTheConfirmationFromTheJoinedPaneWhenTheControllerOffersIt",
	"TestDoctorReport_BoundByDoctor",
	"TestEngine_ConcurrentLaunch_OnBootRaceFree",
	"TestLaunchOutcome_GoldenRowsCoverEveryCodeCtxAndFixture",
}

type sourceMutant struct {
	label string
	file  string
	old   string
	new   string
}

var (
	swallowedLaunchOutcome = sourceMutant{
		label: "Engine.Launch returns an empty success instead of the outcome of the launch",
		file:  "engine.go",
		old:   "\treturn resp, err\n}\n\nfunc (e *Engine) launchModel",
		new:   "\t_, _ = resp, err\n\treturn core.BridgeResponse{}, nil\n}\n\nfunc (e *Engine) launchModel",
	}
	plainPaneWait = sourceMutant{
		label: "the cloud wait reads a plain pane although the controller offers a joined one",
		file:  "driver_claudecloud.go",
		old:   "if j, ok := l.deps.Tmux.(JoinedPaneCapturer); ok {",
		new:   "if j, ok := l.deps.Tmux.(JoinedPaneCapturer); ok && false {",
	}
	failedDeepProbe = sourceMutant{
		label: "the deep probe never passes",
		file:  "doctor.go",
		old:   "Passed: err == nil && rc == 0",
		new:   "Passed: err == nil && rc == 0 && false",
	}
	wrongDoctorVerdict = sourceMutant{
		label: "a clean binary is reported as a warning",
		file:  "doctor.go",
		old:   "\t\tr.Verdict = \"ready\"",
		new:   "\t\tr.Verdict = \"warning\"",
	}
)

func bridgeFindings(t *testing.T, goDir string) []string {
	t.Helper()
	cfg, err := testaudit.DefaultConfig()
	if err != nil {
		t.Fatalf("testaudit.DefaultConfig: %v", err)
	}
	prevDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(goDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(prevDir); err != nil {
			t.Errorf("restore the working directory: %v", err)
		}
	})
	found, err := testaudit.Analyze(".", cfg)
	if err != nil {
		t.Fatalf("testaudit.Analyze: %v", err)
	}
	var rows []string
	for _, f := range found {
		inBridge := f.Package == bridgePkg || strings.HasPrefix(f.Package, bridgePkg+"/")
		if inBridge && slices.Contains(tautologyRules, f.Rule) {
			rows = append(rows, fmt.Sprintf("%s %s %s", f.Package, f.Test, f.Rule))
		}
	}
	return rows
}

func bridgeShardRows(t *testing.T, goDir string) []string {
	t.Helper()
	shards, err := acsassert.WalkModule(goDir, func(rel string) bool {
		return strings.HasPrefix(rel, bridgePkg+"/") && strings.HasSuffix("/"+rel, baselineShard)
	})
	if err != nil {
		t.Fatalf("scan baseline shards: %v", err)
	}
	var rows []string
	for _, rel := range shards {
		raw, err := os.ReadFile(filepath.Join(goDir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		var shard struct {
			Rows []testaudit.Finding `json:"rows"`
		}
		if err := json.Unmarshal(raw, &shard); err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		for _, r := range shard.Rows {
			if slices.Contains(tautologyRules, r.Rule) {
				rows = append(rows, fmt.Sprintf("%s: %s %s %s", rel, r.Package, r.Test, r.Rule))
			}
		}
	}
	return rows
}

func TestC2504_001_NoTautologicalFindingLeftInBridge(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	if left := bridgeFindings(t, goDir); len(left) > 0 {
		t.Errorf("RED: %d tautological finding(s) remain in internal/bridge:\n%s", len(left), strings.Join(left, "\n"))
	}
	if rows := bridgeShardRows(t, goDir); len(rows) > 0 {
		t.Errorf("RED: %d tautology row(s) sit in an internal/bridge baseline shard:\n%s", len(rows), strings.Join(rows, "\n"))
	}
}

func declaredTests(t *testing.T, dir string) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	declared := map[string]bool{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, e.Name()), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		for _, d := range file.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil {
				declared[fn.Name.Name] = true
			}
		}
	}
	return declared
}

func TestC2504_002_FlaggedBridgeTestsAreRewrittenNotDeleted(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	declared := declaredTests(t, filepath.Join(goDir, filepath.FromSlash(bridgePkg)))
	for _, name := range flaggedBridgeTests {
		if !declared[name] {
			t.Errorf("RED: %s was deleted or renamed instead of rewritten; a deleted test leaves the baseline without ever failing a mutant", name)
		}
	}
}

func compileBridgeTests(t *testing.T, goDir, overlay string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "bridge.test")
	args := []string{"test", "-c", "-o", bin}
	if overlay != "" {
		args = append(args, "-overlay="+overlay)
	}
	args = append(args, "./"+bridgePkg)
	cmd := exec.Command("go", args...)
	cmd.Dir = goDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile the bridge tests (overlay %q): %v\n%s", overlay, err, acsassert.OutputTail(string(out), 20))
	}
	return bin
}

func runBridgeTest(t *testing.T, bin, dir, name string) (string, int) {
	t.Helper()
	cmd := exec.Command(bin, "-test.run", "^"+name+"$", "-test.count=1", "-test.v", "-test.timeout=120s")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("run %s: %v", name, err)
	}
	return string(out), exitErr.ExitCode()
}

func requirePassesUnmutated(t *testing.T, goDir, name string) {
	t.Helper()
	pkgDir := filepath.Join(goDir, filepath.FromSlash(bridgePkg))
	out, code := runBridgeTest(t, compileBridgeTests(t, goDir, ""), pkgDir, name)
	if code != 0 || !strings.Contains(out, "--- PASS: "+name) {
		t.Fatalf("%s does not pass on the unmutated bridge package (exit %d):\n%s", name, code, acsassert.OutputTail(out, 20))
	}
}

func overlayFor(t *testing.T, goDir string, m sourceMutant) string {
	t.Helper()
	original := filepath.Join(goDir, filepath.FromSlash(bridgePkg), m.file)
	raw, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(raw), m.old); n != 1 {
		t.Fatalf("the mutation anchor for %q matches %d times in %s, want 1; the production code drifted and the mutant is stale", m.label, n, m.file)
	}
	mutated := filepath.Join(t.TempDir(), m.file)
	if err := os.WriteFile(mutated, []byte(strings.Replace(string(raw), m.old, m.new, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(t.TempDir(), "overlay.json")
	doc, err := json.Marshal(map[string]map[string]string{"Replace": {original: mutated}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overlay, doc, 0o644); err != nil {
		t.Fatal(err)
	}
	return overlay
}

func requireKillsSourceMutants(t *testing.T, name string, mutants ...sourceMutant) {
	t.Helper()
	goDir := acsassert.GoModuleDir(t)
	requirePassesUnmutated(t, goDir, name)
	pkgDir := filepath.Join(goDir, filepath.FromSlash(bridgePkg))
	for _, m := range mutants {
		t.Run(m.file+"/"+m.label, func(t *testing.T) {
			bin := compileBridgeTests(t, goDir, overlayFor(t, goDir, m))
			out, code := runBridgeTest(t, bin, pkgDir, name)
			if code == 0 {
				t.Errorf("RED: %s still passes when %s", name, m.label)
				return
			}
			if !strings.Contains(out, "--- FAIL: "+name) {
				t.Errorf("%s exits %d on the mutant without a reported failure of its own:\n%s", name, code, acsassert.OutputTail(out, 20))
			}
		})
	}
}

func TestC2504_003_ConcurrentLaunchTestFailsWhenLaunchSwallowsItsOutcome(t *testing.T) {
	requireKillsSourceMutants(t, "TestEngine_ConcurrentLaunch_OnBootRaceFree", swallowedLaunchOutcome)
}

func TestC2504_004_DoctorReportTestFailsOnABrokenDeepProbeOrVerdict(t *testing.T) {
	requireKillsSourceMutants(t, "TestDoctorReport_BoundByDoctor", failedDeepProbe, wrongDoctorVerdict)
}

func TestC2504_005_JoinedPaneTestFailsWhenTheWaitReadsAPlainPane(t *testing.T) {
	requireKillsSourceMutants(t, "TestClaudeCloudDriver_ReadsTheConfirmationFromTheJoinedPaneWhenTheControllerOffersIt", plainPaneWait)
}

func mutatedGoldenDir(t *testing.T, goDir string, mutate func(rows []any) []any) string {
	t.Helper()
	cwd := t.TempDir()
	testdata := filepath.Join(cwd, "testdata")
	if err := os.CopyFS(testdata, os.DirFS(filepath.Join(goDir, filepath.FromSlash(bridgePkg), "testdata"))); err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join(testdata, goldenFileName)
	raw, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	var rows []any
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatalf("%s is not a JSON array: %v", goldenFileName, err)
	}
	if len(rows) < 3 {
		t.Fatalf("%s holds %d rows, too few to mutate", goldenFileName, len(rows))
	}
	doc, err := json.Marshal(mutate(rows))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(golden, doc, 0o644); err != nil {
		t.Fatal(err)
	}
	return cwd
}

func TestC2504_006_GoldenRowsTestFailsOnADuplicatedOrDroppedRow(t *testing.T) {
	const name = "TestLaunchOutcome_GoldenRowsCoverEveryCodeCtxAndFixture"
	goDir := acsassert.GoModuleDir(t)
	requirePassesUnmutated(t, goDir, name)
	bin := compileBridgeTests(t, goDir, "")
	mutations := map[string]func(rows []any) []any{
		"a row is duplicated over its neighbour": func(rows []any) []any {
			rows[2] = rows[1]
			return rows
		},
		"the last row is dropped": func(rows []any) []any { return rows[:len(rows)-1] },
	}
	for label, mutate := range mutations {
		t.Run(label, func(t *testing.T) {
			out, code := runBridgeTest(t, bin, mutatedGoldenDir(t, goDir, mutate), name)
			if code == 0 {
				t.Errorf("RED: %s still passes when %s", name, label)
				return
			}
			if !strings.Contains(out, "--- FAIL: "+name) {
				t.Errorf("%s exits %d on the mutant without a reported failure of its own:\n%s", name, code, acsassert.OutputTail(out, 20))
			}
		})
	}
}

const productionDerivedSrc = `package probe

import "testing"

type outcomeRow struct {
	Code int
	Ctx  string
}

func decodeRows() []outcomeRow { return []outcomeRow{{0, "live"}, {1, "live"}} }

type recorder struct{ reads int }

func (r *recorder) Read() { r.reads++ }

type deps struct{ sink *recorder }

type holder struct{ deps deps }

func newHolder() *holder { return &holder{} }

func (h *holder) run() { h.deps.sink.Read() }

func TestKeyBuiltFromProductionRows(t *testing.T) {
	seen := map[outcomeRow]bool{}
	for _, r := range decodeRows() {
		key := outcomeRow{Code: r.Code, Ctx: r.Ctx}
		if seen[key] {
			t.Errorf("duplicate row %+v", key)
		}
		seen[key] = true
	}
}

func TestRecorderHandedToTheCodeUnderTest(t *testing.T) {
	rec := &recorder{}
	h := newHolder()
	h.deps.sink = rec
	h.run()
	if rec.reads == 0 {
		t.Error("the holder never read")
	}
}
`

func TestC2504_007_KeyBuiltFromProductionRowsIsNotOwnSetup(t *testing.T) {
	probeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(probeDir, "probe_test.go"), []byte(productionDerivedSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	auditCfg, err := testaudit.DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	probeFindings, err := testaudit.Analyze(probeDir, auditCfg)
	if err != nil {
		t.Fatal(err)
	}
	flagged := map[string]string{}
	for _, f := range probeFindings {
		flagged[f.Test] = f.Rule
	}
	if rule, bad := flagged["TestKeyBuiltFromProductionRows"]; bad {
		t.Errorf("RED: a key built from rows that production decoded is flagged %q; fix the detector, not the caller", rule)
	}
}

func TestC2504_008_RecorderHandedToTheCodeUnderTestIsNotOwnSetup(t *testing.T) {
	probeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(probeDir, "probe_test.go"), []byte(productionDerivedSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	auditCfg, err := testaudit.DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	probeFindings, err := testaudit.Analyze(probeDir, auditCfg)
	if err != nil {
		t.Fatal(err)
	}
	flagged := map[string]string{}
	for _, f := range probeFindings {
		flagged[f.Test] = f.Rule
	}
	if rule, bad := flagged["TestRecorderHandedToTheCodeUnderTest"]; bad {
		t.Errorf("RED: a recorder that the test hands to the code under test is flagged %q; fix the detector, not the caller", rule)
	}
}

const ownLiteralsSrc = `package probe

import "testing"

type outcomeRow struct {
	Code int
	Ctx  string
}

type recorder struct{ reads int }

func TestPointerLiteralNeverHandedOut(t *testing.T) {
	rec := &recorder{reads: 1}
	if rec.reads != 1 {
		t.Fatal("never fires")
	}
}

func TestKeyOfOwnLiterals(t *testing.T) {
	seen := map[outcomeRow]bool{}
	key := outcomeRow{Code: 1, Ctx: "live"}
	seen[key] = true
	if !seen[key] {
		t.Fatal("never fires")
	}
}
`

func TestC2504_009_ValuesTheTestWroteFromLiteralsAreStillOwnSetup(t *testing.T) {
	probeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(probeDir, "probe_test.go"), []byte(ownLiteralsSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	auditCfg, err := testaudit.DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	probeFindings, err := testaudit.Analyze(probeDir, auditCfg)
	if err != nil {
		t.Fatal(err)
	}
	flagged := map[string]string{}
	for _, f := range probeFindings {
		flagged[f.Test] = f.Rule
	}
	for _, name := range []string{"TestPointerLiteralNeverHandedOut", "TestKeyOfOwnLiterals"} {
		if flagged[name] != "asserts-own-setup" {
			t.Errorf("%s asserts only values it wrote from literals and is not flagged asserts-own-setup: %q", name, flagged[name])
		}
	}
}

func TestC2504_010_RewritesKeepEveryBridgeExportNamed(t *testing.T) {
	goDir := acsassert.GoModuleDir(t)
	dir := filepath.Join(goDir, filepath.FromSlash(bridgePkg))
	syms, err := apicover.Enumerate(context.Background(), dir)
	if err != nil {
		t.Fatalf("enumerate %s: %v", bridgePkg, err)
	}
	named, err := apicover.NamesReferencedInTests(context.Background(), dir)
	if err != nil {
		t.Fatalf("scan the tests of %s: %v", bridgePkg, err)
	}
	for _, s := range apicover.Classify(syms, named, nil).Uncovered {
		t.Errorf("RED: %s (%s) is named by no test, so the apicover gate of ./internal/bridge fails", s.Name, s.File)
	}
}
