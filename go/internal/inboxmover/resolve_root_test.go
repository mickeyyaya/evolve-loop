package inboxmover

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveOpts_AnInboxUnderAPlaneNamesItsProjectRoot(t *testing.T) {
	root := t.TempDir()
	o := Options{InboxDir: filepath.Join(root, ".evolve", "inbox")}
	o.resolveOpts()
	if o.ProjectRoot != root {
		t.Fatalf("ProjectRoot=%q, want the plane root %q", o.ProjectRoot, root)
	}
	if want := filepath.Join(root, ".evolve", "ledger.jsonl"); o.LedgerPath != want {
		t.Fatalf("LedgerPath=%q, want %q", o.LedgerPath, want)
	}
}

func TestPlaneRootOf_OnlyThePlaneInboxLayoutNamesARoot(t *testing.T) {
	root := t.TempDir()
	cases := map[string]string{
		filepath.Join(root, ".evolve", "inbox"):       root,
		filepath.Join(root, ".evolve", "inbox") + "/": root,
		filepath.Join(root, ".evolve", "queue"):       "",
		filepath.Join(root, "other", "inbox"):         "",
		"":                                            "",
	}
	for in, want := range cases {
		if got := planeRootOf(in); got != want {
			t.Errorf("planeRootOf(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestResolveOpts_AnInboxOutsideAPlaneKeepsTheGivenRoot(t *testing.T) {
	dir := t.TempDir()
	o := Options{InboxDir: filepath.Join(dir, "queue")}
	o.resolveOpts()
	if o.ProjectRoot != "" {
		t.Fatalf("ProjectRoot=%q, want empty for an inbox that is not <root>/.evolve/inbox", o.ProjectRoot)
	}
	given := Options{ProjectRoot: dir, InboxDir: filepath.Join(t.TempDir(), ".evolve", "inbox")}
	given.resolveOpts()
	if given.ProjectRoot != dir {
		t.Fatalf("ProjectRoot=%q, want the caller's %q", given.ProjectRoot, dir)
	}
}

func TestClaimPending_AnInboxOnlyCallerWritesTheClaimRowToThePlaneLedger(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	item := `{"id":"item-a","title":"t","summary":"s","fix":"f","kind":"bug","priority_class":"correctness","files":["go/x.go"],"acceptance":["a"],"weight":0.5}`
	if err := os.WriteFile(filepath.Join(inbox, "2026-11-02T00-00-00Z-item-a.json"), []byte(item), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ClaimPending(Options{InboxDir: inbox, Stderr: io.Discard}, 7, []string{"item-a"}); err != nil {
		t.Fatalf("ClaimPending: %v", err)
	}
	rows, err := os.ReadFile(filepath.Join(root, ".evolve", "ledger.jsonl"))
	if err != nil || len(rows) == 0 {
		t.Fatalf("the claim row must land in the plane ledger: err=%v bytes=%d", err, len(rows))
	}
}
