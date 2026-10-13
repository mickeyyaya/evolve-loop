package topngate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestTDDScopeGate_WithoutATriageReportASingleCommittedItemIsTheBindingSet(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "lane-scope.json"), []byte(`{"todo_ids":["only-task"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTDDReport(t, ws, "other-task", "go/acs/cycle1/predicates_test.go")

	reason, block := tddScopeGate{}.check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})

	if block {
		t.Fatalf("label drift never blocks: %q", reason)
	}
	if !strings.Contains(reason, "label drift") || !strings.Contains(reason, "{only-task}") || !strings.Contains(reason, "'other-task'") {
		t.Errorf("the drift must be judged against the lane's committed item: %q", reason)
	}

	ws2 := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws2, "lane-scope.json"), []byte(`{"todo_ids":["only-task"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTDDReport(t, ws2, "only-task", "go/acs/cycle1/predicates_test.go")
	if reason, block := (tddScopeGate{}).check(core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws2}); block || strings.Contains(reason, "label drift") {
		t.Errorf("a matching label has no drift: block=%v reason=%q", block, reason)
	}
}
