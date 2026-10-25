package inputindex

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

var lineageFiles = []source{
	{path: "retrospective-report.md", kind: "retrospective", phase: "retro"},
	{path: "failure-digest.json", kind: "failure-digest", phase: "retro"},
	{path: "carryover-todos.json", kind: "carryover", phase: "retro"},
	{path: "audit-report.md", kind: "audit-report", phase: "audit"},
}

var lineageGlobs = []source{
	{path: "retrospectives/*.md", kind: "recovery-retrospective", phase: "retro"},
	{path: "failure-lesson*.yaml", kind: "failure-lesson", phase: "retro"},
}

func runsDir(root string) string {
	return filepath.Join(root, ".evolve", "runs")
}

func Lineage(projectRoot string, cycle int, itemIDs []string) ([]int, []string) {
	found := []int{}
	records, notes := readRecords(projectRoot)
	for _, r := range records {
		if r.Cycle > 0 && r.Cycle != cycle && slices.ContainsFunc(r.ItemIDs, func(id string) bool { return slices.Contains(itemIDs, id) }) {
			found = append(found, r.Cycle)
		}
	}
	chain, chainNotes := continuationCycles(projectRoot, cycle, itemIDs)
	found = append(found, chain...)
	notes = append(notes, chainNotes...)
	slices.Sort(found)
	return slices.Compact(found), notes
}

func continuationCycles(projectRoot string, cycle int, itemIDs []string) ([]int, []string) {
	var out []int
	var notes []string
	for _, id := range itemIDs {
		c, ok, err := continuation.ReadRegistryEntry(projectRoot, id)
		if err != nil {
			notes = append(notes, "the continuation registry is unreadable, so the chain of "+id+" is not in the lineage: "+err.Error())
			continue
		}
		if ok && c.Cycle > 0 && c.Cycle != cycle {
			out = append(out, c.Cycle)
		}
	}
	return out, notes
}

func lineageSources(projectRoot string, lineage []int, firstAttempt int) []source {
	var out []source
	for i, n := range lineage {
		dir := filepath.Join(runsDir(projectRoot), "cycle-"+strconv.Itoa(n))
		var cycleSources []source
		for _, f := range lineageFiles {
			f.path = filepath.Join(dir, f.path)
			cycleSources = append(cycleSources, f)
		}
		for _, g := range lineageGlobs {
			matches, _ := filepath.Glob(filepath.Join(dir, g.path))
			for _, m := range matches {
				g.path = m
				cycleSources = append(cycleSources, g)
			}
		}
		for _, s := range cycleSources {
			s.class, s.cycle, s.attempt = ClassLineage, n, firstAttempt+i
			out = append(out, s)
		}
	}
	return out
}

func lessonSources(projectRoot string, itemIDs []string, lineage []int) []source {
	names := slices.Clone(itemIDs)
	for _, n := range lineage {
		names = append(names, "cycle-"+strconv.Itoa(n))
	}
	if len(names) == 0 {
		return nil
	}
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = regexp.QuoteMeta(n)
	}
	named := regexp.MustCompile(`(^|[^A-Za-z0-9_-])(` + strings.Join(quoted, "|") + `)($|[^A-Za-z0-9_-])`)
	lessons, _ := filepath.Glob(filepath.Join(projectRoot, ".evolve", "instincts", "lessons", "*.yaml"))
	var out []source
	for _, path := range lessons {
		if body, err := os.ReadFile(path); err == nil && named.Match(body) {
			out = append(out, source{path: path, class: ClassLineage, kind: "lesson"})
		}
	}
	return out
}
