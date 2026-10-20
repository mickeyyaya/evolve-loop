package builddiff

import (
	"slices"
	"strings"
)

func replaceChanges(report string, s Summary, short string) string {
	lines := strings.Split(report, "\n")
	start := slices.Index(lines, changesHeading)
	if start < 0 {
		return strings.TrimRight(report, "\n") + "\n\n" + changesBlock(s, short, nil) + "\n"
	}
	end := start + 1
	for end < len(lines) && !strings.HasPrefix(lines[end], "## ") {
		end++
	}
	from, to := tableSpan(lines, start+1, end)
	described := descriptions(lines[from:to])
	block := strings.Split(changesBlock(s, short, described), "\n")
	return strings.Join(slices.Concat(lines[:start], block, lines[start+1:from], lines[to:]), "\n")
}

func tableSpan(lines []string, from, end int) (int, int) {
	for i := from; i < end; i++ {
		switch {
		case strings.HasPrefix(lines[i], "```"):
			fenceEnd := i + 1
			for fenceEnd < end && !strings.HasPrefix(lines[fenceEnd], "```") {
				fenceEnd++
			}
			return i, min(fenceEnd+1, end)
		case strings.HasPrefix(strings.TrimSpace(lines[i]), "|"):
			last := i
			for last < end && strings.HasPrefix(strings.TrimSpace(lines[last]), "|") {
				last++
			}
			return i, last
		}
	}
	return from, from
}

func changesBlock(s Summary, short string, described map[string]string) string {
	rows := []string{changesHeading, "<!-- host-derived from git diff " + short + ": the table is the diff of the cycle -->", "```tsv", "Action\tFile\tDescription"}
	inDiff := map[string]bool{}
	for _, c := range s.Changes {
		desc, ok := described[c.Path]
		if !ok {
			desc = notDescribed
		}
		inDiff[c.Path] = true
		rows = append(rows, c.Action+"\t"+c.Path+"\t"+desc)
	}
	rows = append(rows, "```")
	var gone []string
	for path := range described {
		if !inDiff[path] {
			gone = append(gone, "`"+path+"`")
		}
	}
	if len(gone) > 0 {
		slices.Sort(gone)
		rows = append(rows, "", "Declared by the build but not in the diff: "+strings.Join(gone, ", "))
	}
	return strings.Join(rows, "\n")
}

func descriptions(section []string) map[string]string {
	described := map[string]string{}
	for _, line := range section {
		cells := rowCells(line)
		if len(cells) < 3 || strings.EqualFold(cells[0], "action") || strings.Trim(cells[0], "-: ") == "" {
			continue
		}
		described[strings.Trim(cells[1], "` ")] = cells[2]
	}
	return described
}

func rowCells(line string) []string {
	if strings.Contains(line, "\t") {
		return strings.SplitN(line, "\t", 3)
	}
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") {
		return nil
	}
	cells := strings.Split(strings.Trim(trimmed, "|"), "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}
