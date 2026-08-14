// Package digest projects a role-scoped subset out of an SSOT
// (single-source-of-truth) instruction document — the cycle-1391 first
// increment of inbox item tokenopt-role-scoped-instruction-digests. Full
// phase-instruction skill files (skills/loop/evolve-scout.md etc.) are
// injected verbatim per phase per cycle even though narrow-role phases
// (e.g. scout) never act on most of the cross-cutting ship-gate/audit
// content in them. Rather than hand-maintain a per-role copy of each skill
// file, authors tag role-specific sections inline with a marker pair:
//
//	<!-- digest:role=scout -->
//	... content scout needs ...
//	<!-- /digest -->
//
// ProjectDigest scans the source for these marker pairs and returns the
// concatenation of every block whose comma-separated role list contains the
// requested role. Untagged prose and blocks tagged for other roles are
// excluded — a role with no matching block gets an empty digest, never a
// silent fallback to the full source, so a no-op/pass-through
// implementation cannot trivially satisfy this contract.
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

// Outcome classifies a role projection without falling back to the source.
type Outcome string

const (
	OutcomeMatched   Outcome = "matched"
	OutcomeNoMatch   Outcome = "no_match"
	OutcomeMalformed Outcome = "malformed"
)

// Result is the classified output of Materialize.
type Result struct {
	Outcome Outcome
	Digest  []byte
	Err     error
}

// Materialize projects source for role and classifies the result.
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

// ShadowRecord records whether a projection is safe to substitute.
type ShadowRecord struct {
	FullBytes   int
	DigestBytes int
	Parity      bool
	Outcome     Outcome
}

// NewShadowRecord derives size and safety facts without changing live input.
func NewShadowRecord(full []byte, result Result) ShadowRecord {
	return ShadowRecord{
		FullBytes:   len(full),
		DigestBytes: len(result.Digest),
		Parity: result.Outcome == OutcomeMatched && result.Err == nil &&
			len(strings.TrimSpace(string(result.Digest))) > 0 && len(result.Digest) < len(full),
		Outcome: result.Outcome,
	}
}

// ProjectDigest scans source for "<!-- digest:role=ROLE[,ROLE2,...] -->
// ... <!-- /digest -->" marker pairs and returns the concatenated content of
// every block whose role list contains role. Blocks tagged for other roles,
// and any content outside a marker pair, are excluded. A source with no
// block tagged for role yields an empty (non-nil) result, not an error and
// not the full source. An opening marker with no matching "<!-- /digest -->"
// before EOF is a malformed-input error, not a silent truncation.
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

// blockMatchesRole reports whether role appears in rolesRaw, a
// comma-separated role list (whitespace around each entry is trimmed).
func blockMatchesRole(rolesRaw, role string) bool {
	for _, r := range strings.Split(rolesRaw, ",") {
		if strings.TrimSpace(r) == role {
			return true
		}
	}
	return false
}
