package dossier

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
)

// delivery.go — the dossier's readers for the two facts that make a cycle
// record answerable: WHAT it was for (the committed task set) and WHETHER it
// delivered (the shipped commit). Both follow the package's established
// evidence discipline (see ciWatchRecord): read the phase's OWN artifact,
// return not-ok on absence, and never fabricate.
//
// Before this, Dossier.CommitSHA existed in the schema but no producer ever
// set it, and the committed task set was not recorded at all — so cycle-1623's
// record could not distinguish "shipped 922 lines" from "shipped nothing", and
// a reader (human or ship-rate query) had to reconstruct it from git.

// ShipBindingFile is the ship phase's own proof of delivery: written only after
// the commit exists, so its presence is the delivery fact and its `commit` is
// the identity. Same filename the lost-landing floor keys on
// (core.detectLostLanding), so the two cannot disagree about what "landed" means.
const ShipBindingFile = "ship-binding.json"

// ShipBinding is the ship phase's proof-of-delivery sidecar. It is declared
// HERE, and phases/ship marshals THIS type (that package already imports
// dossier; the reverse edge does not exist), so the writer and every reader
// share one definition and a field rename becomes a compile error.
//
// It used to be an inline map[string]any at the write site with each reader
// re-typing the key names. That is not a theoretical risk: this very change
// first read "commit" instead of "commit_sha", produced an empty delivery
// record for every cycle, and passed its own unit tests — because the tests
// re-typed the same wrong assumption. Only a real artifact caught it.
type ShipBinding struct {
	AuditBoundTreeSHA string `json:"audit_bound_tree_sha,omitempty"`
	TreeSHACommitted  string `json:"tree_sha_committed,omitempty"`
	// CommitSHA deliberately has NO omitempty: it is the delivery identity, and
	// its emptiness is the "did not deliver" signal shippedCommit keys on. The
	// asymmetry with the other fields is intentional — do not tidy it away.
	CommitSHA string `json:"commit_sha"`
	Cycle     int    `json:"cycle,omitempty"`
}

// shippedCommit returns the commit and committed tree the ship phase bound to
// this cycle. ok is false when the binding is absent or carries no commit —
// the cycle did not deliver, and a dossier must say so by omission rather than
// by invention.
func shippedCommit(workspace string) (commit, tree string, ok bool) {
	body, err := os.ReadFile(filepath.Join(workspace, ShipBindingFile))
	if err != nil {
		return "", "", false
	}
	var binding ShipBinding
	if json.Unmarshal(body, &binding) != nil {
		return "", "", false
	}
	commit = strings.TrimSpace(binding.CommitSHA)
	return commit, strings.TrimSpace(binding.TreeSHACommitted), commit != ""
}

// committedTasks returns the task ids this cycle is bound to, delegating to
// internal/committedset — the ONE projection of "what did this cycle commit
// to" (lane pin, else triage's top_n, minus deferrals). The dossier had its
// own top_n-only parser for exactly one review round; on runtime cycle-1621
// that read one member where the lane pin named two, and 17 of the last 20
// cycles carried a pin. A record must not report a commitment it did not read.
func committedTasks(workspace string) ([]string, bool) {
	return committedset.Committed(workspace)
}
