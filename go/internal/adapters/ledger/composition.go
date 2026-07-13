// composition.go — kernel verification of composition-verdict entries
// (merge ladder RUNG 0, cycle-786; knowledge-base/research/
// merge-concurrency-2026: review verdicts follow the CHANGE via git
// patch-id, gates follow the TREE).
//
// A composition-verdict entry records that an audit verdict carried
// forward across a conflict-free trivial rebase. It is deterministic and
// kernel-recomputable: anyone can re-derive the patch-id of its two
// persisted diff artifacts and compare against the recorded patch_id —
// zero LLM tokens. Verify/VerifyDeep do exactly that for every such
// entry; a mismatch (drifted composed diff, forged patch_id, missing
// artifact) is tampering and breaks the chain like any hash break.
package ledger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// CompositionVerdictKind is the ledger entry kind recording a
// trivial-rebase audit carry-forward (ship's fast path reads it, ledger
// verify kernel-recomputes it).
const CompositionVerdictKind = "composition-verdict"

// compositionFields is the kernel-recomputable subset of a
// composition-verdict line.
type compositionFields struct {
	PatchID          string `json:"patch_id"`
	AuditedDiffPath  string `json:"audited_diff_path"`
	ComposedDiffPath string `json:"composed_diff_path"`
}

// PatchID pipes a unified diff through `git patch-id --stable` and returns
// the patch-id — the offset-insensitive content identity of a change
// (bors-style patch identity; merge-concurrency-2026). patch-id is a pure
// stdin filter: no repo access, works in any directory.
func PatchID(diff []byte) (string, error) {
	cmd := exec.Command("git", "patch-id", "--stable")
	cmd.Stdin = bytes.NewReader(diff)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git patch-id --stable: %w", err)
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "", fmt.Errorf("git patch-id --stable: empty output (empty diff?)")
	}
	return fields[0], nil
}

// verifyCompositionLine kernel-recomputes one composition-verdict line:
// both persisted diff artifacts must recompute to the recorded patch_id.
// Any failure wraps core.ErrLedgerChainBroken so `evolve ledger verify`
// exits 2 exactly as it does for a hash-chain break.
func verifyCompositionLine(i int, line []byte) error {
	var f compositionFields
	if err := json.Unmarshal(line, &f); err != nil {
		return fmt.Errorf("%w: line %d composition-verdict unmarshal: %v", core.ErrLedgerChainBroken, i, err)
	}
	if f.PatchID == "" {
		return fmt.Errorf("%w: line %d composition-verdict has no patch_id — not kernel-recomputable", core.ErrLedgerChainBroken, i)
	}
	for _, p := range []struct{ label, path string }{
		{"audited_diff_path", f.AuditedDiffPath},
		{"composed_diff_path", f.ComposedDiffPath},
	} {
		diff, err := os.ReadFile(p.path)
		if err != nil {
			return fmt.Errorf("%w: line %d composition-verdict %s unreadable: %v", core.ErrLedgerChainBroken, i, p.label, err)
		}
		got, err := PatchID(diff)
		if err != nil {
			return fmt.Errorf("%w: line %d composition-verdict %s: %v", core.ErrLedgerChainBroken, i, p.label, err)
		}
		if got != f.PatchID {
			return fmt.Errorf("%w: line %d composition-verdict tampered: %s recomputes patch-id %s, entry records %s",
				core.ErrLedgerChainBroken, i, p.label, got, f.PatchID)
		}
	}
	return nil
}
