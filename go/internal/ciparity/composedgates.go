// composedgates.go — the composed-tree gate contract for the trivial-rebase
// audit carry-forward (merge ladder RUNG 0, cycle-786). Gates bind to the
// TREE: even when the audit verdict follows the change (patch-id) across a
// clean rebase, the full native gate set must re-run green on the composed
// tree — via the same CI-parity runners the cycle audit uses (ADR-0069), not
// a new gate implementation.
package ciparity

// RequiredComposedGates is the full native gate set a composed tree must
// record as "pass" in a composition-verdict ledger entry before ship's
// trivial-rebase fast path may accept it: compile, go test, ACS suite,
// apicover — the whole-repo CI-parity command set.
var RequiredComposedGates = []string{"compile", "test", "acs", "apicover"}

// MissingComposedGates returns the required composed-tree gates that results
// does not record as "pass" — absent keys and non-"pass" values both count.
// nil means the full native gate set is green and the fast path may stay
// open. Pure; no I/O (this package is a leaf — the audit phase's ciparity
// runners produce the results, ship consumes the recorded entry).
func MissingComposedGates(results map[string]string) []string {
	var missing []string
	for _, g := range RequiredComposedGates {
		if results[g] != "pass" {
			missing = append(missing, g)
		}
	}
	return missing
}
