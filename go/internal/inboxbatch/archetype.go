package inboxbatch

import "strings"

const (
	// operatorStateClass is the declared class an operator-state item carries.
	operatorStateClass = "pipeline-architecture"
	// evolveStatePrefix bounds what counts as live runtime state. The trailing
	// separator is load-bearing: ".evolvex/" is NOT ".evolve/".
	evolveStatePrefix = ".evolve/"
)

// IsOperatorState reports whether an item is the operator-state archetype: a
// declared pipeline-architecture item whose deliverable is entirely a mutation
// of live .evolve/ runtime state, with no source change. Such items have no
// build/audit surface, so they can eventually be applied natively instead of
// paying a full worktree pipeline.
//
// Pure and conservative — it answers false whenever the evidence is short:
//   - any file outside .evolve/ (source-touching ops are NEVER an operator-state
//     manifest — one source path disqualifies the whole item),
//   - an empty file list (nothing declared is not a state mutation),
//   - any other class, including an item that declares none.
//
// Both degenerate answers fail toward the normal pipeline, which is the safe
// direction: a missed detection costs a cycle, a false detection would skip
// review on a source change.
func IsOperatorState(it Item) bool {
	if it.Class != operatorStateClass || len(it.Files) == 0 {
		return false
	}
	for _, f := range it.Files {
		if !strings.HasPrefix(f, evolveStatePrefix) {
			return false
		}
	}
	return true
}
