package phasecontract

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

func RoundArchiveFilename(name string, round int) string {
	ext := filepath.Ext(name)
	return fmt.Sprintf("%s.round%d%s", name[:len(name)-len(ext)], round, ext)
}

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

func PromptArtifactFilename(agent string) string {
	return agent + "-prompt.txt"
}
