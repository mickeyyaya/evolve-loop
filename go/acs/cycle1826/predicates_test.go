//go:build acs

package cycle1826

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	pairCycle     = 1705
	halfPairCycle = 1707
	liveRunCycle  = 1706
	pairCloseout  = "dossier: cycle-1705 closeout\n\nknowledge-base/cycles/cycle-1705.json\nknowledge-base/cycles/cycle-1705.md"
	loopExitFunc  = "publishPendingDossiers"
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
			fmt.Fprintf(os.Stderr, "cycle1826: remove %s: %v\n", evolveBuild.dir, err)
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
	dir, err := os.MkdirTemp("", "cycle1826-evolve-")
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

func (r result) output() string { return r.stdout + r.stderr }

func (r result) String() string {
	return fmt.Sprintf("exit=%d\n--- stdout ---\n%s--- stderr ---\n%s", r.code, r.stdout, r.stderr)
}

func names(r result, token string) bool {
	return regexp.MustCompile(`\b` + regexp.QuoteMeta(token) + `\b`).MatchString(r.output())
}

func isolatedEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "EVOLVE_") || strings.HasPrefix(name, "GIT_") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
}

func runDossierCLI(t *testing.T, dir string, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, evolveBin(t), append([]string{"dossier"}, args...)...)
	cmd.Dir, cmd.Env, cmd.WaitDelay = dir, isolatedEnv(), 5*time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	r := result{stdout: stdout.String(), stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case ctx.Err() != nil:
		t.Fatalf("evolve dossier %s did not return within its budget; a held publish must exit, not wait: %s", strings.Join(args, " "), r)
	case err == nil:
	case errors.As(err, &exitErr):
		r.code = exitErr.ExitCode()
	default:
		t.Fatalf("evolve dossier %s: %v", strings.Join(args, " "), err)
	}
	return r
}

func publish(t *testing.T, plane string, extra ...string) result {
	t.Helper()
	return runDossierCLI(t, plane, append([]string{"publish", "--project-root", plane}, extra...)...)
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newPlane(t *testing.T) *gittest.Repo {
	t.Helper()
	r := gittest.Fixture(t)
	writeFile(t, filepath.Join(r.Dir, "README.md"), "plane\n")
	r.Git("add", "README.md")
	r.Git("commit", "-q", "-m", "base")
	return r
}

func pendingFile(root string, cycle int, ext string) string {
	return filepath.Join(dossier.PendingDir(root), "cycle-"+strconv.Itoa(cycle)+ext)
}

func corpusFile(root string, cycle int, ext string) string {
	return filepath.Join(dossier.CyclesDir(root), "cycle-"+strconv.Itoa(cycle)+ext)
}

func writePendingPair(t *testing.T, root string, cycle int) {
	t.Helper()
	d := &dossier.Dossier{Cycle: cycle, Goal: "a lane's closeout", FinalVerdict: dossier.VerdictPass,
		Phases: []dossier.PhaseRecord{{Name: "build", Verdict: dossier.VerdictPass}}}
	if err := dossier.Write(d, dossier.PendingDir(root), false); err != nil {
		t.Fatal(err)
	}
}

func writePendingHalfPair(t *testing.T, root string, cycle int) {
	t.Helper()
	writePendingPair(t, root, cycle)
	if err := os.Remove(pendingFile(root, cycle, ".json")); err != nil {
		t.Fatal(err)
	}
}

func addPairAndHalfPair(t *testing.T, r *gittest.Repo) *gittest.Repo {
	t.Helper()
	writePendingPair(t, r.Dir, pairCycle)
	writePendingHalfPair(t, r.Dir, halfPairCycle)
	return r
}

func leaseRun(t *testing.T, root string, cycle, ownerPID int) {
	t.Helper()
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(cycle))
	writeFile(t, filepath.Join(runDir, "run.json"), "{}\n")
	if err := runlease.Write(runDir, runlease.Lease{RunID: "run-" + strconv.Itoa(cycle), OwnerPID: ownerPID}, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func exitedPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return cmd.ProcessState.Pid()
}

func absent(path string) bool {
	_, err := os.Lstat(path)
	return errors.Is(err, fs.ErrNotExist)
}

func assertNothingPublished(t *testing.T, r *gittest.Repo, head string) {
	t.Helper()
	if got := r.Git("rev-parse", "HEAD"); got != head {
		t.Errorf("HEAD moved from %s to %s: a held publish committed", head, got)
	}
	if absent(pendingFile(r.Dir, pairCycle, ".json")) {
		t.Error("a held pair left .evolve/dossiers-pending")
	}
	if !absent(corpusFile(r.Dir, pairCycle, ".json")) {
		t.Error("a held pair reached knowledge-base/cycles")
	}
}

type planeState struct {
	Head   string
	Status string
	Files  map[string]string
}

func snapshot(t *testing.T, r *gittest.Repo) planeState {
	t.Helper()
	files := map[string]string{}
	shipLock := filepath.Join(".evolve", "ship.lock")
	err := filepath.WalkDir(r.Dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(r.Dir, path)
		if err != nil {
			return err
		}
		if d.IsDir() && rel == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() || rel == shipLock {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		files[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return planeState{
		Head:   r.Git("rev-parse", "HEAD"),
		Status: r.Git("status", "--porcelain=v1", "--untracked-files=all"),
		Files:  files,
	}
}

func TestC1826_001_PublishCommitsThePendingPairAndLeavesTheHalfPair(t *testing.T) {
	r := addPairAndHalfPair(t, newPlane(t))
	base := r.Git("rev-parse", "HEAD")
	halfBefore, err := os.ReadFile(pendingFile(r.Dir, halfPairCycle, ".md"))
	if err != nil {
		t.Fatal(err)
	}

	res := publish(t, r.Dir)

	if res.code != 0 {
		t.Fatalf("evolve dossier publish on a plane with one pending pair must exit 0: %s", res)
	}
	if got := r.Git("show", "--name-only", "--format=%s", "HEAD"); got != pairCloseout {
		t.Fatalf("HEAD = %q, want one commit of cycle 1705's closeout\n%s", got, res)
	}
	if got := r.Git("rev-parse", "HEAD~1"); got != base {
		t.Errorf("the closeout must be one commit on the base %s, HEAD~1 = %s", base, got)
	}
	for _, ext := range []string{".json", ".md"} {
		if !absent(pendingFile(r.Dir, pairCycle, ext)) {
			t.Errorf("published cycle-1705%s is still pending", ext)
		}
		if !absent(corpusFile(r.Dir, halfPairCycle, ext)) {
			t.Errorf("the half pair's cycle-1707%s reached the corpus", ext)
		}
	}
	if halfAfter, err := os.ReadFile(pendingFile(r.Dir, halfPairCycle, ".md")); err != nil || !bytes.Equal(halfAfter, halfBefore) {
		t.Errorf("the half pair must stay pending, untouched: err=%v", err)
	}
	for _, cycle := range []string{"1705", "1707"} {
		if !names(res, cycle) {
			t.Errorf("the report must name published cycle 1705 and half pair 1707; %s is missing: %s", cycle, res)
		}
	}
}

func TestC1826_002_ALiveRunLeaseHoldsThePublishWithExit1NamingTheRun(t *testing.T) {
	t.Run("a live run another process owns", func(t *testing.T) {
		r := addPairAndHalfPair(t, newPlane(t))
		leaseRun(t, r.Dir, liveRunCycle, os.Getpid())
		head := r.Git("rev-parse", "HEAD")

		res := publish(t, r.Dir)

		if res.code != 1 {
			t.Fatalf("a publish held by a live run must exit 1: %s", res)
		}
		assertNothingPublished(t, r, head)
		for _, want := range []string{"cycle-1706", "live pid " + strconv.Itoa(os.Getpid())} {
			if !strings.Contains(res.output(), want) {
				t.Errorf("the hold must name the live run (%q): %s", want, res)
			}
		}
	})
	t.Run("a sealed run whose owner exited does not hold", func(t *testing.T) {
		r := addPairAndHalfPair(t, newPlane(t))
		leaseRun(t, r.Dir, liveRunCycle, exitedPID(t))

		res := publish(t, r.Dir)

		if res.code != 0 {
			t.Fatalf("a lease whose owner exited must not hold the publish: %s", res)
		}
		if got := r.Git("show", "--name-only", "--format=%s", "HEAD"); got != pairCloseout {
			t.Errorf("HEAD = %q, want cycle 1705's closeout published past the dead lease\n%s", got, res)
		}
	})
}

func TestC1826_003_DryRunChangesNothingAndListsWhatWouldPublish(t *testing.T) {
	t.Run("an unheld plane", func(t *testing.T) {
		r := addPairAndHalfPair(t, newPlane(t))
		before := snapshot(t, r)

		res := publish(t, r.Dir, "--dry-run")

		if res.code != 0 {
			t.Fatalf("a dry run on an unheld plane must exit 0: %s", res)
		}
		if after := snapshot(t, r); !reflect.DeepEqual(after, before) {
			t.Errorf("--dry-run changed the plane:\nbefore %+v\nafter  %+v\n%s", before, after, res)
		}
		for _, cycle := range []string{"1705", "1707"} {
			if !names(res, cycle) {
				t.Errorf("the dry run must list pending cycle 1705 and half pair 1707; %s is missing: %s", cycle, res)
			}
		}
	})
	t.Run("a plane a live run holds", func(t *testing.T) {
		r := addPairAndHalfPair(t, newPlane(t))
		leaseRun(t, r.Dir, liveRunCycle, os.Getpid())
		before := snapshot(t, r)

		res := publish(t, r.Dir, "--dry-run")

		if after := snapshot(t, r); !reflect.DeepEqual(after, before) {
			t.Errorf("--dry-run changed a held plane:\nbefore %+v\nafter  %+v\n%s", before, after, res)
		}
		if !names(res, "1705") {
			t.Errorf("the dry run must list pending cycle 1705: %s", res)
		}
		for _, want := range []string{"cycle-1706", "live pid " + strconv.Itoa(os.Getpid())} {
			if !strings.Contains(res.output(), want) {
				t.Errorf("the dry run must say why a publish would be held (%q): %s", want, res)
			}
		}
	})
}

func TestC1826_004_AHeldPublishExits1NamingTheHoldAndNeverWaits(t *testing.T) {
	t.Run("the git-mutation lock is busy", func(t *testing.T) {
		r := addPairAndHalfPair(t, newPlane(t))
		release, err := flock.Lock(flock.ShipLockPath(r.Dir))
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		head := r.Git("rev-parse", "HEAD")

		res := publish(t, r.Dir)

		if res.code != 1 {
			t.Fatalf("a publish that cannot take the git-mutation lock must exit 1: %s", res)
		}
		assertNothingPublished(t, r, head)
		if !strings.Contains(strings.ToLower(res.output()), "lock") {
			t.Errorf("the hold must name the busy lock: %s", res)
		}
	})
	t.Run("the plane is behind origin/main", func(t *testing.T) {
		r := newPlane(t)
		r.Git("remote", "add", "origin", filepath.Join(r.Dir, "unreachable.git"))
		r.Git("commit", "-q", "--allow-empty", "-m", "upstream")
		r.Git("update-ref", "refs/remotes/origin/main", "HEAD")
		r.Git("reset", "-q", "--hard", "HEAD~1")
		addPairAndHalfPair(t, r)
		head := r.Git("rev-parse", "HEAD")

		res := publish(t, r.Dir)

		if res.code != 1 {
			t.Fatalf("a publish onto a plane behind origin/main must exit 1: %s", res)
		}
		assertNothingPublished(t, r, head)
		if !strings.Contains(res.output(), "BEHIND origin/main") {
			t.Errorf("the hold must name the relation to origin/main: %s", res)
		}
	})
	t.Run("nothing is pending while the lock is busy", func(t *testing.T) {
		r := newPlane(t)
		release, err := flock.Lock(flock.ShipLockPath(r.Dir))
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		head := r.Git("rev-parse", "HEAD")

		res := publish(t, r.Dir)

		if res.code != 0 {
			t.Fatalf("with nothing pending the publish must exit 0 without touching the lock: %s", res)
		}
		if got := r.Git("rev-parse", "HEAD"); got != head {
			t.Errorf("HEAD moved from %s to %s with nothing pending", head, got)
		}
	})
}

func TestC1826_005_AnUnlistablePendingDirectoryExits2(t *testing.T) {
	for _, extra := range [][]string{nil, {"--dry-run"}} {
		t.Run("publish "+strings.Join(extra, " "), func(t *testing.T) {
			r := newPlane(t)
			writeFile(t, dossier.PendingDir(r.Dir), "a file where the pending directory belongs\n")
			head := r.Git("rev-parse", "HEAD")

			res := publish(t, r.Dir, extra...)

			if res.code != 2 {
				t.Fatalf("a pending directory that cannot be listed is an I/O failure, exit 2: %s", res)
			}
			if got := r.Git("rev-parse", "HEAD"); got != head {
				t.Errorf("HEAD moved from %s to %s on an I/O failure", head, got)
			}
			if !strings.Contains(res.output(), "dossiers-pending") {
				t.Errorf("the failure must name what could not be read: %s", res)
			}
		})
	}
}

type callGraph struct {
	bodies     map[string][]ast.Node
	edges      map[string]map[string]bool
	primitives map[string]map[string]bool
}

func loadCallGraph(t *testing.T, dir string) callGraph {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	g := callGraph{bodies: map[string][]ast.Node{}, edges: map[string]map[string]bool{}, primitives: map[string]map[string]bool{}}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		g.addDecls(f.Decls)
	}
	for name, nodes := range g.bodies {
		g.edges[name], g.primitives[name] = map[string]bool{}, map[string]bool{}
		for _, n := range nodes {
			ast.Inspect(n, func(x ast.Node) bool {
				switch v := x.(type) {
				case *ast.Ident:
					if _, known := g.bodies[v.Name]; known {
						g.edges[name][v.Name] = true
					}
				case *ast.SelectorExpr:
					g.primitives[name]["."+v.Sel.Name] = true
					if pkg, ok := v.X.(*ast.Ident); ok {
						g.primitives[name][pkg.Name+"."+v.Sel.Name] = true
					}
				}
				return true
			})
		}
	}
	return g
}

func (g callGraph) addDecls(decls []ast.Decl) {
	for _, decl := range decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Body != nil {
				g.bodies[d.Name.Name] = append(g.bodies[d.Name.Name], d.Body)
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, v := range vs.Values {
					if lit, ok := v.(*ast.FuncLit); ok && i < len(vs.Names) {
						g.bodies[vs.Names[i].Name] = append(g.bodies[vs.Names[i].Name], lit.Body)
					}
				}
			}
		}
	}
}

func (g callGraph) closure(roots ...string) map[string]bool {
	seen := map[string]bool{}
	stack := append([]string(nil), roots...)
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[n] {
			continue
		}
		seen[n] = true
		for m := range g.edges[n] {
			stack = append(stack, m)
		}
	}
	return seen
}

func (g callGraph) reaches(set map[string]bool, primitives ...string) bool {
	for _, p := range primitives {
		found := false
		for n := range set {
			if g.primitives[n][p] {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (g callGraph) funcRefs(n ast.Node) []string {
	var refs []string
	ast.Inspect(n, func(x ast.Node) bool {
		if id, ok := x.(*ast.Ident); ok {
			if _, known := g.bodies[id.Name]; known {
				refs = append(refs, id.Name)
			}
		}
		return true
	})
	return refs
}

func isPublishLiteral(e ast.Expr) bool {
	lit, ok := e.(*ast.BasicLit)
	return ok && lit.Kind == token.STRING && lit.Value == `"publish"`
}

func (g callGraph) publishHandlers() []string {
	var handlers []string
	for _, body := range g.bodies["runDossier"] {
		ast.Inspect(body, func(x ast.Node) bool {
			switch v := x.(type) {
			case *ast.CaseClause:
				if slices.ContainsFunc(v.List, isPublishLiteral) {
					for _, stmt := range v.Body {
						handlers = append(handlers, g.funcRefs(stmt)...)
					}
				}
			case *ast.KeyValueExpr:
				if isPublishLiteral(v.Key) {
					handlers = append(handlers, g.funcRefs(v.Value)...)
				}
			}
			return true
		})
	}
	return handlers
}

// acs-predicate: config-check
func TestC1826_006_TheVerbAndTheLoopExitCallOneChecksFunction(t *testing.T) {
	g := loadCallGraph(t, filepath.Join(acsassert.RepoRoot(t), "go", "cmd", "evolve"))
	handlers := g.publishHandlers()
	if len(handlers) == 0 {
		t.Fatal(`runDossier routes no "publish" subcommand to a handler`)
	}
	if _, ok := g.bodies[loopExitFunc]; !ok {
		t.Fatalf("%s, the loop exit's publisher, is gone", loopExitFunc)
	}
	verb, loopExit := g.closure(handlers...), g.closure(loopExitFunc)
	if !g.reaches(verb, "dossier.PublishPending") {
		t.Errorf("the publish verb (%v) never reaches dossier.PublishPending", handlers)
	}
	var shared []string
	for name := range verb {
		if slices.Contains(handlers, name) || !loopExit[name] {
			continue
		}
		if g.reaches(g.closure(name), "plane.Classify", "loopchain.LiveSiblingRun", ".RelationToRemote") {
			shared = append(shared, name)
		}
	}
	if len(shared) == 0 {
		t.Errorf("no one function that both the publish verb (%v) and %s call runs every check: plane.Classify, the live-run check (loopchain.LiveSiblingRun) and the origin/main relation (RelationToRemote)", handlers, loopExitFunc)
	}
}

func TestC1826_007_TheDossierUsageNamesPublishAndKeepsExit10(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"no subcommand", nil},
		{"an unknown subcommand", []string{"nonesuch"}},
	} {
		res := runDossierCLI(t, dir, tc.args...)
		if res.code != 10 || !names(result{stderr: res.stderr}, "publish") {
			t.Errorf("evolve dossier with %s must exit 10 and name publish among its subcommands: %s", tc.name, res)
		}
	}
	res := runDossierCLI(t, dir, "publish", "--no-such-flag")
	if res.code != 10 || strings.Contains(res.stderr, "unknown subcommand") {
		t.Errorf("an unknown flag to a known publish verb is a usage error, exit 10: %s", res)
	}
}

// acs-predicate: config-check
func TestC1826_008_TheBoundaryProcedureDocumentsDossierPublish(t *testing.T) {
	doc := filepath.Join(acsassert.RepoRoot(t), "docs", "operations", "runtime-reference.md")
	body, err := os.ReadFile(doc)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	start := strings.Index(text, "**Wave boundary, through the CLI only")
	if start < 0 {
		t.Fatal("runtime-reference.md has no wave-boundary procedure")
	}
	section := text[start:]
	if end := strings.Index(section, "**Landing a patch or a salvaged cycle"); end > 0 {
		section = section[:end]
	}
	if !strings.Contains(section, "evolve dossier publish") {
		t.Error("the wave-boundary procedure in runtime-reference.md must document `evolve dossier publish`")
	}
}
