package core

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// placeholderTokenRE matches an UPPER_SNAKE template token ending in
// _PLACEHOLDER (FULLSUITE_PLACEHOLDER, ACS_RESULT_PLACEHOLDER): the shape a
// builder's own report scaffolding uses for a slot it must overwrite with an
// executed result (no shipped persona emits the token — cycle 1679 minted it
// itself — so the grammar here is the only home of the rule). Prose mentioning a "placeholder", a lowercase
// identifier, or a token that merely starts with PLACEHOLDER is not a slot.
var placeholderTokenRE = regexp.MustCompile(`\b[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)*_PLACEHOLDER\b`)

// PlaceholderTokenFailures is the build handoff floor's deterministic check
// that no template slot survived into build-report.md. Cycle 1679 round 5
// handed off "`go test -count=1 ./...` → FULLSUITE_PLACEHOLDER" under a
// heading promising every number was executed; the floor passed it and the
// audit spent a round naming it (M3 verification-gap). One failure per token
// per line, naming the report line, so the correction ladder's instruction is
// exact. No report → nothing to refuse (the deliverables gate owns absence).
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
