package codeshape

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var ruleLine = regexp.MustCompile(`^\s*[-*0-9.]+\s*\**(If|When|Unless)\b`)

func RuleLines(repoRoot string) (lines, files int, err error) {
	var all []string
	for _, pattern := range []string{"agents/*.md", "skills/*/SKILL.md"} {
		m, _ := filepath.Glob(filepath.Join(repoRoot, pattern))
		all = append(all, m...)
	}
	for _, path := range all {
		b, err := os.ReadFile(path)
		if err != nil {
			return 0, 0, err
		}
		for _, l := range strings.Split(string(b), "\n") {
			if ruleLine.MatchString(l) {
				lines++
			}
		}
	}
	return lines, len(all), nil
}
