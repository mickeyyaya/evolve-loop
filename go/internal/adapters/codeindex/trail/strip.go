package trail

import (
	"slices"
	"strings"
)

var (
	instructionPrefixes = []string{"[trail]", "also matched:"}
)

func cleanLines(text string) []string {
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if !instructsTheAgent(line) {
			kept = append(kept, line)
		}
	}
	return kept
}

func instructsTheAgent(line string) bool {
	trimmed := strings.TrimSpace(line)
	return slices.ContainsFunc(instructionPrefixes, func(p string) bool { return strings.HasPrefix(trimmed, p) })
}
