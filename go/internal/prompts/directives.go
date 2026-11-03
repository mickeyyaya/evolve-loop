package prompts

import (
	"regexp"
	"strings"
)

var operationalDirective = regexp.MustCompile(`MANDATORY|STOP CRITERION|Completion Gates|force-FAIL|REQUIRED|POSTHOC|Verdict Rules|Constitutional audit`)

func StrippedDirectives(body string) []string {
	kept := StripOnDemandSections(body)
	var lost []string
	for _, line := range strings.Split(body[len(kept):], "\n") {
		if operationalDirective.MatchString(line) && !strings.Contains(kept, line) {
			lost = append(lost, strings.TrimSpace(line))
		}
	}
	return lost
}
