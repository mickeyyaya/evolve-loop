package modelquery

import "strings"

// family.go implements design point D7 (FAMILY CONSTRAINT) of the
// latest-model-preference feature: two pure, deterministic, zero-I/O helpers
// that keep a CLI's candidate model set family-pure BEFORE classification /
// newest-wins.
//
// Live evidence (inbox 2026-07-02 latest-model-preference): the agy classifier
// flapped an identical list Sonnet-4.6→GPT-OSS-120B and agy's tier map carried
// Claude/GPT-OSS models — a family violation for a Gemini-only CLI. Cross-family
// coverage belongs to the cli_fallback chain, never a CLI's own tier map. A
// deterministic family filter makes that purity structural instead of an
// operator hand-correction, and it composes with the D2 NewestInLineage
// comparator (see FilterByFamily → NewestInLineage in the acceptance scenario).

// familyTokens lists, per family, the lowercase substrings that identify a raw
// model id as belonging to that family. Families are tested in this fixed order
// so classification is deterministic; the token sets are disjoint in practice
// (no known id carries tokens from two families).
var familyTokens = []struct {
	family string
	tokens []string
}{
	{"claude", []string{"opus", "sonnet", "haiku", "claude", "fable"}},
	{"gpt", []string{"gpt"}},
	{"gemini", []string{"gemini"}},
}

// FamilyOf classifies a raw model id into a family — "claude", "gpt", or
// "gemini" — or "" for an unknown/local id (e.g. an Ollama model or a novel
// vendor). Matching is a case-insensitive substring test, so a display-name
// token ("Gemini 3.1 Pro (High)") classifies the same as a bare id
// ("gemini-3.1"). An unknown id is returned as "" rather than force-fit into a
// family.
func FamilyOf(id string) string {
	lower := strings.ToLower(id)
	for _, fam := range familyTokens {
		for _, tok := range fam.tokens {
			if strings.Contains(lower, tok) {
				return fam.family
			}
		}
	}
	return ""
}

// FilterByFamily returns the subset of ids whose FamilyOf is in allowed,
// preserving input order. An empty allowed set means "no constraint" and returns
// ids unchanged (the unconstrained-CLI edge, e.g. ollama). Under a constraint,
// unknown-family ("") ids are dropped — only proven same-family models survive,
// so a list with no allowed-family model yields an empty result, never a
// passthrough no-op.
func FilterByFamily(ids []string, allowed ...string) []string {
	if len(allowed) == 0 {
		return ids
	}
	allow := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		allow[a] = true
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if allow[FamilyOf(id)] {
			out = append(out, id)
		}
	}
	return out
}
