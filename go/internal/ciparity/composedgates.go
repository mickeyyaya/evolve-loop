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

// GateOutcome is one composed-tree gate's result: its pass/fail Status plus
// the captured output Tail when it failed, so a decline can quote what the
// gate actually printed instead of naming only the bare status.
type GateOutcome struct {
	Status string
	Tail   string
}

// GateStatuses projects a GateOutcome map down to the status-only shape
// MissingComposedGates and the ledger's GateResults still expect, so neither
// needs to change when the gate runner starts carrying tails too.
func GateStatuses(results map[string]GateOutcome) map[string]string {
	statuses := make(map[string]string, len(results))
	for gate, outcome := range results {
		statuses[gate] = outcome.Status
	}
	return statuses
}
