package stelint

import (
	"fmt"
	"strings"
)

func ParseWordTable(markdown []byte) ([]Substitution, error) {
	lines := strings.Split(string(markdown), "\n")
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if isHeading(t) && headingText(t) == WordTableHeading {
			return tableUnderHeading(lines[i+1:])
		}
	}
	return nil, fmt.Errorf("the standard has no %q section", WordTableHeading)
}

func tableUnderHeading(lines []string) ([]Substitution, error) {
	var subs []Substitution
	rows := 0
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if isHeading(t) || (rows > 0 && !strings.HasPrefix(t, "|")) {
			break
		}
		if !strings.HasPrefix(t, "|") {
			continue
		}
		rows++
		cells := splitCells(t)
		if rows == 1 || isSeparatorRow(cells) {
			continue
		}
		sub, err := substitution(cells)
		if err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	if len(subs) == 0 {
		return nil, fmt.Errorf("the %q section has no table rows", WordTableHeading)
	}
	return subs, nil
}

func substitution(cells []string) (Substitution, error) {
	clean := func(c string) string { return strings.Trim(strings.TrimSpace(c), "`") }
	if len(cells) < 2 || clean(cells[0]) == "" || clean(cells[1]) == "" {
		return Substitution{}, fmt.Errorf("the %q table has a row without a word and its replacement: %q", WordTableHeading, strings.Join(cells, "|"))
	}
	return Substitution{Phrase: clean(cells[0]), Replacement: clean(cells[1])}, nil
}
