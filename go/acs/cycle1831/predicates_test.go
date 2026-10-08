//go:build acs

package cycle1831

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	stalePackage       = "./acs/cycle1723"
	stalePredicate     = "TestC1723_003_"
	stalePredicateRuns = 8
	packageDocRule     = "internal/commentaudit/stats.go"
)

var redirectingGitVars = []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY"}

var fixtureModuleFiles = []string{"go.mod", "go.sum", "vendor/modules.txt"}

var protectedCommentauditSources = []string{"go/internal/commentaudit/", "go/cmd/commentaudit/"}

type ruleMutant struct {
	name, anchor, replacement string
}

var packageDocRuleMutants = []ruleMutant{
	{"pre753_rewrite_of_an_existing_doc_is_never_spared", "limit = max(limit, existing)", "return false"},
	{"rewrite_held_to_three_lines_not_its_old_length", "limit = max(limit, existing)", "_ = existing"},
	{"rewrite_past_three_lines_and_its_old_length_is_spared", "return len(packageDoc(after)) <= limit", "return len(packageDoc(after)) <= limit*100"},
	{"doc_added_to_a_file_with_none_at_base_is_spared", "if existing == 0 {", "if existing < 0 {"},
}

func childEnv() []string {
	return slices.DeleteFunc(os.Environ(), func(kv string) bool {
		name, _, _ := strings.Cut(kv, "=")
		return slices.Contains(redirectingGitVars, name)
	})
}

func run(t *testing.T, dir, name string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = childEnv()
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return string(out), 0
	case errors.As(err, &exitErr):
		return string(out), exitErr.ExitCode()
	}
	t.Fatalf("launch %s %v in %s: %v", name, args, dir, err)
	return "", -1
}

func mustRun(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	out, code := run(t, dir, name, args...)
	if code != 0 {
		t.Fatalf("%s %v in %s exited %d:\n%s", name, args, dir, code, out)
	}
	return out
}

func copyDir(t *testing.T, from, to string) {
	t.Helper()
	entries, err := os.ReadDir(from)
	if err != nil {
		t.Fatalf("fixture: read %s: %v", from, err)
	}
	for _, e := range entries {
		if e.Type().IsRegular() {
			copyFile(t, filepath.Join(from, e.Name()), filepath.Join(to, e.Name()))
		}
	}
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	body, err := os.ReadFile(from)
	if err != nil {
		t.Fatalf("fixture: read %s: %v", from, err)
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		t.Fatalf("fixture: mkdir for %s: %v", to, err)
	}
	if err := os.WriteFile(to, body, 0o644); err != nil {
		t.Fatalf("fixture: write %s: %v", to, err)
	}
}

func commentauditBuildDirs(t *testing.T, goDir string) []string {
	t.Helper()
	out := mustRun(t, goDir, "go", "list", "-deps", "-f", "{{if not .Standard}}{{.Dir}}{{end}}", "./cmd/commentaudit")
	var rels []string
	for _, dir := range strings.Fields(out) {
		rel, err := filepath.Rel(goDir, dir)
		if err != nil || strings.HasPrefix(rel, "..") {
			t.Fatalf("fixture: commentaudit dependency %s is outside %s (rel %q, err %v)", dir, goDir, rel, err)
		}
		rels = append(rels, rel)
	}
	return rels
}

func commentauditRepo(t *testing.T, goDir string, buildDirs []string) string {
	t.Helper()
	repo := gittest.Fixture(t)
	fixtureGo := filepath.Join(repo.Dir, "go")
	for _, rel := range buildDirs {
		copyDir(t, filepath.Join(goDir, rel), filepath.Join(fixtureGo, rel))
	}
	for _, rel := range fixtureModuleFiles {
		copyFile(t, filepath.Join(goDir, filepath.FromSlash(rel)), filepath.Join(fixtureGo, filepath.FromSlash(rel)))
	}
	repo.Git("add", ".")
	repo.Git("commit", "-q", "-m", "commentaudit at the cycle's tree")
	return repo.Dir
}

func mutate(t *testing.T, root string, m ruleMutant) {
	t.Helper()
	path := filepath.Join(root, "go", filepath.FromSlash(packageDocRule))
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("mutant %s: read %s: %v", m.name, path, err)
	}
	if n := strings.Count(string(src), m.anchor); n != 1 {
		t.Fatalf("mutant %s: anchor %q occurs %d times in %s, want 1: the package-doc rule changed shape, so this mutant table needs the new shape", m.name, m.anchor, n, packageDocRule)
	}
	mutated := strings.Replace(string(src), m.anchor, m.replacement, 1)
	if err := os.WriteFile(path, []byte(mutated), 0o644); err != nil {
		t.Fatalf("mutant %s: write %s: %v", m.name, path, err)
	}
}

func compileStalePredicates(t *testing.T, goDir string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "cycle1723.test")
	mustRun(t, goDir, "go", "test", "-c", "-tags", "acs", "-o", bin, stalePackage)
	return bin
}

func stalePredicatePasses(t *testing.T, bin, root string) (bool, string) {
	t.Helper()
	out, code := run(t, root, bin, "-test.run", "^"+stalePredicate, "-test.v", "-test.count=1")
	switch {
	case code == 0 && strings.Contains(out, "--- PASS: "+stalePredicate):
		return true, out
	case code != 0 && strings.Contains(out, "--- FAIL: "+stalePredicate):
		return false, out
	}
	t.Fatalf("%s neither passed nor failed on an assertion in %s (exit %d); a deleted, skipped or unbuildable predicate pins nothing:\n%s", stalePredicate, root, code, out)
	return false, out
}

func TestC1831_001_Cycle1723PackagePassesWithEveryPredicateRun(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	out, code := run(t, goDir, "go", "test", "-tags", "acs", "-count=1", "-v", stalePackage)
	if code != 0 {
		t.Fatalf("go test -tags acs %s exited %d, want 0 under the #753 package-doc rule:\n%s", stalePackage, code, out)
	}
	for n := 1; n <= stalePredicateRuns; n++ {
		if pass := fmt.Sprintf("--- PASS: TestC1723_%03d_", n); !strings.Contains(out, pass) {
			t.Errorf("%q is missing: every cycle-1723 predicate must still run and pass, none deleted or skipped:\n%s", pass, out)
		}
	}
}

func TestC1831_002_PackageDocPredicateKillsEveryRuleMutant(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	bin := compileStalePredicates(t, goDir)
	buildDirs := commentauditBuildDirs(t, goDir)

	if passed, out := stalePredicatePasses(t, bin, commentauditRepo(t, goDir, buildDirs)); !passed {
		t.Fatalf("control: %s fails against the unmutated #753 package-doc rule, so it still asserts the pre-#753 expectation:\n%s", stalePredicate, out)
	}
	for _, m := range packageDocRuleMutants {
		t.Run(m.name, func(t *testing.T) {
			root := commentauditRepo(t, goDir, buildDirs)
			mutate(t, root, m)
			if passed, out := stalePredicatePasses(t, bin, root); passed {
				t.Errorf("%s passes against a commentaudit whose package-doc rule is mutated (%q -> %q), so it does not pin that half of the #753 rule:\n%s", stalePredicate, m.anchor, m.replacement, out)
			}
		})
	}
}

func TestC1831_003_CommentauditSourceStaysUntouched(t *testing.T) {
	root := acsassert.RepoRoot(t)
	base := strings.TrimSpace(mustRun(t, root, "git", "-C", root, "merge-base", "main", "HEAD"))
	tracked := mustRun(t, root, "git", "-C", root, "diff", "--name-only", base)
	untracked := mustRun(t, root, "git", "-C", root, "ls-files", "--others", "--exclude-standard", "--full-name")
	for _, path := range strings.Fields(tracked + "\n" + untracked) {
		for _, protected := range protectedCommentauditSources {
			if strings.HasPrefix(path, protected) {
				t.Errorf("%s changed since %s: commentaudit is a protected surface; the stale cycle-1723 predicate is what moves to the #753 rule", path, base)
			}
		}
	}
}
