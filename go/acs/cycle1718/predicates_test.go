//go:build acs

// Package cycle1718 materialises the acceptance criteria for
// profiles-test-hygiene.
//
// The fixture predicates compile ./internal/profiles with -trimpath. Under
// -trimpath runtime.Caller reports module-relative file paths, so the tests'
// runtime.Caller-anchored profiles directory resolves to
// github.com/mickeyyaya/evolve-loop/.evolve/profiles relative to the test
// binary's working directory. Each fixture lays the repository's tracked
// profiles out under that prefix and decides which of them git tracks, so the
// verdict under test is the real test's own, over a profile set no phase
// sandbox forbids writing.
package cycle1718

import (
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

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	fallbackTest  = "TestEveryAgentProfileHasAFallbackChain"
	oldRouterTest = "TestLoopUnblockProfilesRouteTimeoutPronePhasesToAgy"
	// fixtureRepo is the repository root a -trimpath build of
	// ./internal/profiles resolves: the module path minus its "/go" suffix.
	fixtureRepo = "github.com/mickeyyaya/evolve-loop"
	stubName    = "zz-untracked-stub-c1718"
	rescueDoc   = "docs/operations/rescue-branch-disposition-2026-06-07.md"
)

// stubPayload is a runtime-minted-style profile: dispatchable (cli set, name
// set so Loader.List keeps it) and missing the cli_fallback the test demands.
const stubPayload = `{"name":"` + stubName + `","role":"stub","cli":"claude-tmux","model_tier_default":"fast"}`

// redirectingGitVars would point a child git away from the fixture.
var redirectingGitVars = map[string]bool{
	"GIT_DIR":                          true,
	"GIT_WORK_TREE":                    true,
	"GIT_INDEX_FILE":                   true,
	"GIT_COMMON_DIR":                   true,
	"GIT_OBJECT_DIRECTORY":             true,
	"GIT_ALTERNATE_OBJECT_DIRECTORIES": true,
	"GIT_CEILING_DIRECTORIES":          true,
}

func childEnv(extra ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if !redirectingGitVars[name] {
			env = append(env, kv)
		}
	}
	return append(env, extra...)
}

// git runs git in dir. A non-zero exit is a fixture fault, not a verdict.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = childEnv()
	out, err := cmd.Output()
	if err != nil {
		var stderr []byte
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			stderr = ee.Stderr
		}
		t.Fatalf("fixture: git %s: %v\n%s", strings.Join(args, " "), err, stderr)
	}
	return strings.TrimSpace(string(out))
}

// profilesTestBinary compiles ./internal/profiles's tests from root with -trimpath.
func profilesTestBinary(t *testing.T, root string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "profiles.test")
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "test", "-c", "-trimpath", "-o", bin, "./internal/profiles")
	if code != 0 {
		t.Fatalf("compile ./internal/profiles tests: exit=%d err=%v\n%s%s", code, err, stdout, stderr)
	}
	return bin
}

// newFixture copies root's git-tracked .evolve/profiles into a fresh directory
// laid out for the -trimpath binary and returns fx, the binary's working
// directory, and repo, the repository root it resolves. With tracked, repo is
// a git repository whose index holds every copied profile — the index is what
// git ls-files, and so the tracked-set filter, reads.
func newFixture(t *testing.T, root string, tracked bool) (fx, repo string) {
	t.Helper()
	fx, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	repo = filepath.Join(fx, filepath.FromSlash(fixtureRepo))
	files := strings.Split(git(t, root, "ls-files", "--", ".evolve/profiles"), "\n")
	if len(files) < 50 {
		t.Fatalf("fixture: only %d tracked files under %s/.evolve/profiles", len(files), root)
	}
	for _, rel := range files {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("fixture: %v", err)
		}
		dst := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatalf("fixture: %v", err)
		}
		if err := os.WriteFile(dst, body, 0o644); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	if tracked {
		git(t, repo, "init", "-q")
		git(t, repo, "add", ".evolve/profiles")
	}
	return fx, repo
}

func writeStub(t *testing.T, repo string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, ".evolve", "profiles", stubName+".json"), []byte(stubPayload), 0o644); err != nil {
		t.Fatalf("fixture: %v", err)
	}
}

// runTest runs one top-level test from bin in fx, with git discovery fenced at
// fx, and returns "PASS", "FAIL", or "" when it reported neither (skipped,
// renamed, not run).
func runTest(t *testing.T, bin, fx, name string) (verdict, output string) {
	t.Helper()
	cmd := exec.Command(bin, "-test.run", "^"+name+"$", "-test.v", "-test.count=1")
	cmd.Dir = fx
	cmd.Env = childEnv("GIT_CEILING_DIRECTORIES=" + fx)
	out, err := cmd.CombinedOutput()
	var ee *exec.ExitError
	if err != nil && !errors.As(err, &ee) {
		t.Fatalf("fixture: launch %s: %v", bin, err)
	}
	output = string(out)
	switch {
	case err == nil && strings.Contains(output, "--- PASS: "+name+" "):
		return "PASS", output
	case err != nil && strings.Contains(output, "--- FAIL: "+name+" "):
		return "FAIL", output
	}
	return "", output
}

func TestC1718_001_FallbackChainIgnoresUntrackedStub(t *testing.T) {
	root := acsassert.RepoRoot(t)
	bin := profilesTestBinary(t, root)
	fx, repo := newFixture(t, root, true)
	if v, out := runTest(t, bin, fx, fallbackTest); v != "PASS" {
		t.Fatalf("fixture: %s over the tracked profiles alone = %q, want PASS — the -trimpath binary did not bind the fixture's profiles\n%s", fallbackTest, v, out)
	}
	writeStub(t, repo)
	if v, out := runTest(t, bin, fx, fallbackTest); v != "PASS" {
		t.Errorf("RED: %s = %q with an untracked stub (cli set, no cli_fallback) beside the tracked profiles, want PASS — it binds runtime-minted state instead of the tracked set\n%s", fallbackTest, v, out)
	}
}

func TestC1718_002_FallbackChainStillFailsTrackedViolator(t *testing.T) {
	root := acsassert.RepoRoot(t)
	bin := profilesTestBinary(t, root)
	fx, repo := newFixture(t, root, true)
	writeStub(t, repo)
	git(t, repo, "add", ".evolve/profiles")
	if v, out := runTest(t, bin, fx, fallbackTest); v != "FAIL" || !strings.Contains(out, stubName) {
		t.Errorf("%s = %q on a TRACKED profile with no cli_fallback, want FAIL naming %s — the filter may skip only untracked stubs, never a real violator\n%s", fallbackTest, v, stubName, out)
	}
}

func TestC1718_003_FallbackChainBindsEveryProfileWithoutGit(t *testing.T) {
	root := acsassert.RepoRoot(t)
	bin := profilesTestBinary(t, root)
	fx, repo := newFixture(t, root, false)
	if v, out := runTest(t, bin, fx, fallbackTest); v != "PASS" {
		t.Fatalf("%s = %q with no git context, want PASS — without a tracked set it must bind every on-disk profile, not crash\n%s", fallbackTest, v, out)
	}
	writeStub(t, repo)
	if v, out := runTest(t, bin, fx, fallbackTest); v != "FAIL" || !strings.Contains(out, stubName) {
		t.Errorf("%s = %q with no git context and a violating profile on disk, want FAIL naming %s — the bind-all fallback went dark\n%s", fallbackTest, v, stubName, out)
	}
}

var namedHelperRE = regexp.MustCompile(`(?m)^func .*ProfilesDir\(t \*testing\.T\) string`)

// profilesDirHelpers returns the package's funcs named like a profiles-dir
// helper, and the funcs that resolve .evolve through runtime.Caller.
func profilesDirHelpers(t *testing.T, pkgDir string) (named, resolvers []string) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(pkgDir, "*.go"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("fixture: no Go files in %s (%v)", pkgDir, err)
	}
	fset := token.NewFileSet()
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("fixture: %v", err)
		}
		for _, m := range namedHelperRE.FindAllString(string(src), -1) {
			named = append(named, filepath.Base(path)+": "+m)
		}
		file, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Body != nil && callsRuntimeCaller(fn.Body) && mentionsEvolveDir(fn.Body) {
				resolvers = append(resolvers, filepath.Base(path)+": "+fn.Name.Name)
			}
		}
	}
	sort.Strings(named)
	sort.Strings(resolvers)
	return named, resolvers
}

func callsRuntimeCaller(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "Caller" {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "runtime" {
				found = true
			}
		}
		return !found
	})
	return found
}

func mentionsEvolveDir(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if s, err := strconv.Unquote(lit.Value); err == nil && strings.Contains(s, ".evolve") {
				found = true
			}
		}
		return !found
	})
	return found
}

// acs-predicate: config-check — "grep finds one profiles-dir helper" is a
// source-structure criterion; the go test half proves the surviving helper
// still resolves the live profiles for the retargeted effort matrix.
func TestC1718_004_OneProfilesDirHelper(t *testing.T) {
	root := acsassert.RepoRoot(t)
	named, resolvers := profilesDirHelpers(t, filepath.Join(root, "go", "internal", "profiles"))
	if len(named) != 1 {
		t.Errorf("RED: %d funcs match `func .*ProfilesDir(t *testing.T) string` in go/internal/profiles, want exactly 1: %v", len(named), named)
	}
	if len(resolvers) != 1 {
		t.Errorf("RED: %d funcs resolve .evolve through runtime.Caller in go/internal/profiles, want exactly 1 canonical helper: %v", len(resolvers), resolvers)
	}
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "test", "-count=1", "-run", "^TestEffortDefaults_Matrix$", "-v", "./internal/profiles")
	if code != 0 || !strings.Contains(stdout, "--- PASS: TestEffortDefaults_Matrix ") {
		t.Errorf("TestEffortDefaults_Matrix over the live profiles: exit=%d err=%v, want PASS\n%s%s", code, err, stdout, stderr)
	}
}

func listTests(t *testing.T, bin string) []string {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(bin, "-test.list", ".*")
	if code != 0 {
		t.Fatalf("list tests: exit=%d err=%v\n%s", code, err, stderr)
	}
	return strings.Fields(stdout)
}

// rewriteRouter writes router.json with field replaced (orig when value is nil).
func rewriteRouter(t *testing.T, path string, orig []byte, field string, value any) {
	t.Helper()
	body := orig
	if value != nil {
		doc := map[string]any{}
		if err := json.Unmarshal(orig, &doc); err != nil {
			t.Fatalf("fixture: parse router.json: %v", err)
		}
		doc[field] = value
		var err error
		if body, err = json.Marshal(doc); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("fixture: %v", err)
	}
}

// pinsRouter reports whether test passes on the real router profile and fails
// both when the router leaves agy-tmux and when its fallback leaves claude-tmux.
func pinsRouter(t *testing.T, bin, fx, routerPath string, orig []byte, test string) bool {
	t.Helper()
	defer rewriteRouter(t, routerPath, orig, "", nil)
	cases := []struct {
		field string
		value any
		want  string
	}{
		{"", nil, "PASS"},
		{"cli", "claude-tmux", "FAIL"},
		{"cli_fallback", []string{"codex-tmux"}, "FAIL"},
	}
	for _, c := range cases {
		rewriteRouter(t, routerPath, orig, c.field, c.value)
		if v, out := runTest(t, bin, fx, test); v != c.want {
			t.Logf("%s with router %s=%v: %q, want %s\n%s", test, c.field, c.value, v, c.want, out)
			return false
		}
	}
	return true
}

var routerNameRE = []*regexp.Regexp{
	regexp.MustCompile(`(?i)router`),
	regexp.MustCompile(`(?i)agy`),
	regexp.MustCompile(`(?i)fallback`),
}

var misleadingNameRE = regexp.MustCompile(`(?i)phases|timeoutprone`)

func TestC1718_005_RouterTestNameStatesItsAssertion(t *testing.T) {
	root := acsassert.RepoRoot(t)
	bin := profilesTestBinary(t, root)
	names := listTests(t, bin)
	var candidates []string
	for _, n := range names {
		if n == oldRouterTest {
			t.Errorf("RED: %s is still registered — its name promises agy routing for timeout-prone phases, but it checks only the router's CLI and fallback", oldRouterTest)
		}
		named := !misleadingNameRE.MatchString(n)
		for _, re := range routerNameRE {
			named = named && re.MatchString(n)
		}
		if named {
			candidates = append(candidates, n)
		}
	}
	if len(candidates) == 0 {
		t.Fatalf("RED: no test in ./internal/profiles is named for what the router check asserts — want a name containing Router, Agy and Fallback (e.g. TestRouterStaysOnAgyWithClaudeFallback), without Phases/TimeoutProne; %d tests registered", len(names))
	}
	fx, repo := newFixture(t, root, true)
	routerPath := filepath.Join(repo, ".evolve", "profiles", "router.json")
	orig, err := os.ReadFile(routerPath)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	for _, c := range candidates {
		if pinsRouter(t, bin, fx, routerPath, orig, c) {
			return
		}
	}
	t.Errorf("RED: none of %v passes on the real router profile and fails when router.json leaves agy-tmux or its [claude-tmux] fallback — the renamed test must still assert both", candidates)
}

// acs-predicate: config-check — documentation accuracy criterion; the
// Loader.Get half pins the behavior the corrected doc must describe.
func TestC1718_006_RescueDocStatesMainExpandsAllowedTools(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"tool-policy.json": `{"policies":{"c1718":["Read","Grep"]}}`,
		"probe.json":       `{"name":"probe","cli":"claude-tmux","allowed_tools":["$include_policy:c1718","Bash"]}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	p, err := profiles.NewFromDir(dir).Get("probe")
	if err != nil {
		t.Fatalf("fixture: Get: %v", err)
	}
	if want := []string{"Read", "Grep", "Bash"}; !reflect.DeepEqual(p.AllowedTools, want) {
		t.Fatalf("fixture: Get returned AllowedTools %v, want %v — main no longer expands allowed_tools sentinels, so this criterion's premise changed", p.AllowedTools, want)
	}

	doc := filepath.Join(acsassert.RepoRoot(t), rescueDoc)
	for _, stale := range []string{
		"Main expands exactly the field that uses the mechanism",
		"add the expansion then",
	} {
		acsassert.FileNotContains(t, doc, stale)
	}
	hit, err := acsassert.LineContainsAllChecked(doc, "expandPolicies(AllowedTools)", "Get")
	if err != nil {
		t.Fatal(err)
	}
	if !hit {
		t.Errorf("RED: the `expandPolicies(AllowedTools)` row of %s does not name Get — it must state that main's (*Loader).Get now expands allowed_tools", rescueDoc)
	}
}
