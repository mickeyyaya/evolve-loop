//go:build acs

package cycle1829

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unicode"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const laneModule = "github.com/mickeyyaya/evolve-loop/go"

var treeReadingPackagesAwaitingSelection = []string{
	"cmd/evolve", "internal/bridge", "internal/changedpkgs", "internal/core", "internal/cycleoutcome",
	"internal/inboxmover", "internal/inboxmover/lifecycle", "internal/phaseobserver", "internal/phases/audit",
	"internal/phases/runner", "internal/phases/ship", "internal/reachabilityprobe", "internal/subagent",
}

var packagesTooSlowToRunWhole = []string{"cmd/evolve", "internal/core"}

const (
	verdictSeamPackage   = "internal/phases/runner"
	verdictSeamTest      = "TestVerdictEngine_OneConstructionSite"
	secondVerdictSite    = "internal/secondverdictsite/site.go"
	detectorTest         = "TestPackages_HoldEveryTestThatReadsTheWholeTree"
	awaitingSelectionTag = "awaitsTestLevelSelection"
)

const secondVerdictSiteSource = `package secondverdictsite

import "github.com/mickeyyaya/evolve-loop/go/internal/phases/runner/verdict"

func engine() any { return verdict.New(verdict.Options{}) }
`

const stubFailingWhenSelected = `package stub

import "testing"

func TestLaneTreeStub(t *testing.T) {}

func laneTreeSelected(t *testing.T) {
	t.Helper()
	t.Fatal("ran: the repo-contract pack selected this test")
}
`

func stubSleepingWhenRun(pidDir string) string {
	return fmt.Sprintf(`package stub

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestLaneTreeStub(t *testing.T) { laneTreeSelected(t) }

func laneTreeSelected(t *testing.T) {
	if err := os.WriteFile(filepath.Join(%s, strconv.Itoa(os.Getpid())), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Minute)
}
`, strconv.Quote(pidDir))
}

type packRecord struct {
	ran        bool
	reds       []string
	diagnostic string
	err        error
	findings   []string
}

func buildFloorOver(ctx context.Context, root string) packRecord {
	var rec packRecord
	productionPack := func(ctx context.Context, root string) ([]string, string, error) {
		rec.ran = true
		rec.reds, rec.diagnostic, rec.err = ship.RunRepoContractPack(ctx, root)
		return rec.reds, rec.diagnostic, rec.err
	}
	findings := core.RepoContractFloorChecks(productionPack)(ctx, core.ReviewInput{Worktree: root, ProjectRoot: root})
	rec.findings = findings
	return rec
}

func redsIn(reds []string, pkg string) []string {
	var in []string
	for _, red := range reds {
		if strings.HasPrefix(red, laneModule+"/"+pkg+".") {
			in = append(in, red)
		}
	}
	return in
}

type laneFile struct {
	rel     string
	source  string
	defines []string
}

func writeLaneTree(realGo, root, stubSource string, extra ...laneFile) (map[string]int, error) {
	files := []laneFile{{rel: "go.mod", source: "module " + laneModule + "\n\ngo 1.23\n"}}
	dirs, err := packageDirs(realGo)
	if err != nil {
		return nil, err
	}
	for _, dir := range dirs {
		files = append(files, laneFile{rel: path.Join(dir, "zz_lane_stub_test.go"), source: stubSource})
	}
	mirrors, mirrored, err := mirrorFiles(realGo, extra)
	if err != nil {
		return nil, err
	}
	for _, file := range slices.Concat(files, mirrors, extra) {
		target := filepath.Join(root, "go", filepath.FromSlash(file.rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(target, []byte(file.source), 0o644); err != nil {
			return nil, err
		}
	}
	return mirrored, nil
}

func packageDirs(realGo string) ([]string, error) {
	dirs := map[string]bool{}
	err := filepath.WalkDir(realGo, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(realGo, p)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() && rel != "." && notAPackageTree(rel, entry.Name()) {
			return filepath.SkipDir
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			dirs[path.Dir(rel)] = true
		}
		return nil
	})
	return slices.Sorted(maps.Keys(dirs)), err
}

func notAPackageTree(rel, name string) bool {
	return rel == "acs" || name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

func mirrorFiles(realGo string, extra []laneFile) ([]laneFile, map[string]int, error) {
	var files []laneFile
	mirrored := map[string]int{}
	for _, pkg := range treeReadingPackagesAwaitingSelection {
		names, err := testNames(filepath.Join(realGo, filepath.FromSlash(pkg)))
		if err != nil {
			return nil, nil, err
		}
		names = slices.DeleteFunc(names, func(name string) bool { return definedBy(extra, pkg, name) })
		var source strings.Builder
		source.WriteString("package stub\n\nimport \"testing\"\n")
		for _, name := range names {
			fmt.Fprintf(&source, "\nfunc %s(t *testing.T) { laneTreeSelected(t) }\n", name)
		}
		files = append(files, laneFile{rel: path.Join(pkg, "zz_lane_mirror_test.go"), source: source.String()})
		mirrored[pkg] = len(names) + countDefinedIn(extra, pkg)
	}
	return files, mirrored, nil
}

func definedBy(extra []laneFile, pkg, name string) bool {
	for _, file := range extra {
		if path.Dir(file.rel) == pkg && slices.Contains(file.defines, name) {
			return true
		}
	}
	return false
}

func countDefinedIn(extra []laneFile, pkg string) int {
	count := 0
	for _, file := range extra {
		if path.Dir(file.rel) == pkg {
			count += len(file.defines)
		}
	}
	return count
}

func testNames(dir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	if err != nil || len(files) == 0 {
		return nil, fmt.Errorf("no test files in %s (%v)", dir, err)
	}
	names := map[string]bool{}
	for _, file := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, decl := range parsed.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && isGoTest(fn) {
				names[fn.Name.Name] = true
			}
		}
	}
	return slices.Sorted(maps.Keys(names)), nil
}

func isGoTest(fn *ast.FuncDecl) bool {
	rest, ok := strings.CutPrefix(fn.Name.Name, "Test")
	if !ok || fn.Recv != nil || fn.Name.Name == "TestMain" || fn.Type.Params.NumFields() != 1 {
		return false
	}
	return rest == "" || !unicode.IsLower(rune(rest[0]))
}

func realVerdictSeamTest(realGo string) (laneFile, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath.Join(realGo, filepath.FromSlash(verdictSeamPackage), "verdict_engine_test.go"), nil, parser.SkipObjectResolution)
	if err != nil {
		return laneFile{}, err
	}
	funcs := map[string]*ast.FuncDecl{}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil {
			funcs[fn.Name.Name] = fn
		}
	}
	if funcs[verdictSeamTest] == nil {
		return laneFile{}, fmt.Errorf("%s no longer declares %s", verdictSeamPackage, verdictSeamTest)
	}
	var body bytes.Buffer
	for _, name := range calledFrom(funcs, verdictSeamTest) {
		if err := format.Node(&body, fset, funcs[name]); err != nil {
			return laneFile{}, err
		}
		body.WriteString("\n\n")
	}
	imports, err := stdlibImportsUsed(file, funcs, calledFrom(funcs, verdictSeamTest))
	if err != nil {
		return laneFile{}, err
	}
	source := "package stub\n\nimport (\n" + imports + ")\n\n" + body.String()
	return laneFile{rel: path.Join(verdictSeamPackage, "zz_real_verdict_seam_test.go"), source: source, defines: []string{verdictSeamTest}}, nil
}

func calledFrom(funcs map[string]*ast.FuncDecl, root string) []string {
	reached := []string{root}
	for i := 0; i < len(reached); i++ {
		ast.Inspect(funcs[reached[i]].Body, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && funcs[id.Name] != nil && !slices.Contains(reached, id.Name) {
				reached = append(reached, id.Name)
			}
			return true
		})
	}
	return reached
}

func stdlibImportsUsed(file *ast.File, funcs map[string]*ast.FuncDecl, names []string) (string, error) {
	byLocalName := map[string]string{}
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return "", fmt.Errorf("import %s: %w", spec.Path.Value, err)
		}
		local := path.Base(importPath)
		if spec.Name != nil {
			local = spec.Name.Name
		}
		byLocalName[local] = importPath
	}
	used := map[string]bool{}
	for _, name := range names {
		ast.Inspect(funcs[name], func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if pkg, ok := sel.X.(*ast.Ident); ok && byLocalName[pkg.Name] != "" {
					used[byLocalName[pkg.Name]] = true
				}
			}
			return true
		})
	}
	var lines strings.Builder
	for _, importPath := range slices.Sorted(maps.Keys(used)) {
		if strings.Contains(strings.Split(importPath, "/")[0], ".") {
			return "", fmt.Errorf("%s now needs %s, which a lane-tree fixture cannot carry", verdictSeamTest, importPath)
		}
		fmt.Fprintf(&lines, "\t%q\n", importPath)
	}
	return lines.String(), nil
}

var secondVerdictSiteRun struct {
	sync.Once
	rec      packRecord
	mirrored map[string]int
	err      error
}

func packRunWithASecondVerdictSite(t *testing.T) (packRecord, map[string]int) {
	t.Helper()
	realGo := filepath.Join(acsassert.RepoRoot(t), "go")
	secondVerdictSiteRun.Do(func() {
		root := t.TempDir()
		seam, err := realVerdictSeamTest(realGo)
		if err != nil {
			secondVerdictSiteRun.err = err
			return
		}
		second := laneFile{rel: secondVerdictSite, source: secondVerdictSiteSource}
		secondVerdictSiteRun.mirrored, secondVerdictSiteRun.err = writeLaneTree(realGo, root, stubFailingWhenSelected, seam, second)
		if secondVerdictSiteRun.err == nil {
			secondVerdictSiteRun.rec = buildFloorOver(context.Background(), root)
		}
	})
	if secondVerdictSiteRun.err != nil {
		t.Fatalf("lane-tree fixture: %v", secondVerdictSiteRun.err)
	}
	return secondVerdictSiteRun.rec, secondVerdictSiteRun.mirrored
}

type cancelledRun struct {
	rec       packRecord
	started   bool
	pids      []int
	survivors []int
}

var cancelledPack struct {
	sync.Once
	run cancelledRun
	err error
}

func packCancelledMidRun(t *testing.T) cancelledRun {
	t.Helper()
	realGo := filepath.Join(acsassert.RepoRoot(t), "go")
	cancelledPack.Do(func() {
		root, pidDir := t.TempDir(), t.TempDir()
		t.Cleanup(func() { killRecorded(t, pidDir) })
		if _, err := writeLaneTree(realGo, root, stubSleepingWhenRun(pidDir)); err != nil {
			cancelledPack.err = err
			return
		}
		cancelledPack.run = cancelOnceATestBinaryRuns(root, pidDir)
		killRecorded(t, pidDir)
	})
	if cancelledPack.err != nil {
		t.Fatalf("lane-tree fixture: %v", cancelledPack.err)
	}
	return cancelledPack.run
}

func cancelOnceATestBinaryRuns(root, pidDir string) cancelledRun {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan packRecord, 1)
	go func() { done <- buildFloorOver(ctx, root) }()
	for len(recordedPIDs(pidDir)) == 0 {
		select {
		case rec := <-done:
			return cancelledRun{rec: rec}
		case <-time.After(100 * time.Millisecond):
		}
	}
	cancel()
	rec := <-done
	pids := recordedPIDs(pidDir)
	var survivors []int
	for _, pid := range pids {
		if !exitsAfterTheCancel(pid) {
			survivors = append(survivors, pid)
		}
	}
	return cancelledRun{rec: rec, started: true, pids: pids, survivors: survivors}
}

func recordedPIDs(pidDir string) []int {
	entries, err := os.ReadDir(pidDir)
	if err != nil {
		return nil
	}
	var pids []int
	for _, entry := range entries {
		if pid, err := strconv.Atoi(entry.Name()); err == nil {
			pids = append(pids, pid)
		}
	}
	return pids
}

func exitsAfterTheCancel(pid int) bool {
	for range 50 {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

func killRecorded(t *testing.T, pidDir string) {
	for _, pid := range recordedPIDs(pidDir) {
		if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			t.Logf("could not kill the lane tree's test binary %d: %v", pid, err)
		}
	}
}

func recordsAwaitingSelection(t *testing.T, packagesTest string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), packagesTest, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	var records []string
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok || !slices.ContainsFunc(spec.Names, func(id *ast.Ident) bool { return id.Name == "readsTheTreeOutsideThePack" }) {
			return true
		}
		for _, value := range spec.Values {
			if lit, ok := value.(*ast.CompositeLit); ok {
				records = append(records, awaitingEntries(lit)...)
			}
		}
		return false
	})
	return records
}

func awaitingEntries(lit *ast.CompositeLit) []string {
	var records []string
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, _ := kv.Key.(*ast.BasicLit)
		reason, _ := kv.Value.(*ast.Ident)
		pkg := ""
		if key != nil {
			pkg, _ = strconv.Unquote(key.Value)
		}
		if slices.Contains(treeReadingPackagesAwaitingSelection, pkg) || (reason != nil && reason.Name == awaitingSelectionTag) {
			records = append(records, pkg)
		}
	}
	return records
}

func detectorPasses(goDir string) (bool, string) {
	cmd := exec.Command("go", "test", "-json", "-count=1", "-run", "^"+detectorTest+"$", "./internal/repocontract")
	cmd.Dir = goDir
	out, runErr := cmd.CombinedOutput()
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		var ev struct{ Action, Test string }
		if json.Unmarshal(sc.Bytes(), &ev) == nil && ev.Action == "pass" && ev.Test == detectorTest {
			return true, ""
		}
	}
	return false, fmt.Sprintf("go test: %v\n%s", runErr, tail(string(out)))
}

func tail(s string) string {
	const keep = 3000
	if len(s) <= keep {
		return s
	}
	return "…" + s[len(s)-keep:]
}

func packCancelledWhileATestBinaryRuns(t *testing.T) cancelledRun {
	t.Helper()
	run := packCancelledMidRun(t)
	if !run.started {
		t.Fatalf("the lane tree's pack ended before any test binary started (err %v, reds %v); the fixture never reached the cancel", run.rec.err, run.rec.reds)
	}
	return run
}

func TestC1829_001_CancelledPackLeavesNoTestBinaryRunning(t *testing.T) {
	run := packCancelledWhileATestBinaryRuns(t)
	if len(run.survivors) > 0 {
		t.Errorf("RED: the build floor cancelled the repo-contract pack, yet test binaries %v (of %v) were still running after it returned — the cancel killed the go command and orphaned its test binaries until their own -test.timeout", run.survivors, run.pids)
	}
}

func TestC1829_002_CancelledPackStillNamesNothing(t *testing.T) {
	run := packCancelledWhileATestBinaryRuns(t)
	if !run.rec.ran || run.rec.err == nil {
		t.Errorf("a cancelled pack must still return its exit error, so ship's gate classes it ambiguous rather than green; ran=%v err=%v", run.rec.ran, run.rec.err)
	}
	if len(run.rec.reds) > 0 || len(run.rec.findings) > 0 {
		t.Errorf("a cancel names no test: RunRepoContractPack reds %v, build-floor findings %v", run.rec.reds, run.rec.findings)
	}
}

func TestC1829_003_SecondVerdictConstructionSiteRedsTheBuildFloor(t *testing.T) {
	rec, _ := packRunWithASecondVerdictSite(t)
	want := laneModule + "/" + verdictSeamPackage + "." + verdictSeamTest
	if !slices.Contains(rec.reds, want) {
		t.Fatalf("RED: a lane tree with a second verdict.New( call in %s passed the build floor's repo-contract pack without %s red (ran=%v, err %v, reds %v) — only main's CI would find it", secondVerdictSite, want, rec.ran, rec.err, rec.reds)
	}
	if !strings.Contains(rec.diagnostic, secondVerdictSite) {
		t.Errorf("the red must come from the second construction site %s; the failing tests' output was:\n%s", secondVerdictSite, tail(rec.diagnostic))
	}
	if len(rec.findings) == 0 || !strings.Contains(rec.findings[0], verdictSeamTest) {
		t.Errorf("the build floor must refuse the handoff naming %s; findings %v", verdictSeamTest, rec.findings)
	}
}

func TestC1829_004_TreeReadersAwaitingSelectionMoveIntoThePack(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	if records := recordsAwaitingSelection(t, filepath.Join(goDir, "internal", "repocontract", "packages_test.go")); len(records) > 0 {
		t.Errorf("RED: readsTheTreeOutsideThePack still records %v as waiting for test-level selection", records)
	}
	if passed, out := detectorPasses(goDir); !passed {
		t.Errorf("%s must pass on this tree, so every test that reads the whole tree is in the pack:\n%s", detectorTest, out)
	}
	rec, _ := packRunWithASecondVerdictSite(t)
	for _, pkg := range treeReadingPackagesAwaitingSelection {
		if len(redsIn(rec.reds, pkg)) == 0 {
			t.Errorf("RED: the build floor's repo-contract pack ran no test of %s, whose tests read the whole tree, so a lane that touches neither it nor an importer can break one and only main's CI finds out", pkg)
		}
	}
}

func TestC1829_005_SlowPackagesRunByTestNameWithinTheFloorDeadline(t *testing.T) {
	rec, mirrored := packRunWithASecondVerdictSite(t)
	for _, pkg := range packagesTooSlowToRunWhole {
		if ran := len(redsIn(rec.reds, pkg)); ran == 0 || ran >= mirrored[pkg] {
			t.Errorf("RED: the pack must run %s's tree-reading tests by name — some, never all, of its %d tests; it ran %d", pkg, mirrored[pkg], ran)
		}
	}
	live := buildFloorOver(context.Background(), acsassert.RepoRoot(t))
	if !live.ran || live.err != nil || len(live.reds) > 0 {
		t.Errorf("the repo-contract pack must finish green on this lane's own tree inside the build floor's deadline; ran=%v err=%v reds=%v findings=%v\n%s", live.ran, live.err, live.reds, live.findings, tail(live.diagnostic))
	}
}
