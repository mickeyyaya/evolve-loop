package reviewcheck

import (
	"fmt"
	"path"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

const diffReviewerPrefix = "evolve-diff-"

type Persona struct {
	Path string
	Text string
}

func PersonaStrip(personas []Persona) []Refusal {
	var refusals []Refusal
	for _, p := range personas {
		for _, line := range prompts.StrippedDirectives(p.Text) {
			refusals = append(refusals, Refusal{
				Check:   CheckPersonaStrip,
				Subject: p.Path,
				Reason:  fmt.Sprintf("the directive %q is below the ## Reference Index marker, so the dispatched prompt never holds it", line),
			})
		}
		if !strings.HasPrefix(path.Base(p.Path), diffReviewerPrefix) {
			continue
		}
		for _, n := range hiddenVerdictLines(p.Text) {
			refusals = append(refusals, Refusal{
				Check:   CheckPersonaStrip,
				Subject: p.Path,
				Reason:  fmt.Sprintf("the verdict parser (reportdoc.Verdict) cannot see the Verdict: line at line %d, because a code fence or another hidden block holds it", n),
			})
		}
	}
	return refusals
}

func hiddenVerdictLines(text string) []int {
	lines := strings.Split(text, "\n")
	var candidates []int
	for i, line := range lines {
		if reportdoc.Verdict(strings.TrimSpace(line)) != "" {
			candidates = append(candidates, i)
		}
	}
	var hidden []int
	for _, i := range candidates {
		probe := make([]string, len(lines))
		copy(probe, lines)
		for _, other := range candidates {
			if other != i {
				probe[other] = ""
			}
		}
		if reportdoc.Verdict(strings.Join(probe, "\n")) == "" {
			hidden = append(hidden, i+1)
		}
	}
	return hidden
}
