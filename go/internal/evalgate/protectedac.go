package evalgate

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const needsChangedColumn = "needs changed"

type protectedACGate struct{}

func (protectedACGate) name() string                { return "protected-ac-predicate" }
func (protectedACGate) appliesTo(phase string) bool { return phase == string(core.PhaseTDD) }

func (protectedACGate) check(in core.ReviewInput) (string, bool) {
	report, err := os.ReadFile(filepath.Join(in.Workspace, phasecontract.ArtifactFilename(string(core.PhaseTDD))))
	if err != nil {
		return "", false
	}
	table, ok := acTable(string(report))
	if !ok {
		return "", false
	}
	if table.needs < 0 {
		return fmt.Sprintf("the AC-Materialization table has no %q column — list the repo paths each AC needs changed, so a predicate on a protected path can be found before the build", needsChangedColumn), true
	}
	acs, paths := table.protectedPredicates()
	if len(acs) == 0 {
		return "", false
	}
	return fmt.Sprintf("AC %s need protected path(s) %s changed — a lane builder can never change them, so the predicate is unsatisfiable (cycle 1788); "+
		"give each such AC the console-followup disposition and drop its predicate", strings.Join(acs, ", "), strings.Join(paths, ", ")), true
}

type acMaterialization struct {
	disposition, needs int
	rows               [][]string
}

func acTable(report string) (acMaterialization, bool) {
	lines := acSection(strings.Split(report, "\n"))
	for i, line := range lines {
		header := tableCells(line)
		disposition := columnIndex(header, "disposition")
		if disposition < 0 {
			continue
		}
		t := acMaterialization{disposition: disposition, needs: columnIndex(header, needsChangedColumn)}
		for _, row := range lines[i+1:] {
			cells := tableCells(row)
			if cells == nil {
				break
			}
			if !isSeparatorRow(cells) {
				t.rows = append(t.rows, cells)
			}
		}
		return t, true
	}
	return acMaterialization{}, false
}

func (t acMaterialization) protectedPredicates() (acs, paths []string) {
	for _, row := range t.rows {
		if max(t.disposition, t.needs) >= len(row) || !isPredicateDisposition(row[t.disposition]) {
			continue
		}
		found := guards.ProtectedPathsIn(row[t.needs])
		if len(found) == 0 {
			continue
		}
		acs = append(acs, row[0])
		for _, p := range found {
			if !slices.Contains(paths, p) {
				paths = append(paths, p)
			}
		}
	}
	slices.Sort(paths)
	return acs, paths
}

func tableCells(line string) []string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") {
		return nil
	}
	cells := strings.Split(strings.Trim(trimmed, "|"), "|")
	for i, c := range cells {
		cells[i] = strings.TrimSpace(c)
	}
	return cells
}

func columnIndex(header []string, name string) int {
	return slices.IndexFunc(header, func(c string) bool { return strings.ToLower(c) == name })
}

func isSeparatorRow(cells []string) bool {
	for _, c := range cells {
		if strings.Trim(c, "-: ") != "" {
			return false
		}
	}
	return true
}

func isPredicateDisposition(cell string) bool {
	return strings.HasPrefix(strings.ToLower(strings.Trim(cell, " *`")), "predicate")
}

func acSection(lines []string) []string {
	start := slices.IndexFunc(lines, func(l string) bool {
		return slices.Contains(phasecontract.TDD.Sections[0].Accepted, strings.TrimSpace(l))
	})
	if start < 0 {
		return nil
	}
	rest := lines[start+1:]
	if end := slices.IndexFunc(rest, func(l string) bool { return strings.HasPrefix(l, "## ") }); end >= 0 {
		rest = rest[:end]
	}
	return rest
}
