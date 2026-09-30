package modelquery

import "strings"

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
	lower := strings.ToLower(id)
	for _, fam := range familyTokens {
		for _, tok := range fam.tokens {
			if strings.Contains(lower, tok) {
				return fam.family
			}
		}
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
