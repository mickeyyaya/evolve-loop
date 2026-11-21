package journeys_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/journeys"
)

const repoRoot = "../../.."

const seededRows = 5

func registryViolations(root string) []string {
	rows, err := journeys.Load(filepath.Join(root, filepath.FromSlash(journeys.RegistryPath)))
	if err != nil {
		return []string{err.Error()}
	}
	violations := journeys.Unresolved(root, rows)
	for _, r := range rows {
		file, err := journeys.Resolve(root, r.Test)
		if r.Retired == "" && err == nil && !guards.IsProtectedSurface(file) {
			violations = append(violations, r.ID+": "+file+" is not on the protected surface")
		}
	}
	return violations
}

func fixtureRoot(t *testing.T, registry string, testFiles map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{journeys.RegistryPath: registry}
	for rel, body := range testFiles {
		files[rel] = body
	}
	for rel, body := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestRegistry_EverySeededRowResolvesToARealProtectedTest(t *testing.T) {
	if got := registryViolations(repoRoot); len(got) != 0 {
		t.Fatalf("the committed registry has violations:\n%s", strings.Join(got, "\n"))
	}
	rows, err := journeys.Load(filepath.Join(repoRoot, filepath.FromSlash(journeys.RegistryPath)))
	if err != nil || len(rows) < seededRows {
		t.Fatalf("want at least %d rows, got %d (err %v)", seededRows, len(rows), err)
	}
}

func TestRegistry_ARowThatNamesAMissingTestFailsTheGuard(t *testing.T) {
	root := fixtureRoot(t, `[{"id":"ghost","journey":"A path","test":"internal/internalx.TestGhost","issue":"#1"}]`, nil)
	got := registryViolations(root)
	if len(got) != 1 || !strings.HasPrefix(got[0], "ghost: ") {
		t.Errorf("violations = %v, want one naming the row ghost", got)
	}
}

func TestRegistry_AnUnretiredRowThatLosesItsTestFailsAndARetiredRowDoesNot(t *testing.T) {
	registry := `[{"id":"lost","journey":"A path","test":"internal/commitgate.TestGone","issue":"#1","retired":"%s"}]`
	if got := registryViolations(fixtureRoot(t, strings.Replace(registry, "%s", "", 1), nil)); len(got) != 1 {
		t.Errorf("an unretired row with no test must fail: %v", got)
	}
	if got := registryViolations(fixtureRoot(t, strings.Replace(registry, "%s", "the path ended", 1), nil)); len(got) != 0 {
		t.Errorf("a retired row needs no test: %v", got)
	}
}

func TestRegistry_ARowWhoseTestFileIsNotProtectedFailsTheGuard(t *testing.T) {
	root := fixtureRoot(t, `[{"id":"open","journey":"A path","test":"internal/widget.TestOpen","issue":"#1"}]`,
		map[string]string{"go/internal/widget/open_test.go": "package widget\n\nimport \"testing\"\n\nfunc TestOpen(t *testing.T) {}\n"})
	got := registryViolations(root)
	if len(got) != 1 || !strings.Contains(got[0], "go/internal/widget/open_test.go is not on the protected surface") {
		t.Errorf("violations = %v, want the unprotected test file named", got)
	}
	if guards.IsProtectedSurface("go/internal/widget/open_test.go") {
		t.Fatal("the fixture path must stay unprotected")
	}
}

func TestRegistry_ThePathOfTheRegistryAndOfItsGuardAreProtected(t *testing.T) {
	for _, p := range []string{journeys.RegistryPath, "go/test/journeys/journeys_test.go", "go/internal/journeys/registry.go", "go/internal/phases/ship/fixjourney.go"} {
		if !guards.IsProtectedSurface(p) {
			t.Errorf("%s must be on the protected surface", p)
		}
	}
}
