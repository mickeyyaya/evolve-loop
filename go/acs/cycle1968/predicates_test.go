//go:build acs

package cycle1968

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/evidence"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	packageNote  = "docs/architecture/packages/internal-handoff.md"
	enforceList  = "go/.apicover-enforce"
	evidencePkg  = "go/internal/handoff/evidence"
	declaredTest = "TestPlannerSelects"
)

func fixtureTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go/internal/planner/planner.go", "package planner\n\nfunc Select() {}\n")
	write("go/internal/planner/planner_test.go", "package planner\n\nimport \"testing\"\n\nfunc "+declaredTest+"(t *testing.T) {}\n")
	write("go/acs/cycle1968/predicates_test.go", "//go:build acs\n\npackage cycle1968\n\nimport \"testing\"\n\nfunc TestC1968_900_Declared(t *testing.T) {}\n")
	write("go/acs/cycle1967/predicates_test.go", "//go:build acs\n\npackage cycle1967\n\nimport \"testing\"\n\nfunc TestC1967_900_Foreign(t *testing.T) {}\n")
	return root
}

func requireUnresolved(t *testing.T, src evidence.Source, ev handoff.Evidence) {
	t.Helper()
	if err := evidence.Resolve(src, ev); !errors.Is(err, evidence.ErrUnresolved) {
		t.Errorf("RED: Resolve(%s %q) = %v, want ErrUnresolved", ev.Kind, ev.Ref, err)
	}
}

func requireResolved(t *testing.T, src evidence.Source, ev handoff.Evidence) {
	t.Helper()
	if err := evidence.Resolve(src, ev); err != nil {
		t.Errorf("RED: Resolve(%s %q) = %v, want nil", ev.Kind, ev.Ref, err)
	}
}

func TestC1968_001_TestNameNoTestFileDeclaresDoesNotResolve(t *testing.T) {
	src := evidence.Source{Root: fixtureTree(t), Cycle: 1968}
	requireResolved(t, src, handoff.Evidence{Kind: "test", Ref: declaredTest})
	requireResolved(t, src, handoff.Evidence{Kind: "test", Ref: "go/internal/planner/planner_test.go:" + declaredTest})
	requireUnresolved(t, src, handoff.Evidence{Kind: "test", Ref: "TestNobodyDeclaresThis"})
	requireUnresolved(t, src, handoff.Evidence{Kind: "test", Ref: "Select"})
	requireUnresolved(t, src, handoff.Evidence{Kind: "test", Ref: "go/internal/planner/planner.go:" + declaredTest})
}

func TestC1968_002_PathLinePastEndOfFileDoesNotResolve(t *testing.T) {
	src := evidence.Source{Root: fixtureTree(t), Cycle: 1968}
	requireResolved(t, src, handoff.Evidence{Kind: "path", Ref: "go/internal/planner/planner.go:1"})
	requireResolved(t, src, handoff.Evidence{Kind: "path", Ref: "go/internal/planner/planner.go:3"})
	requireUnresolved(t, src, handoff.Evidence{Kind: "path", Ref: "go/internal/planner/planner.go:4"})
	requireUnresolved(t, src, handoff.Evidence{Kind: "path", Ref: "go/internal/planner/planner.go:0"})
	requireUnresolved(t, src, handoff.Evidence{Kind: "path", Ref: "go/internal/planner/gone.go:1"})
	requireUnresolved(t, src, handoff.Evidence{Kind: "path", Ref: "../escape.go:1"})
}

func TestC1968_003_PathLineBoundIsTheRealLineCountOfARepoFile(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := "go/internal/handoff/types.go"
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Count(string(body), "\n")
	src := evidence.Source{Root: root, Cycle: 1968}
	requireResolved(t, src, handoff.Evidence{Kind: "path", Ref: rel + ":" + strconv.Itoa(lines)})
	requireUnresolved(t, src, handoff.Evidence{Kind: "path", Ref: rel + ":" + strconv.Itoa(lines+1)})
}

func TestC1968_004_PredicateResolvesOnlyWhenTheCyclePackageDeclaresIt(t *testing.T) {
	root := fixtureTree(t)
	own := handoff.Evidence{Kind: "predicate", Ref: "go/acs/cycle1968/predicates_test.go:TestC1968_900_Declared"}
	foreign := handoff.Evidence{Kind: "predicate", Ref: "go/acs/cycle1967/predicates_test.go:TestC1967_900_Foreign"}
	requireResolved(t, evidence.Source{Root: root, Cycle: 1968}, own)
	requireUnresolved(t, evidence.Source{Root: root, Cycle: 1968}, foreign)
	requireUnresolved(t, evidence.Source{Root: root, Cycle: 1967}, own)
	requireUnresolved(t, evidence.Source{Root: root, Cycle: 1968}, handoff.Evidence{Kind: "predicate", Ref: "go/acs/cycle1968/predicates_test.go:TestC1968_901_Undeclared"})
	requireUnresolved(t, evidence.Source{Root: root, Cycle: 1968}, handoff.Evidence{Kind: "predicate", Ref: declaredTest})
}

func TestC1968_005_CommandResolvesOnlyWhenItsResultIsRecorded(t *testing.T) {
	src := evidence.Source{Root: fixtureTree(t), Cycle: 1968, Commands: []string{"go test ./internal/planner"}}
	requireResolved(t, src, handoff.Evidence{Kind: "command", Ref: "go test ./internal/planner"})
	requireUnresolved(t, src, handoff.Evidence{Kind: "command", Ref: "go test ./internal/other"})
	requireUnresolved(t, evidence.Source{Root: src.Root, Cycle: 1968}, handoff.Evidence{Kind: "command", Ref: "go test ./internal/planner"})
}

func TestC1968_006_UnknownKindDoesNotResolve(t *testing.T) {
	src := evidence.Source{Root: fixtureTree(t), Cycle: 1968, Commands: []string{"screenshot"}}
	requireUnresolved(t, src, handoff.Evidence{Kind: "screenshot", Ref: "go/internal/planner/planner.go:1"})
	requireUnresolved(t, src, handoff.Evidence{Kind: "", Ref: "go/internal/planner/planner.go:1"})
	requireUnresolved(t, src, handoff.Evidence{Kind: "screenshot", Ref: "screenshot"})
}

// acs-predicate: config-check
func TestC1968_007_PackageNoteListsTheEvidenceKinds(t *testing.T) {
	root := acsassert.RepoRoot(t)
	note := filepath.Join(root, filepath.FromSlash(packageNote))
	if !acsassert.FileExists(t, note) {
		t.Fatalf("RED: %s missing", packageNote)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", packageNote); code != 0 {
		t.Errorf("RED: %s is not tracked", packageNote)
	}
	body, err := os.ReadFile(note)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"`path`", "`test`", "`predicate`", "`command`", "`internal/handoff/evidence`"} {
		if !strings.Contains(string(body), kind) {
			t.Errorf("RED: %s does not name %s", packageNote, kind)
		}
	}
}

// acs-predicate: config-check
func TestC1968_008_NewPackageGraduatesIntoTheApicoverGate(t *testing.T) {
	root := acsassert.RepoRoot(t)
	list, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(enforceList)))
	if err != nil {
		t.Fatal(err)
	}
	enrolled := false
	for _, line := range strings.Split(string(list), "\n") {
		if strings.TrimSpace(line) == "./internal/handoff/evidence" {
			enrolled = true
		}
	}
	if !enrolled {
		t.Errorf("RED: ./internal/handoff/evidence is not a line of %s", enforceList)
	}
	named := filepath.Join(root, filepath.FromSlash(evidencePkg), "apicover_named_test.go")
	if !acsassert.FileExists(t, named) {
		t.Fatalf("RED: %s missing", named)
	}
	body, err := os.ReadFile(named)
	if err != nil {
		t.Fatal(err)
	}
	for _, symbol := range []string{"Resolve", "Source", "ErrUnresolved"} {
		if !strings.Contains(string(body), "evidence."+symbol) {
			t.Errorf("RED: apicover_named_test.go does not name evidence.%s", symbol)
		}
	}
}

// acs-predicate: config-check
func TestC1968_009_PackageCarriesNoComments(t *testing.T) {
	root := acsassert.RepoRoot(t)
	dir := filepath.Join(root, filepath.FromSlash(evidencePkg))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("RED: %s: %v", evidencePkg, err)
	}
	checked := 0
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		checked++
		for i, line := range strings.Split(string(body), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") || strings.Contains(line, " // ") {
				t.Errorf("RED: %s:%d carries a comment: %q", entry.Name(), i+1, line)
			}
		}
	}
	if checked == 0 {
		t.Errorf("RED: %s holds no Go file", evidencePkg)
	}
}
