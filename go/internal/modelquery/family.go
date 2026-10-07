package modelquery

import (
	"slices"
	"strings"
)

var familyTokens = []struct {
	family string
	tokens []string
}{
	{"claude", []string{"opus", "sonnet", "haiku", "claude", "fable"}},
	{"gpt", []string{"gpt"}},
	{"gemini", []string{"gemini"}},
}

const unknownFamily = ""

func FamilyOf(id string) string {
	if families := FamiliesIn(id); len(families) > 0 {
		return families[0]
	}
	return unknownFamily
}

func FilterByFamily(ids []string, allowed ...string) []string {
	isUnconstrained := len(allowed) == 0
	if isUnconstrained {
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

func FamiliesIn(label string) []string {
	lower := strings.ToLower(label)
	var families []string
	for _, fam := range familyTokens {
		if slices.ContainsFunc(fam.tokens, func(tok string) bool { return strings.Contains(lower, tok) }) {
			families = append(families, fam.family)
		}
	}
	return families
}
