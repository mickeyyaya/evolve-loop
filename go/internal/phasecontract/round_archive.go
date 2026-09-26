package phasecontract

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// RoundArchiveFilename names a repair round's archived copy: audit-report.md round 2 becomes audit-report.round2.md.
func RoundArchiveFilename(name string, round int) string {
	ext := filepath.Ext(name)
	return fmt.Sprintf("%s.round%d%s", name[:len(name)-len(ext)], round, ext)
}

// ParseRoundArchive inverts RoundArchiveFilename, returning the round index an archive of liveName carries.
func ParseRoundArchive(filename, liveName string) (round int, ok bool) {
	ext := filepath.Ext(liveName)
	stem := liveName[:len(liveName)-len(ext)] + ".round"
	if !strings.HasPrefix(filename, stem) || !strings.HasSuffix(filename, ext) {
		return 0, false
	}
	digits := filename[len(stem) : len(filename)-len(ext)]
	n, err := strconv.Atoi(digits)
	if err != nil || n < 1 || digits != strconv.Itoa(n) {
		return 0, false
	}
	return n, true
}

// PromptArtifactFilename names the dispatched prompt the bridge writes beside the phase artifacts.
func PromptArtifactFilename(agent string) string {
	return agent + "-prompt.txt"
}
