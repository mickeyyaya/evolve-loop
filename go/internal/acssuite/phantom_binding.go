package acssuite

import "regexp"

// phantomBindingRE matches the binding-assert vocabulary the cycle suites
// share: `binding test <Name> did NOT pass`. Only the name is captured; the
// surrounding wording belongs to the shared assert helper.
var phantomBindingRE = regexp.MustCompile(`binding test (Test[A-Za-z0-9_]+) did NOT pass`)

// phantomBindings returns the bound test names in output that never ran:
// reported did-NOT-pass, absent from failingTests. Deduped, first-seen order.
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
