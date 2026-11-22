package inboxtest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func TestFixtures_BuildAndReadBackAnInboxTree(t *testing.T) {
	root := CurationRoot(t, map[string]string{"a.json": `{"id":"a","weight":0.3}`, "consumed/b.json": `{"id":"b"}`})
	if got := os.Getenv("EVOLVE_PROJECT_ROOT"); got != root {
		t.Fatalf("EVOLVE_PROJECT_ROOT = %q, want %q", got, root)
	}
	if got := ItemFields(t, filepath.Join(root, ".evolve", "inbox", "a.json"))["weight"]; got != 0.3 {
		t.Errorf("weight = %v, want 0.3", got)
	}
	tree := TreeBytes(t, root)
	if len(tree) != 2 || tree[filepath.Join(root, ".evolve", "inbox", "consumed", "b.json")] != `{"id":"b"}` {
		t.Errorf("tree = %v", tree)
	}
}

func TestFixtures_EmptyRootHasAnInboxAndPolicyAndRoutedAreWritten(t *testing.T) {
	root := EmptyRoot(t)
	if fi, err := os.Stat(filepath.Join(root, ".evolve", "inbox")); err != nil || !fi.IsDir() {
		t.Fatalf("inbox dir: %v %v", fi, err)
	}
	WritePolicy(t, root, `{"k":1}`)
	if body, _ := os.ReadFile(filepath.Join(root, ".evolve", "policy.json")); string(body) != `{"k":1}` {
		t.Errorf("policy = %q", body)
	}
	p := filepath.Join(root, "routed.json")
	WriteFile(t, p, `{"route":"console-manual","routed_reason":"why","routed_cycle":7}`)
	if got := ReadRouted(t, p); got != (Routed{Route: "console-manual", RoutedReason: "why", RoutedCycle: 7}) {
		t.Errorf("routed = %+v", got)
	}
	SeedItems(t, root, map[string]string{"c.json": `{}`})
	if _, err := os.Stat(filepath.Join(root, ".evolve", "inbox", "c.json")); err != nil {
		t.Errorf("seeded item: %v", err)
	}
}

func TestFixtures_WriteLiveRunLeaseRecordsTheRunningProcess(t *testing.T) {
	root := EmptyRoot(t)
	WriteLiveRunLease(t, root)
	lease, ok, err := runlease.Read(filepath.Join(root, ".evolve", "runs", "cycle-1790"))
	if err != nil || !ok {
		t.Fatalf("lease: %v %v", ok, err)
	}
	if lease.RunID != "01LIVE" || lease.OwnerPID != os.Getpid() {
		t.Errorf("lease = %+v", lease)
	}
}

func TestFixtures_WriteItemBindBoundMkdirAllAndRealDirDoWhatTheirNamesSay(t *testing.T) {
	root := EmptyRoot(t)
	inbox := filepath.Join(root, ".evolve", "inbox")
	path := WriteItem(t, inbox, "alpha")
	fields := ItemFields(t, path)
	if fields["id"] != "alpha" || fields["kind"] != "bug" || fields["priority"] != "high" || fields["title"] != "fixture item alpha" {
		t.Errorf("item fields = %v", fields)
	}
	if filepath.Base(path) != "2026-08-16T15-10-00Z-alpha.json" {
		t.Errorf("item path = %s", path)
	}
	if Bound(t, root, "alpha") {
		t.Fatal("a scope with no binding reads as bound")
	}
	Bind(t, root, "alpha", continuation.Continuation{Cycle: 3})
	if !Bound(t, root, "alpha") {
		t.Error("Bind did not bind the scope")
	}
	nested := filepath.Join(root, "a", "b")
	MkdirAll(t, nested)
	if fi, err := os.Stat(nested); err != nil || !fi.IsDir() {
		t.Errorf("MkdirAll: %v %v", fi, err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(nested, link); err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(nested)
	if got := RealDir(t, link); got != want {
		t.Errorf("RealDir = %s, want %s", got, want)
	}
}
