// Package digest projects the role-scoped sections of an instruction document.
// It returns only the blocks tagged for one role, never the full source.
package digest

import (
	"fmt"
	"strings"
)

const (
	markerPrefix = "<!-- digest:role="
	markerSuffix = " -->"
	markerEnd    = "<!-- /digest -->"
)

type Outcome string

const (
	OutcomeMatched   Outcome = "matched"
	OutcomeNoMatch   Outcome = "no_match"
	OutcomeMalformed Outcome = "malformed"
)

type Result struct {
	Outcome Outcome
	Digest  []byte
	Err     error
}

func Materialize(source []byte, role string) Result {
	projected, err := ProjectDigest(source, role)
	if err != nil {
		return Result{Outcome: OutcomeMalformed, Err: err}
	}
	if len(strings.TrimSpace(string(projected))) == 0 {
		return Result{Outcome: OutcomeNoMatch, Digest: projected}
	}
	return Result{Outcome: OutcomeMatched, Digest: projected}
}

type ShadowRecord struct {
	FullBytes   int
	DigestBytes int
	Parity      bool
	Outcome     Outcome
}

func NewShadowRecord(full []byte, result Result) ShadowRecord {
	return ShadowRecord{
		FullBytes:   len(full),
		DigestBytes: len(result.Digest),
		Parity: result.Outcome == OutcomeMatched && result.Err == nil &&
			len(strings.TrimSpace(string(result.Digest))) > 0 && len(result.Digest) < len(full),
		Outcome: result.Outcome,
	}
}

func ProjectDigest(source []byte, role string) ([]byte, error) {
	s := string(source)
	var out strings.Builder
	pos := 0

	for {
		rel := strings.Index(s[pos:], markerPrefix)
		if rel == -1 {
			break
		}
		markerStart := pos + rel
		rolesStart := markerStart + len(markerPrefix)

		suffixRel := strings.Index(s[rolesStart:], markerSuffix)
		if suffixRel == -1 {
			return nil, fmt.Errorf("digest: malformed marker at offset %d: missing %q terminator on opening tag", markerStart, markerSuffix)
		}
		rolesRaw := s[rolesStart : rolesStart+suffixRel]
		contentStart := rolesStart + suffixRel + len(markerSuffix)

		endRel := strings.Index(s[contentStart:], markerEnd)
		if endRel == -1 {
			return nil, fmt.Errorf("digest: unterminated marker for role(s) %q opened at offset %d: no matching %q before EOF", rolesRaw, markerStart, markerEnd)
		}
		body := s[contentStart : contentStart+endRel]

		if blockMatchesRole(rolesRaw, role) {
			out.WriteString(body)
		}

		pos = contentStart + endRel + len(markerEnd)
	}

	return []byte(out.String()), nil
}

func blockMatchesRole(rolesRaw, role string) bool {
	for _, r := range strings.Split(rolesRaw, ",") {
		if strings.TrimSpace(r) == role {
			return true
		}
	}
	return false
}
