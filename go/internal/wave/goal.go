package wave

import (
	"fmt"
	"strings"
)

type Facts struct {
	Wave      int
	RunID     string
	Cycles    []Cycle
	MergedPRs []string
	Limit     int
}

func (f Facts) Outcome() string {
	if len(f.Cycles) == 0 {
		return "no cycle ran"
	}
	shipped := 0
	for _, c := range f.Cycles {
		if c.Shipped {
			shipped++
		}
	}
	return fmt.Sprintf("%d of %d cycles shipped", shipped, len(f.Cycles))
}

func (f Facts) Section() string {
	lines := []string{fmt.Sprintf("Wave %d facts (run %s): %s.", f.Wave, f.RunID, f.Outcome())}
	shown := f.Cycles
	if over := len(shown) - f.Limit; f.Limit > 0 && over > 0 {
		lines = append(lines, fmt.Sprintf("- %d older cycles are not shown.", over))
		shown = shown[over:]
	}
	for _, c := range shown {
		lines = append(lines, "- "+cycleLine(c))
	}
	prs := "none"
	if len(f.MergedPRs) > 0 {
		prs = "#" + strings.Join(f.MergedPRs, ", #")
	}
	return strings.Join(append(lines, "- Pull requests merged at this boundary: "+prs+"."), "\n")
}

func cycleLine(c Cycle) string {
	parts := []string{fmt.Sprintf("Cycle %d: final verdict %s.", c.ID, c.Verdict)}
	if c.Verdict == "" {
		parts = []string{fmt.Sprintf("Cycle %d: no final verdict.", c.ID)}
	}
	if c.Phase != "" && !c.Shipped {
		parts = append(parts, "Last phase: "+c.Phase+".")
	}
	if c.Shipped {
		parts = append(parts, "It shipped.")
	} else {
		parts = append(parts, "It did not ship.")
	}
	if c.QuotaPauses > 0 {
		parts = append(parts, fmt.Sprintf("Quota pauses: %d.", c.QuotaPauses))
	}
	return strings.Join(parts, " ")
}

type GoalInput struct {
	Next     int
	Standing string
	Last     *Facts
	Notes    []Note
	History  []Record
}

func Compose(in GoalInput) string {
	blocks := []string{fmt.Sprintf("Wave %d.", in.Next), strings.TrimSpace(in.Standing)}
	if in.Last == nil {
		blocks = append(blocks, "No earlier wave is recorded.")
	} else {
		blocks = append(blocks, in.Last.Section())
	}
	if len(in.Notes) > 0 {
		blocks = append(blocks, bulleted(fmt.Sprintf("Operator notes for wave %d:", in.Next), noteTexts(in.Notes)))
	}
	if len(in.History) > 0 {
		blocks = append(blocks, bulleted("Earlier waves:", historyLines(in.History)))
	}
	return strings.Join(blocks, "\n\n") + "\n"
}

func bulleted(header string, items []string) string {
	lines := []string{header}
	for _, it := range items {
		lines = append(lines, "- "+it)
	}
	return strings.Join(lines, "\n")
}

func noteTexts(notes []Note) []string {
	out := make([]string, 0, len(notes))
	for _, n := range notes {
		out = append(out, n.Text)
	}
	return out
}

func historyLines(records []Record) []string {
	out := make([]string, 0, len(records))
	for _, r := range records {
		outcome := r.Outcome
		if outcome == "" {
			outcome = "outcome not recorded"
		}
		out = append(out, fmt.Sprintf("Wave %d (run %s): %s.", r.Number, r.RunID, outcome))
	}
	return out
}
