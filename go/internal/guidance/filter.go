package guidance

import "slices"

type Filter struct {
	Audience string
	Phase    string
	Family   string
}

func Select(rules []Rule, f Filter) []Rule {
	var out []Rule
	for _, r := range Active(rules) {
		if matchAudience(r.Audience, f.Audience) && matchOpen(r.Phases, f.Phase) && matchOpen(r.Families, f.Family) {
			out = append(out, r)
		}
	}
	return out
}

func matchAudience(audience []string, want string) bool {
	return want == "" || slices.Contains(audience, want) || slices.Contains(audience, "all")
}

func matchOpen(list []string, want string) bool {
	return want == "" || len(list) == 0 || slices.Contains(list, want)
}
