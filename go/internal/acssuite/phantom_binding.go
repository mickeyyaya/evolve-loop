package acssuite

// phantom_binding.go — a red whose bound test NEVER RAN is named as such.
//
// Cycle-suite predicates commonly delegate to named "binding" tests in a
// production package (`go test -run '^(Name)$' <pkg>`, then require the
// `--- PASS: Name` line). That bind is identity by NAME across a package
// boundary, and the name can silently stop resolving — a continuation cycle
// renames the tests, or a builder never writes them (cycle-1532). The predicate
// then reds FOREVER: `go test -run` on a pattern matching nothing exits 0 with
// "no tests to run", the assert reports "did NOT pass", and the red reaches the
// EGPS gate as a bare count nobody can act on. On a continuation chain that red
// is ABSORBING — the 1539-1546 streak's dominant class, cured in PR #486 by a
// 2-line binding repoint that took a console session to diagnose.
//
// The discriminator is deliberately NOT the "no tests to run" string alone:
// that only appears when NOTHING matched. The robust rule covers the partial
// shape too (siblings matched, the renamed one silently didn't): a name the
// predicate reports as did-NOT-pass that is ALSO absent from the failing set
// never ran at all. FailingTests is the authority because it is extracted from
// the FULL output before the excerpt cap (cycles 1107/1116/1123) — truncation
// cannot fake a phantom.
//
// Anti-gaming boundary, stated: classification NEVER changes the verdict. A
// phantom red is still red — skipping it would make deleting a test the way to
// green a gate. What changes is that the red now names its own cure.

import "regexp"

// phantomBindingRE matches the binding-assert vocabulary the cycle suites
// share: `binding test <Name> did NOT pass`. Only the name is captured; the
// surrounding wording is the shared assert helper's and is pinned by the
// real-artifact test.
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
