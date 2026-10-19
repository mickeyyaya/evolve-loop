//go:build acs

package cycle1963

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	oldBinaryBody      = "old-binary-v1"
	rebuiltBinaryBody  = "rebuilt-binary-v2"
	failureMarker      = "FINAL-FAILURE-MARKER-9f3a"
	invocationRecord   = "make-invocation.txt"
	pinBeforeRebuild   = "pin-before-rebuild"
	succeedingMakefile = "build:\n\tmkdir -p bin\n\tprintf '%s' '" + rebuiltBinaryBody + "' > bin/evolve\n\tprintf '%s %s' \"$(CURDIR)\" \"$(MAKECMDGOALS)\" > ../" + invocationRecord + "\n"
	failingMakefile    = "build:\n\t@i=1; while [ $$i -le 80 ]; do echo noise-line-$$i >&2; i=$$((i+1)); done; echo " + failureMarker + " >&2; exit 7\n"
)

var evolveBuild struct {
	once      sync.Once
	dir, path string
	failure   string
}

func TestMain(m *testing.M) {
	code := m.Run()
	if evolveBuild.dir != "" {
		if err := os.RemoveAll(evolveBuild.dir); err != nil {
			fmt.Fprintf(os.Stderr, "cycle1963: remove %s: %v\n", evolveBuild.dir, err)
		}
	}
	os.Exit(code)
}

func evolveBin(t *testing.T) string {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	evolveBuild.once.Do(func() { evolveBuild.failure = buildEvolve(goDir) })
	if evolveBuild.failure != "" {
		t.Fatalf("go build ./cmd/evolve: %s", evolveBuild.failure)
	}
	return evolveBuild.path
}

func buildEvolve(goDir string) string {
	dir, err := os.MkdirTemp("", "cycle1963-evolve-")
	if err != nil {
		return err.Error()
	}
	evolveBuild.dir, evolveBuild.path = dir, filepath.Join(dir, "evolve")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", evolveBuild.path, "./cmd/evolve")
	cmd.Dir = goDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Sprintf("%v\n%s", err, out)
	}
	return ""
}

type result struct {
	stdout, stderr string
	code           int
}

func (r result) String() string {
	return fmt.Sprintf("exit=%d\n--- stdout ---\n%s--- stderr ---\n%s", r.code, r.stdout, r.stderr)
}

func (r result) all() string { return r.stdout + r.stderr }

func isolatedEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "EVOLVE_") || strings.HasPrefix(name, "GIT_") {
			continue
		}
		env = append(env, kv)
	}
	return env
}

func runEvolve(t *testing.T, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, evolveBin(t), args...)
	cmd.Env = isolatedEnv()
	cmd.Dir = t.TempDir()
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("evolve %v: %v", args, err)
		}
		code = exitErr.ExitCode()
	}
	return result{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func sha256Hex(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

func rebuildFixture(t *testing.T, makefile string) string {
	t.Helper()
	if _, err := exec.LookPath("make"); err != nil {
		t.Skipf("make not on PATH: %v", err)
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go", "Makefile"), makefile)
	writeFile(t, filepath.Join(root, "go", "bin", "evolve"), oldBinaryBody)
	writeFile(t, filepath.Join(root, ".evolve", "state.json"), `{"expected_ship_sha":"`+pinBeforeRebuild+`"}`)
	return root
}

func boundBinaryBody(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "go", "bin", "evolve"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestC1963_001_RebuildBuildsHeadToBootRefreshPathAndPrintsBothSHAs(t *testing.T) {
	root := rebuildFixture(t, succeedingMakefile)
	r := runEvolve(t, "rebuild", "--project-root", root)
	if r.code != 0 {
		t.Fatalf("evolve rebuild must exit 0 on a successful build; got %s", r)
	}
	if got := boundBinaryBody(t, root); got != rebuiltBinaryBody {
		t.Errorf("go/bin/evolve = %q, want the rebuilt %q (the path the boot refresh writes)", got, rebuiltBinaryBody)
	}
	for label, body := range map[string]string{"old": oldBinaryBody, "new": rebuiltBinaryBody} {
		if sum := sha256Hex(body); !strings.Contains(r.stdout, sum[:12]) {
			t.Errorf("stdout lacks the %s binary sha %s...: %s", label, sum[:12], r)
		}
	}
}

func TestC1963_002_RebuildRunsTheBootRefreshMakeTargetInPlaneGoDir(t *testing.T) {
	root := rebuildFixture(t, succeedingMakefile)
	if r := runEvolve(t, "rebuild", "--project-root", root); r.code != 0 {
		t.Fatalf("evolve rebuild: %s", r)
	}
	rec, err := os.ReadFile(filepath.Join(root, invocationRecord))
	if err != nil {
		t.Fatalf("make never ran in the fixture: %v", err)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(root, "go"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(string(rec)), resolved+" build"; got != want {
		t.Errorf("make invocation = %q, want %q (make -C go build from the project root)", got, want)
	}
}

func TestC1963_003_RebuildListedInHelp(t *testing.T) {
	r := runEvolve(t, "help")
	if !strings.Contains(r.all(), "rebuild") {
		t.Errorf("evolve help does not list the rebuild verb: %s", r)
	}
}

func TestC1963_004_RebuildLeavesShipPinAloneWithoutRepin(t *testing.T) {
	root := rebuildFixture(t, succeedingMakefile)
	if r := runEvolve(t, "rebuild", "--project-root", root); r.code != 0 {
		t.Fatalf("evolve rebuild: %s", r)
	}
	statePath := filepath.Join(root, ".evolve", "state.json")
	if !acsassert.JSONFieldEquals(t, statePath, "expected_ship_sha", pinBeforeRebuild) {
		t.Errorf("rebuild without --repin must not move expected_ship_sha")
	}
}

func TestC1963_005_LiveRunLeaseRefusesRebuildWithExit1(t *testing.T) {
	root := rebuildFixture(t, succeedingMakefile)
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "live", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	r := runEvolve(t, "rebuild", "--project-root", root)
	if r.code != 1 {
		t.Errorf("a live run lease must exit 1; got %s", r)
	}
	if _, err := os.Stat(filepath.Join(root, invocationRecord)); err == nil {
		t.Errorf("make ran although a live run lease holds the plane")
	}
	if got := boundBinaryBody(t, root); got != oldBinaryBody {
		t.Errorf("go/bin/evolve changed to %q under a live lease", got)
	}
}

func TestC1963_006_StaleRunLeaseDoesNotBlockRebuild(t *testing.T) {
	root := rebuildFixture(t, succeedingMakefile)
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	longAgo := time.Unix(1_000_000_000, 0)
	if err := runlease.Write(runDir, runlease.Lease{RunID: "stale", OwnerPID: os.Getpid()}, longAgo); err != nil {
		t.Fatal(err)
	}
	r := runEvolve(t, "rebuild", "--project-root", root)
	if r.code != 0 {
		t.Errorf("a stale run lease must not block the rebuild; got %s", r)
	}
	if got := boundBinaryBody(t, root); got != rebuiltBinaryBody {
		t.Errorf("go/bin/evolve = %q, want %q after a rebuild past a stale lease", got, rebuiltBinaryBody)
	}
}

func TestC1963_007_FailingBuildExits2WithOutputTail(t *testing.T) {
	root := rebuildFixture(t, failingMakefile)
	r := runEvolve(t, "rebuild", "--project-root", root)
	if r.code != 2 {
		t.Errorf("a failing build must exit 2; got %s", r)
	}
	if !strings.Contains(r.all(), failureMarker) {
		t.Errorf("output lacks the tail of the build output (%s): %s", failureMarker, r)
	}
}

type sourceIndex struct {
	files        map[string]*ast.File
	funcBodies   map[string]ast.Node
	varInits     map[string]ast.Node
	makeEncloser []string
}

func parseCmdEvolve(t *testing.T) sourceIndex {
	t.Helper()
	dir := filepath.Join(acsassert.RepoRoot(t), "go", "cmd", "evolve")
	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	idx := sourceIndex{files: map[string]*ast.File{}, funcBodies: map[string]ast.Node{}, varInits: map[string]ast.Node{}}
	fset := token.NewFileSet()
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		idx.files[filepath.Base(p)] = f
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && d.Body != nil {
					idx.funcBodies[d.Name.Name] = d.Body
					if callsMake(d.Body) {
						idx.makeEncloser = append(idx.makeEncloser, d.Name.Name)
					}
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, name := range vs.Names {
						if i < len(vs.Values) {
							idx.varInits[name.Name] = vs.Values[i]
						}
					}
				}
			}
		}
	}
	sort.Strings(idx.makeEncloser)
	return idx
}

func callsMake(body ast.Node) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || (sel.Sel.Name != "Command" && sel.Sel.Name != "CommandContext") {
			return true
		}
		for _, arg := range call.Args {
			if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if s, err := strconv.Unquote(lit.Value); err == nil && s == "make" {
					found = true
				}
			}
		}
		return true
	})
	return found
}

func identsOf(n ast.Node) []string {
	var out []string
	ast.Inspect(n, func(x ast.Node) bool {
		if id, ok := x.(*ast.Ident); ok {
			out = append(out, id.Name)
		}
		return true
	})
	return out
}

func (idx sourceIndex) reaches(start, target string) bool {
	seen := map[string]bool{}
	queue := []string{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == target {
			return true
		}
		if seen[cur] {
			continue
		}
		seen[cur] = true
		if body, ok := idx.funcBodies[cur]; ok {
			queue = append(queue, identsOf(body)...)
		}
		if init, ok := idx.varInits[cur]; ok {
			queue = append(queue, identsOf(init)...)
		}
	}
	return false
}

func (idx sourceIndex) registeredRunFunc(verb string) (string, bool) {
	var run string
	for _, f := range idx.files {
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			var name, runIdent string
			for _, elt := range lit.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, ok := kv.Key.(*ast.Ident)
				if !ok {
					continue
				}
				switch key.Name {
				case "Name":
					if bl, ok := kv.Value.(*ast.BasicLit); ok {
						name, _ = strconv.Unquote(bl.Value)
					}
				case "Run":
					if id, ok := kv.Value.(*ast.Ident); ok {
						runIdent = id.Name
					}
				}
			}
			if name == verb && runIdent != "" {
				run = runIdent
			}
			return true
		})
	}
	return run, run != ""
}

func TestC1963_008_BootRefreshAndRebuildVerbShareOneBuildFunction(t *testing.T) {
	idx := parseCmdEvolve(t)
	run, ok := idx.registeredRunFunc("rebuild")
	if !ok {
		t.Fatalf("no \"rebuild\" subcommand registered with a main-package Run function")
	}
	var verbSites, bootSites []string
	for _, site := range idx.makeEncloser {
		if idx.reaches(run, site) {
			verbSites = append(verbSites, site)
		}
		if idx.reaches("bootBinaryRefresh", site) {
			bootSites = append(bootSites, site)
		}
	}
	if len(verbSites) != 1 || len(bootSites) != 1 {
		t.Fatalf("each path must reach exactly one make site; rebuild verb %s reaches %v, bootBinaryRefresh reaches %v", run, verbSites, bootSites)
	}
	if verbSites[0] != bootSites[0] {
		t.Errorf("rebuild verb builds through %s but the boot refresh builds through %s; they must call one build function", verbSites[0], bootSites[0])
	}
}
