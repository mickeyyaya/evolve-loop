package dossier

import (
	"fmt"
	"path"
	"slices"
)

func CorpusPair(cycle int) ([]string, string) {
	spec := corpusPathspec(fmt.Sprintf("cycle-%d", cycle))
	return []string{spec + ".json", spec + ".md"}, closeoutMessage(spec)
}

func closeoutMessage(base string) string {
	return fmt.Sprintf("dossier: %s closeout", path.Base(base))
}

func IsCloseoutCommit(subject string, paths []string) bool {
	var cycle int
	if _, err := fmt.Sscanf(subject, "dossier: cycle-%d closeout", &cycle); err != nil {
		return false
	}
	pair, message := CorpusPair(cycle)
	if subject != message || len(paths) == 0 {
		return false
	}
	for _, p := range paths {
		if !slices.Contains(pair, p) {
			return false
		}
	}
	return true
}
