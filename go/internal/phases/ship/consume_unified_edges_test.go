package ship

import (
	"os"
	"path/filepath"
	"testing"
)

// The two member states consumeUnified decides beyond the TDD contract: a
// member an earlier landing already consumed counts as closed, and a member
// this ship does not commit keeps the whole unit open.
func TestConsumeCommittedItems_UnifiedMemberEdges(t *testing.T) {
	pair := []string{unifiedMemberA, unifiedMemberB}

	t.Run("a member already in consumed/ counts as closed", func(t *testing.T) {
		root, ws, base, _ := unifiedConsumeFixture(t, unifiedDecision(t, pair, pair, true), pair)
		inbox := filepath.Join(root, ".evolve", "inbox")
		earlier := filepath.Join(inbox, "consumed", base[unifiedMemberB])
		if err := os.MkdirAll(filepath.Dir(earlier), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(filepath.Join(inbox, base[unifiedMemberB]), earlier); err != nil {
			t.Fatal(err)
		}
		opts, res := runUnifiedConsume(root, ws, stubRunner)
		assertConsumed(t, opts, res, root, unifiedMemberA, base[unifiedMemberA])
		if unifiedWarn(res, unifiedMemberB) {
			t.Errorf("an already-consumed member must not fail the unit; logs: %v", res.Logs)
		}
		if sanctionsBase(opts, base[unifiedMemberB]) {
			t.Errorf("this ship did not move %s, so it must not sanction its paths: %v", unifiedMemberB, opts.internalConsumedPaths)
		}
	})

	t.Run("a member outside the committed set keeps every member pickable", func(t *testing.T) {
		root, ws, base, orig := unifiedConsumeFixture(t, unifiedDecision(t, []string{unifiedMemberA}, pair, true), pair)
		opts, res := runUnifiedConsume(root, ws, stubRunner)
		for _, id := range pair {
			assertPickable(t, opts, res, root, id, base[id], orig[id])
		}
		if !unifiedWarn(res, unifiedMemberB) {
			t.Errorf("expected a WARN naming unified_commitment and the uncommitted member %q; logs: %v", unifiedMemberB, res.Logs)
		}
	})
}
