package dossier

import (
	"fmt"
	"path"
)

func CorpusPair(cycle int) ([]string, string) {
	spec := corpusPathspec(fmt.Sprintf("cycle-%d", cycle))
	return []string{spec + ".json", spec + ".md"}, closeoutMessage(spec)
}

func closeoutMessage(base string) string {
	return fmt.Sprintf("dossier: %s closeout", path.Base(base))
}
