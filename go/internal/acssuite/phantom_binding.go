package acssuite

import "regexp"

var phantomBindingRE = regexp.MustCompile(`binding test (Test[A-Za-z0-9_]+) did NOT pass`)

func phantomBindings(output string, failingTests []string) []string {
	ms := phantomBindingRE.FindAllStringSubmatch(output, -1)
	if len(ms) == 0 {
		return nil
	}
	failed := make(map[string]bool, len(failingTests))
	for _, f := range failingTests {
		failed[f] = true
	}
	var out []string
	seen := map[string]bool{}
	for _, m := range ms {
		name := m[1]
		if failed[name] || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}
