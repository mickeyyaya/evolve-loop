package core_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/site"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func TestInboxResolver_AgreesWithTheContractResolverForEachLifecycleState(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	items := map[string]string{
		"pending-id":    ".evolve/inbox/2026-10-09-pending-id.json",
		"processing-id": ".evolve/inbox/processing/cycle-7/2026-10-09-processing-id.json",
	}
	for id, rel := range items {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(`{"id":"`+id+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	scope := func(projectRoot, taskID string) string {
		st := inboxmover.ResolveDispatchState(inboxmover.Options{ProjectRoot: projectRoot}, taskID)
		if st.State != inboxmover.StatePending && st.State != inboxmover.StateProcessing {
			return ""
		}
		return st.Path
	}
	orch := core.NewOrchestrator(nil, nil, nil, core.WithScopePathResolver(scope))
	contract := core.ContractPathResolverForTest(orch, root)
	audit := site.InboxResolver(root)
	for _, id := range []string{"pending-id", "processing-id", "missing-id"} {
		if got, want := audit(id), contract(id); got != want {
			t.Errorf("id %q: the audit resolver returns %q, the contract resolver returns %q", id, got, want)
		}
	}
}
