//go:build acs

package cycle2418

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	exitPass       = 0
	exitTestFailed = 1
	exitIOError    = 2
	exitUsageError = 10
	scopedVerb     = "test-scoped"
	scopedSchema   = "evolve.test-scoped/1"
	modulePath     = "example.com/m"
)

var (
	buildOnce sync.Once
	builtBin  string
	buildErr  error
	buildDir  string
)

func TestMain(m *testing.M) {
	code := m.Run()
	if buildDir != "" {
		_ = os.RemoveAll(buildDir)
	}
	os.Exit(code)
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		moduleDir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
		if err != nil {
			buildErr = err
			return
		}
		buildDir, err = os.MkdirTemp("", "evolve-acs-2418-")
		if err != nil {
			buildErr = err
			return
		}
		builtBin = filepath.Join(buildDir, "evolve")
		cmd := exec.Command("go", "build", "-o", builtBin, "./cmd/evolve")
		cmd.Dir = moduleDir
		if out, err := cmd.CombinedOutput(); err != nil {
			buildErr = &buildFailure{output: acsassert.OutputTail(string(out), 20)}
		}
	})
	if buildErr != nil {
		t.Fatalf("build evolve: %v", buildErr)
	}
	return builtBin
}

type buildFailure struct{ output string }

func (b *buildFailure) Error() string { return b.output }

type scopedResult struct {
	Schema     string   `json:"schema"`
	Selected   []string `json:"selected"`
	DurationMS *int64   `json:"duration_ms"`
	Pass       *bool    `json:"pass"`
}

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func passingTest(pkg string) string {
	return "package " + pkg + "\n\nimport \"testing\"\n\nfunc TestOK(t *testing.T) {}\n"
}

func failingTest(pkg string) string {
	return "package " + pkg + "\n\nimport \"testing\"\n\nfunc TestBroken(t *testing.T) { t.Fatal(\"broken\") }\n"
}

func importerSource(pkg, imported string) string {
	return "package " + pkg + "\n\nimport _ \"" + modulePath + "/" + imported + "\"\n"
}

func lane(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := gittest.Fixture(t)
	writeTree(t, repo.Dir, map[string]string{
		"go/go.mod":      "module " + modulePath + "\n\ngo 1.23\n",
		"go/a/a.go":      "package a\n",
		"go/a/a_test.go": passingTest("a"),
		"go/b/b.go":      importerSource("b", "a"),
		"go/b/b_test.go": passingTest("b"),
		"go/d/d.go":      importerSource("d", "b"),
		"go/d/d_test.go": passingTest("d"),
		"go/c/c.go":      "package c\n",
		"go/c/c_test.go": passingTest("c"),
		"docs/x.md":      "x\n",
	})
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "baseline")
	return repo
}

func commitOnLane(t *testing.T, repo *gittest.Repo, files map[string]string) {
	t.Helper()
	writeTree(t, repo.Dir, files)
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "lane change")
}

func runScoped(t *testing.T, root string, extra ...string) (scopedResult, string, int) {
	t.Helper()
	args := append([]string{scopedVerb, "--base", "main", "--root", root, "--json"}, extra...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(evolveBinary(t), args...)
	var res scopedResult
	if code == exitPass || code == exitTestFailed {
		if err := json.Unmarshal([]byte(stdout), &res); err != nil {
			t.Fatalf("RED: evolve %s --json printed no JSON document (exit %d): %v\nstdout: %q\nstderr: %s",
				scopedVerb, code, err, stdout, acsassert.OutputTail(stderr, 10))
		}
	}
	return res, stderr, code
}

func normalized(selected []string) []string {
	out := make([]string, 0, len(selected))
	for _, s := range selected {
		s = strings.TrimPrefix(s, modulePath+"/")
		s = strings.TrimPrefix(s, "./")
		s = strings.TrimSuffix(s, "/...")
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func sameSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func requireShape(t *testing.T, res scopedResult) {
	t.Helper()
	if res.Schema != scopedSchema {
		t.Errorf("schema = %q, want %q", res.Schema, scopedSchema)
	}
	if res.DurationMS == nil {
		t.Errorf("JSON has no duration_ms field")
	}
	if res.Pass == nil {
		t.Errorf("JSON has no pass field")
	}
}

func TestC2418_001_TheSelectedSetHoldsTheChangedPackageAndEachDirectAndTransitiveImporter(t *testing.T) {
	repo := lane(t)
	repo.Git("checkout", "-q", "-b", "lane")
	commitOnLane(t, repo, map[string]string{"go/a/a.go": "package a\n\nfunc A() {}\n"})

	res, stderr, code := runScoped(t, repo.Dir)

	if code != exitPass {
		t.Fatalf("RED: exit = %d, want %d; stderr: %s", code, exitPass, acsassert.OutputTail(stderr, 10))
	}
	requireShape(t, res)
	got := normalized(res.Selected)
	if want := []string{"a", "b", "d"}; !sameSet(got, want) {
		t.Errorf("selected = %v, want %v: the changed package a, its importer b and the transitive importer d; c imports nothing from a", got, want)
	}
	if res.Pass == nil || !*res.Pass {
		t.Errorf("pass = %v, want true for a change whose selected tests all pass", res.Pass)
	}
}

func TestC2418_002_CommitsThatLandOnMainAfterTheLaneForkedStayOutOfTheSelectedSet(t *testing.T) {
	repo := lane(t)
	repo.Git("checkout", "-q", "-b", "lane")
	repo.Git("checkout", "-q", "main")
	commitOnLane(t, repo, map[string]string{"go/c/c.go": "package c\n\nfunc C() {}\n"})
	repo.Git("checkout", "-q", "lane")
	commitOnLane(t, repo, map[string]string{"go/a/a.go": "package a\n\nfunc A() {}\n"})

	res, stderr, code := runScoped(t, repo.Dir)

	if code != exitPass {
		t.Fatalf("RED: exit = %d, want %d; stderr: %s", code, exitPass, acsassert.OutputTail(stderr, 10))
	}
	got := normalized(res.Selected)
	if want := []string{"a", "b", "d"}; !sameSet(got, want) {
		t.Errorf("selected = %v, want %v: package c changed on main only, after the fork point", got, want)
	}
}

func TestC2418_003_AChangeToANonGoFileSelectsNothingAndPasses(t *testing.T) {
	repo := lane(t)
	repo.Git("checkout", "-q", "-b", "lane")
	commitOnLane(t, repo, map[string]string{"docs/x.md": "changed\n"})

	res, stderr, code := runScoped(t, repo.Dir)

	if code != exitPass {
		t.Fatalf("RED: exit = %d, want %d; stderr: %s", code, exitPass, acsassert.OutputTail(stderr, 10))
	}
	requireShape(t, res)
	if len(res.Selected) != 0 {
		t.Errorf("selected = %v, want empty for a docs-only change", res.Selected)
	}
	if res.Pass == nil || !*res.Pass {
		t.Errorf("pass = %v, want true for an empty selection", res.Pass)
	}
}

func TestC2418_004_AFailingTestInAnImporterTurnsTheResultRedWithExitOne(t *testing.T) {
	repo := lane(t)
	repo.Git("checkout", "-q", "-b", "lane")
	commitOnLane(t, repo, map[string]string{
		"go/a/a.go":      "package a\n\nfunc A() {}\n",
		"go/b/b_test.go": failingTest("b"),
	})

	res, _, code := runScoped(t, repo.Dir)

	if code != exitTestFailed {
		t.Fatalf("RED: exit = %d, want %d: the importer b has a failing test", code, exitTestFailed)
	}
	if res.Pass == nil || *res.Pass {
		t.Errorf("pass = %v, want false when a selected test fails", res.Pass)
	}
	if got := normalized(res.Selected); !sameSet(got, []string{"a", "b", "d"}) {
		t.Errorf("selected = %v, want [a b d]", got)
	}
}

func TestC2418_005_AFailingTestInAnUnrelatedPackageIsNeverRun(t *testing.T) {
	repo := gittest.Fixture(t)
	writeTree(t, repo.Dir, map[string]string{
		"go/go.mod":      "module " + modulePath + "\n\ngo 1.23\n",
		"go/a/a.go":      "package a\n",
		"go/a/a_test.go": passingTest("a"),
		"go/c/c.go":      "package c\n",
		"go/c/c_test.go": failingTest("c"),
	})
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "baseline")
	repo.Git("checkout", "-q", "-b", "lane")
	commitOnLane(t, repo, map[string]string{"go/a/a.go": "package a\n\nfunc A() {}\n"})

	res, stderr, code := runScoped(t, repo.Dir)

	if code != exitPass {
		t.Fatalf("RED: exit = %d, want %d: c is unrelated to the change and must not run; stderr: %s", code, exitPass, acsassert.OutputTail(stderr, 10))
	}
	if got := normalized(res.Selected); !sameSet(got, []string{"a"}) {
		t.Errorf("selected = %v, want [a]", got)
	}
}

func TestC2418_006_AnUnknownFlagIsAUsageErrorAndARootThatIsNotARepositoryIsAnIOError(t *testing.T) {
	bin := evolveBinary(t)

	_, stderr, code, _ := acsassert.SubprocessOutput(bin, scopedVerb, "--no-such-flag")
	if code != exitUsageError {
		t.Errorf("unknown flag: exit = %d, want %d; stderr: %s", code, exitUsageError, acsassert.OutputTail(stderr, 5))
	}

	notARepository := t.TempDir()
	_, stderr, code, _ = acsassert.SubprocessOutput(bin, scopedVerb, "--base", "main", "--root", notARepository, "--json")
	if code != exitIOError || strings.Contains(stderr, "unknown command") {
		t.Errorf("root that is not a repository: exit = %d, want %d from the verb itself; stderr: %s", code, exitIOError, acsassert.OutputTail(stderr, 5))
	}
}

func TestC2418_007_ARunLeavesTheRepositoryAndItsRunStateUntouched(t *testing.T) {
	repo := lane(t)
	repo.Git("checkout", "-q", "-b", "lane")
	commitOnLane(t, repo, map[string]string{"go/a/a.go": "package a\n\nfunc A() {}\n"})
	before := repo.Git("status", "--porcelain", "--ignored")

	if _, stderr, code := runScoped(t, repo.Dir); code != exitPass {
		t.Fatalf("RED: exit = %d, want %d; stderr: %s", code, exitPass, acsassert.OutputTail(stderr, 10))
	}

	if after := repo.Git("status", "--porcelain", "--ignored"); after != before {
		t.Errorf("the run changed the tree:\nbefore: %q\nafter:  %q", before, after)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".evolve")); err == nil {
		t.Errorf("the run created .evolve: a shadow helper leaves no state for a gate to read")
	}
}

func TestC2418_008_OnlyTheVerbAndItsRegistrationMentionTheVerbInProductionSource(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	files, err := acsassert.WalkModule(goDir, func(rel string) bool {
		return strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go") && !strings.HasPrefix(rel, "acs/")
	})
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		"cmd/evolve/registry.go":        true,
		"cmd/evolve/cmd_test_scoped.go": true,
	}
	mentioned := map[string]bool{}
	for _, rel := range files {
		raw, err := os.ReadFile(filepath.Join(goDir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), scopedVerb) || strings.Contains(string(raw), scopedSchema) {
			mentioned[rel] = true
		}
	}
	for rel := range allowed {
		if !mentioned[rel] {
			t.Errorf("RED: %s does not mention %q: the verb is not registered", rel, scopedVerb)
		}
	}
	for rel := range mentioned {
		if !allowed[rel] {
			t.Errorf("%s mentions %q: a gate or another command reads the shadow helper", rel, scopedVerb)
		}
	}
}

func TestC2418_009_TheFloorStillListsEveryTierWithItsFullCommandAfterTheVerbExists(t *testing.T) {
	bin := evolveBinary(t)
	root := acsassert.RepoRoot(t)

	_, stderr, code, _ := acsassert.SubprocessOutput(bin, scopedVerb, "--no-such-flag")
	if code != exitUsageError {
		t.Fatalf("RED: %s is not a registered verb (exit %d, want %d); stderr: %s", scopedVerb, code, exitUsageError, acsassert.OutputTail(stderr, 5))
	}

	stdout, stderr, code, err := acsassert.SubprocessOutput(bin, "floor", "run", "--dry-run", "--root", root)
	if err != nil || code != 0 {
		t.Fatalf("floor run --dry-run: exit %d: %v\n%s", code, err, acsassert.OutputTail(stderr, 10))
	}
	for _, line := range []string{
		"vet: go vet ./...",
		"integration: make test-integration",
		"e2e: make test-e2e",
		"acs-durable: go test -count=1 -tags acs ./acs/regression/...",
		"apicover: make apicover-check",
		"cover: make cover-strict",
		"ratchet: go run ./cmd/evolve ratchet check --root .",
	} {
		if !strings.Contains(stdout, line+"\n") {
			t.Errorf("floor dry-run lost the line %q", line)
		}
	}
	if !strings.Contains(stdout, "fmt: ") {
		t.Errorf("floor dry-run lost the fmt tier")
	}
	if strings.Contains(stdout, scopedVerb) {
		t.Errorf("floor dry-run mentions %q: the floor must not run the scoped subset", scopedVerb)
	}
}

func TestC2418_010_TheIntegrationTargetStillDefaultsToTheWholeModuleAndNoWorkflowRunsTheScopedVerb(t *testing.T) {
	root := acsassert.RepoRoot(t)

	_, stderr, code, _ := acsassert.SubprocessOutput(evolveBinary(t), scopedVerb, "--no-such-flag")
	if code != exitUsageError {
		t.Fatalf("RED: %s is not a registered verb (exit %d, want %d); stderr: %s", scopedVerb, code, exitUsageError, acsassert.OutputTail(stderr, 5))
	}

	makefile := filepath.Join(root, "go", "Makefile")
	acsassert.FileContains(t, makefile, "roots=./...")
	acsassert.FileNotContains(t, makefile, scopedVerb)

	workflows, err := filepath.Glob(filepath.Join(root, ".github", "workflows", "*.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, workflow := range workflows {
		acsassert.FileNotContains(t, workflow, scopedVerb)
	}
}
