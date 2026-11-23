package defectledger

import (
	"os"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

func TestArm_DeletedManifestOfAdoptingCycleStillBlocks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := paths.RunWorkspace(root, 2110)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	bound := continuation.Continuation{Cycle: 2093, Branch: "cycle-2093", SnapshotSHA: "deadbeef", BaseSHA: "cafebabe"}
	if err := continuation.WriteRegistryEntry(root, "item-a", bound); err != nil {
		t.Fatal(err)
	}
	l := New(scopeOf("item-a"), resolveNever)
	req := Request{Cycle: 2110, Workspace: ws, ProjectRoot: root}

	v := l.Reconcile(req)

	if !v.Blocked || len(v.Diagnostics) == 0 || v.Diagnostics[0].Code != string(CodeManifestMissing) || v.Diagnostics[0].Subject != "item-a" {
		t.Fatalf("verdict = %+v; want a block coded %s with subject item-a", v, CodeManifestMissing)
	}

	if err := continuation.WriteManifest(ws, continuation.Continuation{Cycle: 1, SnapshotSHA: "deadbeef"}); err != nil {
		t.Fatal(err)
	}
	v = l.Reconcile(req)
	if !v.Blocked || len(v.Diagnostics) == 0 || v.Diagnostics[0].Code == string(CodeManifestMissing) || !strings.Contains(v.Diagnostics[0].Message, "names cycle-1") {
		t.Fatalf("verdict = %+v; want a rewritten manifest to block as a lineage disagreement", v)
	}
}
