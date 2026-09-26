package core

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// placeholderTokenRE matches an UPPER_SNAKE token ending in _PLACEHOLDER, the
// slot shape of a builder's own report scaffolding. No persona emits it, so
// this grammar is the rule's only home. Prose and lowercase names never match.
var placeholderTokenRE = regexp.MustCompile(`\b[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)*_PLACEHOLDER\b`)

// PlaceholderTokenFailures names each template slot left in the build report; a missing report is the deliverables gate's to refuse.
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
	for i, line := range strings.Split(body, "\n") {
		for _, tok := range placeholderTokenRE.FindAllString(line, -1) {
			failures = append(failures, fmt.Sprintf("%s:%d carries the unsubstituted template token %s — replace it with the executed result it stands for (or delete the line); a template slot is not evidence", name, i+1, tok))
		}
	}
	return failures
}
