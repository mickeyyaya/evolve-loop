package gateregistry

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Dir(dir)
		}
		if parent := filepath.Dir(dir); parent != dir {
			dir = parent
			continue
		}
		t.Fatal("no go.mod above the package directory")
	}
}

func TestEntries_EachHasItsFieldsAndAKnownPhase(t *testing.T) {
	catalog, _, _, err := phasespec.MergedCatalog(repoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range Entries() {
		if problems := entryProblems(e, catalog.Names()); len(problems) > 0 {
			t.Errorf("entry %s breaks the contract:\n%s", e.Name, strings.Join(problems, "\n"))
		}
	}
}

func inventoryRows(doc string) map[string][]string {
	_, table, _ := strings.Cut(doc, "\n## The inventory\n")
	table, _, _ = strings.Cut(table, "\n## ")
	rows := map[string][]string{}
	for _, line := range strings.Split(table, "\n") {
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		if len(cells) != 5 {
			continue
		}
		path := strings.Trim(strings.TrimSpace(cells[1]), "`")
		rows[strings.TrimSuffix(path, "/")] = append(rows[strings.TrimSuffix(path, "/")], strings.TrimSpace(cells[3]))
	}
	return rows
}

func inputProblems(entries []gatecontract.Entry, rows map[string][]string) []string {
	var problems []string
	for _, e := range entries {
		for _, in := range e.Inputs {
			row := "{worktree_path}/" + strings.TrimSuffix(in.Path, "/")
			switch {
			case !slices.Contains(gatecontract.TrustClasses(), in.Class):
				problems = append(problems, e.Name+": "+in.Path+" has the class "+in.Class+", which is not a trust class")
			case !slices.Contains(rows[row], in.Class):
				problems = append(problems, e.Name+": the inventory has no row "+row+" with the class "+in.Class)
			}
		}
	}
	return problems
}

func TestEntries_EachInputIsAnInventoryRowWithTheSameClass(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "architecture", "gate-input-inventory.md"))
	if err != nil {
		t.Fatal(err)
	}
	if problems := inputProblems(Entries(), inventoryRows(string(doc))); len(problems) > 0 {
		t.Errorf("the registry and the inventory disagree:\n%s", strings.Join(problems, "\n"))
	}
}

func TestInputProblems_FlagsAnUnknownClassAndAMissingRow(t *testing.T) {
	t.Parallel()
	entries := []gatecontract.Entry{{Name: "g", Inputs: []gatecontract.Declared{{Path: "go/", Class: "trusted"}, {Path: "go/x", Class: "subject"}, {Path: "go/y", Class: "subject"}}}}
	rows := map[string][]string{"{worktree_path}/go/y": {"subject"}}
	got := strings.Join(inputProblems(entries, rows), "\n")
	for _, want := range []string{"go/ has the class trusted", "no row {worktree_path}/go/x with the class subject"} {
		if !strings.Contains(got, want) {
			t.Errorf("problems = %q, want a line with %q", got, want)
		}
	}
	if strings.Contains(got, "go/y") {
		t.Errorf("problems = %q, want none for the listed row", got)
	}
}

func TestInventoryRows_ReadsOnlyTheInventoryTable(t *testing.T) {
	t.Parallel()
	doc := "| a | `x` | h | authority | c |\n\n## The inventory\n\n| Gate | Input | Writer | Trust | Check |\n|---|---|---|---|---|\n| G | `{worktree_path}/go/` | b | subject | c |\n| H | two words | b | verified | c |\n\n## Decisions\n\n| I | `y` | b | authority | c |\n"
	rows := inventoryRows(doc)
	if len(rows["{worktree_path}/go"]) != 1 || rows["{worktree_path}/go"][0] != "subject" || len(rows["y"]) != 0 || len(rows["x"]) != 0 {
		t.Errorf("rows = %v, want only the table under The inventory", rows)
	}
}

func gateGuaranteeProblems(paths []string) []string {
	var problems []string
	for _, p := range paths {
		if class := guards.ClassOf(p); class != guards.GateGuarantee {
			problems = append(problems, p+" has the class \""+string(class)+"\", want the gate guarantee")
		}
	}
	return problems
}

func TestGates_EachFileEachAuthorityInputAndBothRegistryPackagesAreGateGuaranteeRows(t *testing.T) {
	t.Parallel()
	paths := []string{"go/internal/gatecontract/", "go/internal/gateregistry/", "go/internal/repocontract/repocontract.go", "go/internal/phases/ship/repocontract.go"}
	for _, e := range Entries() {
		paths = append(paths, e.Files...)
		for _, in := range e.Inputs {
			if in.Class == gatecontract.ClassAuthority {
				paths = append(paths, in.Path)
			}
		}
	}
	if problems := gateGuaranteeProblems(paths); len(problems) > 0 {
		t.Errorf("a gate path is not on the protected surface as a gate:\n%s", strings.Join(problems, "\n"))
	}
}

func TestGateGuaranteeProblems_NamesTheClassOfTheLongestRow(t *testing.T) {
	t.Parallel()
	got := gateGuaranteeProblems([]string{"go/internal/guards/integrity_surface.go"})
	if len(got) != 1 || !strings.Contains(got[0], "\""+string(guards.ManifestGuarantee)+"\"") {
		t.Errorf("problems = %q, want one line that names the class %q", got, guards.ManifestGuarantee)
	}
}

func TestGateGuaranteeProblems_FlagsAPathWithNoRow(t *testing.T) {
	t.Parallel()
	got := gateGuaranteeProblems([]string{"go/internal/zzzunlisted/", "go/internal/testdup/"})
	if len(got) != 1 || !strings.HasPrefix(got[0], "go/internal/zzzunlisted/") {
		t.Errorf("problems = %q, want one line for the unlisted path", got)
	}
}
