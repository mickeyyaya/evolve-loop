package core

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

var placeholderTokenRE = regexp.MustCompile(`\b[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)*_PLACEHOLDER\b`)

var correctionHeadingRE = regexp.MustCompile(`^## Correction [0-9]+\b`)

var backtickSpanRE = regexp.MustCompile("`[^`]*`")

func PlaceholderTokenFailures(_ context.Context, in ReviewInput) []string {
	if in.Workspace == "" {
		return nil
	}
	body, ok := readBuildReport(in.Workspace)
	if !ok {
		return nil
	}
	name := phasecontract.ArtifactFilename(string(PhaseBuild))
	var failures []string
	inCorrection, inFence := false, false
	for i, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			if inCorrection {
				continue
			}
		}
		isHeading := strings.HasPrefix(line, "# ") || strings.HasPrefix(line, "## ")
		if isHeading && (!inFence || inCorrection) {
			inFence = false
			inCorrection = correctionHeadingRE.MatchString(line)
		}
		if inCorrection {
			if inFence {
				continue
			}
			line = backtickSpanRE.ReplaceAllString(line, "")
		}
		for _, tok := range placeholderTokenRE.FindAllString(line, -1) {
			failures = append(failures, fmt.Sprintf("%s:%d carries the unsubstituted template token %s — replace it with the executed result it stands for (or delete the line); a template slot is not evidence", name, i+1, tok))
		}
	}
	return failures
}
