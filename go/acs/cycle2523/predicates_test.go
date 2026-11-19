//go:build acs

package cycle2523

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/testaudit"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const shardRel = "testdata/testaudit-baseline.json"

const weakSource = "package %PKG%\n\nimport \"testing\"\n\nfunc %TEST%(t *testing.T) {\n\t_ = 1\n}\n"

const cleanSource = "package %PKG%\n\nimport (\n\t\"strings\"\n\t\"testing\"\n)\n\nfunc %TEST%(t *testing.T) {\n\tif got := strings.ToUpper(\"a\"); got != \"A\" {\n\t\tt.Errorf(\"got %q\", got)\n\t}\n}\n"

var (
	buildOnce sync.Once
	builtBin  string
	buildDir  string
	buildErr  error
)

func TestMain(m *testing.M) {
	code := m.Run()
	if buildDir != "" {
		_ = os.RemoveAll(buildDir)
	}
	os.Exit(code)
}

func evolveCLI(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		moduleDir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
		if err != nil {
			buildErr = err
			return
		}
		if buildDir, err = os.MkdirTemp("", "evolve-acs-2523-"); err != nil {
			buildErr = err
			return
		}
		builtBin = filepath.Join(buildDir, "evolve")
		out, errOut, code, runErr := acsassert.SubprocessOutput("go", "-C", moduleDir, "build", "-o", builtBin, "./cmd/evolve")
		if runErr != nil || code != 0 {
			buildErr = errors.New(acsassert.OutputTail(out+errOut, 20))
		}
	})
	if buildErr != nil {
		t.Fatalf("build evolve: %v", buildErr)
	}
	return builtBin
}

func runIn(t *testing.T, dir, name string, args ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode()
	}
	t.Fatalf("run %s in %s: %v", name, dir, err)
	return "", -1
}

func verbIn(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	return runIn(t, dir, evolveCLI(t), append([]string{"test", "audit"}, args...)...)
}

func source(template, pkg, test string) string {
	return strings.NewReplacer("%PKG%", pkg, "%TEST%", test).Replace(template)
}

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func readRows(t *testing.T, path string) []testaudit.Finding {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read shard: %v", err)
	}
	var shard struct {
		Rows []testaudit.Finding `json:"rows"`
	}
	if err := json.Unmarshal(raw, &shard); err != nil {
		t.Fatalf("shard %s is not JSON rows: %v", path, err)
	}
	return shard.Rows
}

func withDir(t *testing.T, dir string, body func()) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(prev); err != nil {
			t.Fatal(err)
		}
	}()
	body()
}

func moduleCopy(t *testing.T) string {
	t.Helper()
	src := acsassert.GoModuleDir(t)
	files, err := acsassert.WalkModule(src, func(rel string) bool {
		if rel == "evolve" || strings.HasPrefix(rel, "vendor/") {
			return false
		}
		return strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, ".json") || rel == "go.mod" || rel == "go.sum"
	})
	if err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	for _, rel := range files {
		raw, err := os.ReadFile(filepath.Join(src, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		writeTree(t, dst, map[string]string{rel: string(raw)})
	}
	return dst
}

func treeVerdict(t *testing.T, copyRoot string) (string, bool) {
	t.Helper()
	out, code := runIn(t, copyRoot, "go", "test", "-count=1", "./internal/testaudit")
	return out, code == 0
}

func mustPass(t *testing.T, copyRoot, stage string) {
	t.Helper()
	if out, ok := treeVerdict(t, copyRoot); !ok {
		t.Fatalf("RED: %s: the testaudit package fails on the module copy:\n%s", stage, acsassert.OutputTail(out, 25))
	}
}

func mustFailNaming(t *testing.T, copyRoot, stage string, names ...string) {
	t.Helper()
	out, ok := treeVerdict(t, copyRoot)
	if ok {
		t.Errorf("RED: %s: the testaudit package still passes", stage)
		return
	}
	for _, name := range names {
		if !strings.Contains(out, name) {
			t.Errorf("RED: %s: the failure does not name %q:\n%s", stage, name, acsassert.OutputTail(out, 25))
		}
	}
}

func TestC2523_001_RebaselineWritesThePackageShard(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"pkg/x/x_test.go":         source(weakSource, "x", "TestFirstWeak"),
		"pkg/clean/clean_test.go": source(cleanSource, "clean", "TestCleanOnly"),
	})
	out, code := verbIn(t, root, "--rebaseline", "pkg/x")
	if code != 0 {
		t.Fatalf("RED: evolve test audit --rebaseline pkg/x exit=%d:\n%s", code, acsassert.OutputTail(out, 15))
	}
	shard := filepath.Join(root, "pkg", "x", filepath.FromSlash(shardRel))
	want := testaudit.Finding{Package: "pkg/x", Test: "TestFirstWeak", Rule: "no-assertion"}
	if rows := readRows(t, shard); len(rows) != 1 || rows[0] != want {
		t.Errorf("RED: shard rows = %v, want [%v]", rows, want)
	}
	before, err := os.ReadFile(shard)
	if err != nil {
		t.Fatal(err)
	}
	if again, code := verbIn(t, root, "--rebaseline", "pkg/x"); code != 0 {
		t.Errorf("RED: a second rebaseline with no change exit=%d:\n%s", code, acsassert.OutputTail(again, 15))
	}
	after, err := os.ReadFile(shard)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("RED: a rebaseline with no change rewrote the shard:\n%s\n--\n%s", before, after)
	}
	if _, code := verbIn(t, root, "--rebaseline", "pkg/clean"); code != 0 {
		t.Errorf("RED: rebaseline of a package with no finding exit=%d", code)
	}
	if _, err := os.Stat(filepath.Join(root, "pkg", "clean", filepath.FromSlash(shardRel))); err == nil {
		t.Errorf("RED: a package with no finding got a shard")
	}
	cfg, err := testaudit.DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	withDir(t, root, func() {
		if err := testaudit.CheckShard("pkg/x", cfg); err != nil {
			t.Errorf("RED: CheckShard rejects the shard that the verb wrote: %v", err)
		}
	})
}

func TestC2523_002_ASecondRunWithOneMoreFindingRefusesAndNamesIt(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"pkg/x/x_test.go": source(weakSource, "x", "TestFirstWeak")})
	if out, code := verbIn(t, root, "--rebaseline", "pkg/x"); code != 0 {
		t.Fatalf("RED: first rebaseline exit=%d:\n%s", code, acsassert.OutputTail(out, 15))
	}
	shard := filepath.Join(root, "pkg", "x", filepath.FromSlash(shardRel))
	before, err := os.ReadFile(shard)
	if err != nil {
		t.Fatal(err)
	}
	writeTree(t, root, map[string]string{"pkg/x/y_test.go": source(weakSource, "x", "TestSecondWeak")})
	out, code := verbIn(t, root, "--rebaseline", "pkg/x")
	if code == 0 {
		t.Errorf("RED: the rebaseline accepted a shard that grew by one finding")
	}
	if !strings.Contains(out, "TestSecondWeak") {
		t.Errorf("RED: the refusal does not name the new finding TestSecondWeak:\n%s", acsassert.OutputTail(out, 15))
	}
	after, err := os.ReadFile(shard)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("RED: a refused rebaseline changed the shard:\n%s\n--\n%s", before, after)
	}
}

func TestC2523_003_ARebaselineAfterAFixShrinksTheShard(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"pkg/x/x_test.go": source(weakSource, "x", "TestFirstWeak"),
		"pkg/x/y_test.go": source(weakSource, "x", "TestSecondWeak"),
	})
	if out, code := verbIn(t, root, "--rebaseline", "pkg/x"); code != 0 {
		t.Fatalf("RED: first rebaseline exit=%d:\n%s", code, acsassert.OutputTail(out, 15))
	}
	shard := filepath.Join(root, "pkg", "x", filepath.FromSlash(shardRel))
	if rows := readRows(t, shard); len(rows) != 2 {
		t.Fatalf("RED: first shard has %d rows, want 2: %v", len(rows), rows)
	}
	writeTree(t, root, map[string]string{"pkg/x/y_test.go": source(cleanSource, "x", "TestSecondFixed")})
	if out, code := verbIn(t, root, "--rebaseline", "pkg/x"); code != 0 {
		t.Fatalf("RED: a rebaseline after a fix exit=%d:\n%s", code, acsassert.OutputTail(out, 15))
	}
	want := testaudit.Finding{Package: "pkg/x", Test: "TestFirstWeak", Rule: "no-assertion"}
	if rows := readRows(t, shard); len(rows) != 1 || rows[0] != want {
		t.Errorf("RED: shrunk shard rows = %v, want [%v]", rows, want)
	}
	writeTree(t, root, map[string]string{"pkg/x/x_test.go": source(cleanSource, "x", "TestFirstFixed")})
	if out, code := verbIn(t, root, "--rebaseline", "pkg/x"); code != 0 {
		t.Fatalf("RED: the last rebaseline exit=%d:\n%s", code, acsassert.OutputTail(out, 15))
	}
	if _, err := os.Stat(shard); err == nil {
		if rows := readRows(t, shard); len(rows) != 0 {
			t.Errorf("RED: a package with no finding keeps rows %v", rows)
		}
	}
}

func TestC2523_004_ARebaselineOfAMissingPackageFailsAndNamesIt(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"pkg/x/x_test.go": source(weakSource, "x", "TestFirstWeak")})
	if out, code := verbIn(t, root, "--rebaseline", "pkg/x"); code != 0 {
		t.Fatalf("RED: a rebaseline of a present package exit=%d:\n%s", code, acsassert.OutputTail(out, 15))
	}
	shard := filepath.Join(root, "pkg", "x", filepath.FromSlash(shardRel))
	before, err := os.ReadFile(shard)
	if err != nil {
		t.Fatalf("RED: the rebaseline wrote no shard: %v", err)
	}
	out, code := verbIn(t, root, "--rebaseline", "pkg/absent")
	if code == 0 {
		t.Errorf("RED: a rebaseline of a package that does not exist exit=0:\n%s", acsassert.OutputTail(out, 15))
	}
	if !strings.Contains(out, "pkg/absent") {
		t.Errorf("RED: the failure does not name pkg/absent:\n%s", acsassert.OutputTail(out, 15))
	}
	if _, err := os.Stat(filepath.Join(root, "pkg", "absent")); err == nil {
		t.Errorf("RED: the rebaseline of a missing package created a directory")
	}
	after, err := os.ReadFile(shard)
	if err != nil || string(before) != string(after) {
		t.Errorf("RED: a failed call changed the shard of another package: %v", err)
	}
}

type plant struct {
	name string
	dir  string
	file string
	pkg  string
	test string
	tag  string
}

func TestC2523_005_ANewWeakTestInAnyPackageFailsTheTreeTest(t *testing.T) {
	copyRoot := moduleCopy(t)
	mustPass(t, copyRoot, "before any plant")
	plants := []plant{
		{name: "new-package", dir: "internal/zzauditprobe", file: "weak_test.go", pkg: "zzauditprobe", test: "TestZZWeakNewPackage"},
		{name: "package-with-a-shard", dir: "cmd/evolve", file: "zz_weak_probe_test.go", pkg: "main", test: "TestZZWeakMain"},
		{name: "protected-package", dir: "internal/guards", file: "zz_weak_probe_test.go", pkg: "guards", test: "TestZZWeakGuards"},
		{name: "foreign-predicate-package", dir: "acs/cycle9999", file: "zz_weak_probe_test.go", pkg: "cycle9999", test: "TestZZWeakPredicate", tag: "//go:build acs\n\n"},
	}
	for _, p := range plants {
		path := filepath.Join(copyRoot, filepath.FromSlash(p.dir), p.file)
		writeTree(t, copyRoot, map[string]string{filepath.ToSlash(filepath.Join(p.dir, p.file)): p.tag + source(weakSource, p.pkg, p.test)})
		mustFailNaming(t, copyRoot, p.name+" with the weak test", p.dir, p.test, "no-assertion")
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		mustPass(t, copyRoot, p.name+" after the weak test is deleted")
	}
}

func TestC2523_006_ABaselineRowWhoseTestIsGoneFailsTheTreeTest(t *testing.T) {
	copyRoot := moduleCopy(t)
	mustPass(t, copyRoot, "before any plant")
	row := func(pkg string) string {
		return `{"rows":[{"package":"` + pkg + `","test":"TestZZVanished","rule":"no-assertion"}]}` + "\n"
	}
	t.Run("clean-package-with-a-stale-shard", func(t *testing.T) {
		dir := "internal/zzstaleclean"
		writeTree(t, copyRoot, map[string]string{
			dir + "/clean_test.go": source(cleanSource, "zzstaleclean", "TestZZStillHere"),
			dir + "/" + shardRel:   row(dir),
		})
		mustFailNaming(t, copyRoot, "stale row in a clean package", dir, "TestZZVanished", "no-assertion")
		if err := os.Remove(filepath.Join(copyRoot, filepath.FromSlash(dir), filepath.FromSlash(shardRel))); err != nil {
			t.Fatal(err)
		}
		mustPass(t, copyRoot, "stale shard deleted")
	})
	t.Run("package-whose-tests-were-all-deleted", func(t *testing.T) {
		dir := "internal/zznotests"
		writeTree(t, copyRoot, map[string]string{
			dir + "/lib.go":      "package zznotests\n",
			dir + "/" + shardRel: row(dir),
		})
		mustFailNaming(t, copyRoot, "stale row in a package with no test file", dir, "TestZZVanished", "no-assertion")
		if err := os.Remove(filepath.Join(copyRoot, filepath.FromSlash(dir), filepath.FromSlash(shardRel))); err != nil {
			t.Fatal(err)
		}
		mustPass(t, copyRoot, "stale shard deleted")
	})
	t.Run("extra-row-in-an-existing-shard", func(t *testing.T) {
		path := filepath.Join(copyRoot, "cmd", "evolve", filepath.FromSlash(shardRel))
		original, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("RED: cmd/evolve has no shard to extend: %v", err)
		}
		var doc map[string]any
		if err := json.Unmarshal(original, &doc); err != nil {
			t.Fatal(err)
		}
		rows, _ := doc["rows"].([]any)
		doc["rows"] = append(rows, map[string]any{"package": "cmd/evolve", "test": "TestZZVanished", "rule": "no-assertion"})
		extended, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, extended, 0o644); err != nil {
			t.Fatal(err)
		}
		mustFailNaming(t, copyRoot, "stale row in the cmd/evolve shard", "cmd/evolve", "TestZZVanished", "no-assertion")
		if err := os.WriteFile(path, original, 0o644); err != nil {
			t.Fatal(err)
		}
		mustPass(t, copyRoot, "cmd/evolve shard restored")
	})
}

func TestC2523_007_TheShardsOfTheTautologyAndStructureItemsExistAtHead(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := acsassert.GoModuleDir(t)
	cfg, err := testaudit.DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	rules := testaudit.RuleIDs()
	for _, pkg := range []string{"cmd/evolve", "internal/core"} {
		path := filepath.Join(goDir, filepath.FromSlash(pkg), filepath.FromSlash(shardRel))
		if !acsassert.FileExists(t, path) {
			t.Errorf("RED: %s has no shard", pkg)
			continue
		}
		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", path); code != 1 {
			t.Errorf("RED: the shard of %s is git-ignored (check-ignore exit %d) and is dropped at ship", pkg, code)
		}
		rows := readRows(t, path)
		if len(rows) == 0 {
			t.Errorf("RED: the shard of %s holds no row", pkg)
		}
		for _, r := range rows {
			if r.Package != pkg || !slices.Contains(rules, r.Rule) || r.Test == "" {
				t.Errorf("RED: shard of %s holds a malformed row %+v", pkg, r)
			}
		}
		withDir(t, goDir, func() {
			if err := testaudit.CheckShard(pkg, cfg); err != nil {
				t.Errorf("RED: the shard of %s does not match the tree: %v", pkg, err)
			}
		})
	}
}

const centralRel = "internal/testaudit/unwritable-baselines.json"

func TestC2523_008_AnEqualCountSwapOfFindingsRefusesAndNamesTheNewOne(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"pkg/x/x_test.go": source(weakSource, "x", "TestFirstWeak")})
	if out, code := verbIn(t, root, "--rebaseline", "pkg/x"); code != 0 {
		t.Fatalf("RED: first rebaseline exit=%d:\n%s", code, acsassert.OutputTail(out, 15))
	}
	shard := filepath.Join(root, "pkg", "x", filepath.FromSlash(shardRel))
	before, err := os.ReadFile(shard)
	if err != nil {
		t.Fatalf("RED: the rebaseline wrote no shard: %v", err)
	}
	writeTree(t, root, map[string]string{
		"pkg/x/x_test.go": source(cleanSource, "x", "TestFirstFixed"),
		"pkg/x/y_test.go": source(weakSource, "x", "TestSwappedInWeak"),
	})
	out, code := verbIn(t, root, "--rebaseline", "pkg/x")
	if code == 0 {
		t.Errorf("RED: the rebaseline accepted one fixed and one new weak test (same row count)")
	}
	if !strings.Contains(out, "TestSwappedInWeak") {
		t.Errorf("RED: the refusal does not name TestSwappedInWeak:\n%s", acsassert.OutputTail(out, 15))
	}
	after, err := os.ReadFile(shard)
	if err != nil || string(before) != string(after) {
		t.Errorf("RED: a refused rebaseline changed the shard: %v\n%s\n--\n%s", err, before, after)
	}
}

func TestC2523_009_AnEqualCountSwapInACentralPackageRefusesAndNamesTheNewOne(t *testing.T) {
	root := t.TempDir()
	central := `{"packages":["pkg/x"],"rows":[{"package":"pkg/x","test":"TestFirstFixed","rule":"no-assertion"}]}`
	writeTree(t, root, map[string]string{
		centralRel:        central,
		"pkg/x/x_test.go": source(cleanSource, "x", "TestFirstFixed"),
		"pkg/x/y_test.go": source(weakSource, "x", "TestSwappedInWeak"),
	})
	out, code := verbIn(t, root, "--rebaseline", "pkg/x")
	if code == 0 {
		t.Errorf("RED: the rebaseline of a central package accepted one fixed and one new weak test")
	}
	if !strings.Contains(out, "TestSwappedInWeak") {
		t.Errorf("RED: the refusal does not name TestSwappedInWeak:\n%s", acsassert.OutputTail(out, 15))
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(centralRel)))
	if err != nil || strings.Contains(string(raw), "TestSwappedInWeak") {
		t.Errorf("RED: the central file admits the new row: %v\n%s", err, raw)
	}
}

func TestC2523_010_GrowthFromAnEmptyCentralPackageRefusesAndNamesTheNewFinding(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		centralRel:          `{"packages":["pkg/x"],"rows":[]}`,
		"pkg/x/x_test.go":   source(weakSource, "x", "TestFirstWeakInCentral"),
		"pkg/ok/ok_test.go": source(cleanSource, "ok", "TestOnlyClean"),
	})
	if out, code := verbIn(t, root, "--rebaseline", "pkg/ok"); code != 0 {
		t.Fatalf("RED: a rebaseline of a clean package exit=%d:\n%s", code, acsassert.OutputTail(out, 15))
	}
	out, code := verbIn(t, root, "--rebaseline", "pkg/x")
	if code == 0 {
		t.Errorf("RED: a central package went from zero rows to one without a refusal")
	}
	if !strings.Contains(out, "TestFirstWeakInCentral") {
		t.Errorf("RED: the refusal does not name TestFirstWeakInCentral:\n%s", acsassert.OutputTail(out, 15))
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(centralRel)))
	if err != nil || strings.Contains(string(raw), "TestFirstWeakInCentral") {
		t.Errorf("RED: the central file admits the new row: %v\n%s", err, raw)
	}
}
