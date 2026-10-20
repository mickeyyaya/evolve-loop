package evalgate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const (
	needsChangedColumn    = "needs changed"
	dispositionColumn     = "disposition"
	predicatesColumn      = "predicate"
	criterionColumnPrefix = "criteri"
)

type protectedACGate struct {
	file ConsoleFollowupFiler
}

func (protectedACGate) name() string                { return "protected-ac-predicate" }
func (protectedACGate) appliesTo(phase string) bool { return phase == string(core.PhaseTDD) }

func (g protectedACGate) check(in core.ReviewInput) (string, bool) {
	report, err := os.ReadFile(filepath.Join(in.Workspace, phasecontract.ArtifactFilename(string(core.PhaseTDD))))
	if err != nil {
		return "", false
	}
	section, found := acSection(strings.Split(string(report), "\n"))
	if !found {
		return "", false
	}
	table, ok := acTable(section)
	switch {
	case !ok:
		return fmt.Sprintf("the AC section has no AC table with a %q column — give each AC one row with its disposition, so the gate can check it before the build", dispositionColumn), true
	case table.needs < 0:
		return fmt.Sprintf("the AC-Materialization table has no %q column — list the repo paths each AC needs changed, so a predicate on a protected path can be found before the build", needsChangedColumn), true
	}
	if problems := table.problems(); len(problems) > 0 {
		return strings.Join(problems, "; "), true
	}
	return g.fileFollowups(in, table)
}

type acMaterialization struct {
	disposition, needs, predicates, criterion int
	rows                                      [][]string
}

func acTable(lines []string) (acMaterialization, bool) {
	for i, line := range lines {
		header := tableCells(line)
		disposition := columnIndex(header, dispositionColumn)
		if disposition < 0 {
			continue
		}
		t := acMaterialization{disposition: disposition, needs: columnIndex(header, needsChangedColumn), predicates: columnWithPrefix(header, predicatesColumn), criterion: columnWithPrefix(header, criterionColumnPrefix)}
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

func (t acMaterialization) problems() []string {
	var unnamed, leftover, protected, paths []string
	for _, row := range t.rows {
		if t.disposition >= len(row) {
			continue
		}
		needs := cellAt(row, t.needs)
		switch disposition := dispositionOf(row[t.disposition]); {
		case strings.HasPrefix(disposition, "predicate"):
			found := guards.ProtectedPathsIn(needs)
			switch {
			case len(found) > 0:
				protected = append(protected, row[0])
				paths = appendNew(paths, found...)
			case len(guards.PathsIn(needs)) == 0:
				unnamed = append(unnamed, row[0])
			}
		case t.isConsoleFollowup(row) && listsPredicates(cellAt(row, t.predicates)):
			leftover = append(leftover, fmt.Sprintf("AC %s is a console-followup but lists predicate(s) %s — a console follow-up keeps no lane predicate; delete it", row[0], strings.TrimSpace(row[t.predicates])))
		}
	}
	var out []string
	if len(protected) > 0 {
		slices.Sort(paths)
		out = append(out, fmt.Sprintf("AC %s need protected path(s) %s changed — a lane builder can never change them, so the predicate is unsatisfiable (cycle 1788); "+
			"give each such AC the console-followup disposition and drop its predicate", strings.Join(protected, ", "), strings.Join(paths, ", ")))
	}
	if len(unnamed) > 0 {
		out = append(out, fmt.Sprintf("AC %s names no path in %q — a predicate AC always needs a path changed; name each one, including the wiring a plan step names", strings.Join(unnamed, ", "), needsChangedColumn))
	}
	return append(out, leftover...)
}

func (t acMaterialization) isConsoleFollowup(row []string) bool {
	return strings.HasPrefix(dispositionOf(cellAt(row, t.disposition)), "console-followup")
}

func dispositionOf(cell string) string { return strings.ToLower(strings.Trim(cell, " *`")) }

func cellAt(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return row[i]
}

func appendNew(dst []string, items ...string) []string {
	for _, it := range items {
		if !slices.Contains(dst, it) {
			dst = append(dst, it)
		}
	}
	return dst
}

var predicateIDRE = regexp.MustCompile(`\b\d{3}\b|TestC\d+_\d+`)

func listsPredicates(cell string) bool { return predicateIDRE.MatchString(cell) }

func acSection(lines []string) ([]string, bool) {
	start := slices.IndexFunc(lines, isACHeading)
	if start < 0 {
		return nil, false
	}
	level := headingLevel(lines[start])
	fenced := false
	for i, l := range lines[start+1:] {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			fenced = !fenced
		}
		if !fenced && headingLevel(l) > 0 && headingLevel(l) <= level {
			return lines[start+1 : start+1+i], true
		}
	}
	return lines[start+1:], true
}

func headingLevel(line string) int {
	n := len(line) - len(strings.TrimLeft(line, "#"))
	if n == 0 || !strings.HasPrefix(line[n:], " ") {
		return 0
	}
	return n
}

func isACHeading(line string) bool {
	if headingLevel(line) == 0 {
		return false
	}
	title := strings.ToLower(strings.TrimSpace(strings.TrimLeft(line, "#")))
	return slices.ContainsFunc(phasecontract.TDD.Sections[0].Accepted, func(accepted string) bool {
		name := strings.ToLower(strings.TrimSpace(strings.TrimLeft(accepted, "#")))
		rest, ok := strings.CutPrefix(title, name)
		suffix := strings.TrimSpace(rest)
		first, _ := utf8.DecodeRuneInString(suffix)
		return ok && (rest == "" || strings.HasPrefix(rest, ":") || (rest != suffix && strings.ContainsRune("(:-–—", first)))
	})
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

func columnWithPrefix(header []string, prefix string) int {
	return slices.IndexFunc(header, func(c string) bool { return strings.HasPrefix(strings.ToLower(c), prefix) })
}

func isSeparatorRow(cells []string) bool {
	for _, c := range cells {
		if strings.Trim(c, "-: ") != "" {
			return false
		}
	}
	return true
}
