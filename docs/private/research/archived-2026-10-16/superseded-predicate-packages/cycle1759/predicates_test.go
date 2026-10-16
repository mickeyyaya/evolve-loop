//go:build acs

package cycle1759

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	pinnedRollbackAllowance        = 111
	pinnedPruneephemeralAllowance  = 117
	pinnedMarketplacepollAllowance = 107

	baseCommit = "b401e73d"
)

func spanLines(t *testing.T, root, key string) int {
	t.Helper()
	spans, err := sizeratchet.Walk(filepath.Join(root, "go"))
	if err != nil {
		t.Fatalf("sizeratchet.Walk: %v", err)
	}
	max := -1
	for _, s := range spans {
		if s.Key == key && s.Lines > max {
			max = s.Lines
		}
	}
	if max == -1 {
		t.Fatalf("no function span found for key %q", key)
	}
	return max
}

func TestC1759_001_RollbackRunFitsSizeRatchet(t *testing.T) {
	root := acsassert.RepoRoot(t)
	lines := spanLines(t, root, "internal/rollback.Run")
	if lines > sizeratchet.MaxLines {
		t.Errorf("internal/rollback.Run is %d lines > %d: shrink it via extracted named steps, behavior unchanged", lines, sizeratchet.MaxLines)
	}
}

func TestC1759_002_PruneephemeralRunFitsSizeRatchet(t *testing.T) {
	root := acsassert.RepoRoot(t)
	lines := spanLines(t, root, "internal/pruneephemeral.Run")
	if lines > sizeratchet.MaxLines {
		t.Errorf("internal/pruneephemeral.Run is %d lines > %d: shrink it via extracted named steps, behavior unchanged", lines, sizeratchet.MaxLines)
	}
}

func TestC1759_003_MarketplacepollRunFitsSizeRatchet(t *testing.T) {
	root := acsassert.RepoRoot(t)
	lines := spanLines(t, root, "internal/marketplacepoll.Run")
	if lines > sizeratchet.MaxLines {
		t.Errorf("internal/marketplacepoll.Run is %d lines > %d: shrink it via extracted named steps, behavior unchanged", lines, sizeratchet.MaxLines)
	}
}

func TestC1759_004_OffendersJSONAllowancesUnchanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	offenders, err := sizeratchet.LoadOffenders(filepath.Join(root, "go", "internal", "sizeratchet", "offenders.json"))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	for key, want := range map[string]int{
		"internal/rollback.Run":        pinnedRollbackAllowance,
		"internal/pruneephemeral.Run":  pinnedPruneephemeralAllowance,
		"internal/marketplacepoll.Run": pinnedMarketplacepollAllowance,
	} {
		got, listed := offenders[key]
		if !listed {
			t.Errorf("offenders.json: %s entry removed — this lane must leave it as slack for the boundary tighten", key)
			continue
		}
		if got != want {
			t.Errorf("offenders.json: %s allowance changed to %d, want unchanged %d", key, got, want)
		}
	}
}

func TestC1759_005_TargetPackagesTestsPassUnmodified(t *testing.T) {
	for _, pkg := range []string{
		"github.com/mickeyyaya/evolve-loop/go/internal/rollback",
		"github.com/mickeyyaya/evolve-loop/go/internal/pruneephemeral",
		"github.com/mickeyyaya/evolve-loop/go/internal/marketplacepoll",
	} {
		stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-count=1", pkg)
		if err != nil || code != 0 {
			t.Errorf("go test -count=1 %s failed: code=%d err=%v\n%s%s", pkg, code, err, stdout, stderr)
		}
	}
}

func TestC1759_006_NoCommentsAddedToShrunkPackages(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, dir := range []string{
		filepath.Join(root, "go", "internal", "rollback"),
		filepath.Join(root, "go", "internal", "pruneephemeral"),
		filepath.Join(root, "go", "internal", "marketplacepoll"),
	} {
		var out, errOut bytes.Buffer
		args := []string{"comments", "-base", baseCommit, dir}
		code := commentaudit.Main(args, &out, &errOut, worktreeGit{root: root})
		if code != 0 && !strings.Contains(errOut.String(), "no changed Go files under") {
			t.Errorf("commentaudit comments exit=%d — the diff adds comment lines under %s:\n%s%s", code, dir, out.String(), errOut.String())
		}
	}
}

type worktreeGit struct{ root string }

func (g worktreeGit) run(args ...string) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("git", append([]string{"-C", g.root}, args...)...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func (g worktreeGit) ChangedFiles(base string) ([]string, error) {
	tracked, err := g.run("diff", "--name-only", "--no-renames", base)
	if err != nil {
		return nil, err
	}
	untracked, err := g.run("ls-files", "--others", "--exclude-standard", "--full-name", ":/")
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(tracked) + "\n" + string(untracked)), nil
}

func (g worktreeGit) Show(base, path string) ([]byte, error) {
	if _, err := g.run("cat-file", "-e", base+":"+path); err != nil {
		return nil, fs.ErrNotExist
	}
	return g.run("show", base+":"+path)
}

func (g worktreeGit) Root() (string, error) { return g.root, nil }

type shrunkRun struct {
	path      string
	key       string
	allowance int
}

var shrunkRuns = []shrunkRun{
	{"go/internal/rollback/rollback.go", "internal/rollback.Run", pinnedRollbackAllowance},
	{"go/internal/pruneephemeral/pruneephemeral.go", "internal/pruneephemeral.Run", pinnedPruneephemeralAllowance},
	{"go/internal/marketplacepoll/marketplacepoll.go", "internal/marketplacepoll.Run", pinnedMarketplacepollAllowance},
}

var (
	runLineCountClaim  = regexp.MustCompile("`?Run`? \\((\\d+)\\s*(?:→|->)\\s*(\\d+) lines?\\)")
	committedClaim     = regexp.MustCompile(`(?i)(\b(?:not|never)\s+(?:yet\s+)?(?:been\s+)?|n't\s+(?:yet\s+)?(?:been\s+)?)?\bcommitted\b`)
	backtickedRepoPath = regexp.MustCompile("`([\\w.-]+(?:/[\\w.-]+)+\\.\\w+)`")
	cyclePredicateRef  = regexp.MustCompile(`TestC1759_\d{3}_\w+`)
)

func explanationDocLines(t *testing.T, root string) map[string][]string {
	t.Helper()
	docs, err := filepath.Glob(filepath.Join(root, "docs", "explain", "builds", "cycle-1759-*.md"))
	if err != nil {
		t.Fatalf("glob build explanation: %v", err)
	}
	if len(docs) == 0 {
		t.Fatalf("no docs/explain/builds/cycle-1759-*.md build explanation on disk")
	}
	byDoc := make(map[string][]string, len(docs))
	for _, doc := range docs {
		body, err := os.ReadFile(doc)
		if err != nil {
			t.Fatalf("read %s: %v", doc, err)
		}
		byDoc[filepath.Base(doc)] = strings.Split(string(body), "\n")
	}
	return byDoc
}

func runLineCountClaimFor(lines []string, path string) []string {
	for _, line := range lines {
		if !strings.Contains(line, path) {
			continue
		}
		if m := runLineCountClaim.FindStringSubmatch(line); m != nil {
			return m
		}
	}
	return nil
}

func affirmsCommitted(line string) bool {
	for _, m := range committedClaim.FindAllStringSubmatch(line, -1) {
		if m[1] == "" {
			return true
		}
	}
	return false
}

func TestC1759_007_ExplanationRunLineCountsMatchSizeratchet(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for doc, lines := range explanationDocLines(t, root) {
		for _, run := range shrunkRuns {
			claim := runLineCountClaimFor(lines, run.path)
			if claim == nil {
				t.Errorf("%s: the %s entry states no `Run` (before→after lines) count", doc, run.path)
				continue
			}
			before, _ := strconv.Atoi(claim[1])
			after, _ := strconv.Atoi(claim[2])
			if before != run.allowance {
				t.Errorf("%s: claims %s was %d lines before the shrink, the pinned baseline is %d", doc, run.key, before, run.allowance)
			}
			if measured := spanLines(t, root, run.key); after != measured {
				t.Errorf("%s: claims %s is %d lines after the shrink, sizeratchet.Walk measures %d", doc, run.key, after, measured)
			}
		}
	}
}

func TestC1759_008_ExplanationCommitClaimsMatchGitHead(t *testing.T) {
	root := acsassert.RepoRoot(t)
	git := worktreeGit{root: root}
	for doc, lines := range explanationDocLines(t, root) {
		for i, line := range lines {
			if !affirmsCommitted(line) {
				continue
			}
			for _, m := range backtickedRepoPath.FindAllStringSubmatch(line, -1) {
				if _, err := git.run("cat-file", "-e", "HEAD:"+m[1]); err != nil {
					t.Errorf("%s:%d says %s is committed, but HEAD does not contain it: %v", doc, i+1, m[1], err)
				}
			}
		}
	}
}

func cyclePredicateNames(t *testing.T, root string) map[string]bool {
	t.Helper()
	path := filepath.Join(root, "go", "acs", "cycle1759", "predicates_test.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	names := make(map[string]bool)
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && strings.HasPrefix(fn.Name.Name, "TestC1759_") {
			names[fn.Name.Name] = true
		}
	}
	return names
}

func TestC1759_009_EvalsMaterializedForSelectedSlugs(t *testing.T) {
	root := acsassert.RepoRoot(t)
	defined := cyclePredicateNames(t, root)
	for _, slug := range []string{
		"shrink-rollback-run",
		"shrink-pruneephemeral-run",
		"shrink-marketplacepoll-run",
		"sizeratchet-shrink-run-commands",
	} {
		rel := filepath.Join(".evolve", "evals", slug+".md")
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Errorf("selected slug %s has no eval: %v", slug, err)
			continue
		}
		if !bytes.Contains(body, []byte("[code]")) {
			t.Errorf("%s carries no [code] grader", rel)
		}
		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "check-ignore", "-q", rel); code != 1 {
			t.Errorf("%s: git check-ignore exit %d, want 1 (not ignored) — an ignored eval is dropped at ship", rel, code)
		}
		graded := cyclePredicateRef.FindAllString(string(body), -1)
		if len(graded) == 0 {
			t.Errorf("%s grades no TestC1759_ predicate", rel)
		}
		for _, name := range graded {
			if !defined[name] {
				t.Errorf("%s grades %s, which go/acs/cycle1759/predicates_test.go does not define", rel, name)
			}
		}
	}
}

func TestC1759_010_ModuleWideRatchetCheckPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	spans, err := sizeratchet.Walk(filepath.Join(root, "go"))
	if err != nil {
		t.Fatalf("sizeratchet.Walk: %v", err)
	}
	offenders, err := sizeratchet.LoadOffenders(filepath.Join(root, "go", "internal", "sizeratchet", "offenders.json"))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	if err := sizeratchet.Check(spans, offenders); err != nil {
		t.Errorf("module-wide ratchet is red: %v", err)
	}
}
