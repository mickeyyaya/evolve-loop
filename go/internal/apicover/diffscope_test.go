package apicover

// diffscope_test.go — cycle-1048 pins: enforcement is diff-scoped. A lane owns
// the hygiene of files it touched; pre-existing debt in untouched files WARNs
// and never fails the run. Nil ChangedFilesByDir = classic behavior (CI).

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture: package with two exported funcs in separate files; a cover profile
// giving both 0%; both named in a test file (⇒ both false-green).
func writeDiffScopeFixture(t *testing.T) (dir, coverPath string) {
	t.Helper()
	dir = t.TempDir()
	mod := "module fixture.example/diffscope\n\ngo 1.23\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"touched.go":   "package p\n\n// Touched is exported.\nfunc Touched() int { return 1 }\n",
		"untouched.go": "package p\n\n// Legacy is exported.\nfunc Legacy() int { return 2 }\n",
		"p_test.go":    "package p\n\nimport \"testing\"\n\nfunc TestNames(t *testing.T) { _ = Touched; _ = Legacy }\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cover := "fixture.example/diffscope/touched.go:4:\tTouched\t0.0%\nfixture.example/diffscope/untouched.go:4:\tLegacy\t0.0%\n"
	coverPath = filepath.Join(t.TempDir(), "cover.func")
	if err := os.WriteFile(coverPath, []byte(cover), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, coverPath
}

func TestDiffScope_PreExistingDebtWarnsNotFails(t *testing.T) {
	dir, cover := writeDiffScopeFixture(t)
	var buf strings.Builder
	code, err := Run(context.Background(), Config{
		Enforce: true, Dirs: []string{dir}, CoverPath: cover,
		ChangedFilesByDir: map[string]map[string]bool{dir: {"touched.go": true}},
	}, &buf)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if code == 0 {
		t.Fatalf("touched-file violation must still fail:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "PRE-EXISTING DEBT") || !strings.Contains(buf.String(), "Legacy") {
		t.Fatalf("untouched-file violation must surface as PRE-EXISTING DEBT:\n%s", buf.String())
	}
}

func TestDiffScope_OnlyPreExistingPasses(t *testing.T) {
	dir, cover := writeDiffScopeFixture(t)
	var buf strings.Builder
	code, err := Run(context.Background(), Config{
		Enforce: true, Dirs: []string{dir}, CoverPath: cover,
		ChangedFilesByDir: map[string]map[string]bool{dir: {"other.go": true}},
	}, &buf)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if code != 0 {
		t.Fatalf("with ONLY pre-existing debt the run must pass (cycle-1048 class):\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "PRE-EXISTING DEBT") {
		t.Fatalf("debt must still be reported loudly:\n%s", buf.String())
	}
}

func TestDiffScope_NilFilterKeepsClassicBehavior(t *testing.T) {
	dir, cover := writeDiffScopeFixture(t)
	var buf strings.Builder
	code, err := Run(context.Background(), Config{Enforce: true, Dirs: []string{dir}, CoverPath: cover}, &buf)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if code == 0 {
		t.Fatal("nil filter must keep classic enforce semantics (CI unchanged)")
	}
}
