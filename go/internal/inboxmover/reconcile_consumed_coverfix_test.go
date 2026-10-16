package inboxmover

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

func reconcileFixture(t *testing.T) (root, consumed string) {
	t.Helper()
	root = t.TempDir()
	consumed = filepath.Join(root, ".evolve", "inbox", "consumed")
	if err := os.MkdirAll(consumed, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, consumed
}

func writeConsumed(t *testing.T, consumed, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(consumed, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func bindScope(t *testing.T, root, id string, cycle int) {
	t.Helper()
	if err := continuation.WriteRegistryEntry(root, id, continuation.Continuation{Branch: "b", SnapshotSHA: "s", Cycle: cycle}); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileConsumedBindings_NoConsumedDirReleasesNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if got := ReconcileConsumedBindings(Options{ProjectRoot: root, Stderr: os.Stderr}); len(got) != 0 {
		t.Fatalf("released = %v, want none when consumed/ does not exist", got)
	}
}

func TestReconcileConsumedBindings_SkipsEntriesThatAreNotItems(t *testing.T) {
	t.Parallel()
	root, consumed := reconcileFixture(t)
	if err := os.MkdirAll(filepath.Join(consumed, "sub.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeConsumed(t, consumed, "notes.txt", `{"id":"txt"}`)
	writeConsumed(t, consumed, "noid.json", `{"title":"no id"}`)
	writeConsumed(t, consumed, "unbound.json", `{"id":"unbound"}`)
	bindScope(t, root, "txt", 3)

	got := ReconcileConsumedBindings(Options{ProjectRoot: root, Stderr: os.Stderr})
	if len(got) != 0 {
		t.Fatalf("released = %v, want none: dirs, non-json files, id-less and unbound items release nothing", got)
	}
	if _, ok, _ := continuation.ReadRegistryEntry(root, "txt"); !ok {
		t.Error("binding of a non-json consumed file must stay in place")
	}
}

func TestReconcileConsumedBindings_UnreadableRegistryLeavesBindingsAlone(t *testing.T) {
	t.Parallel()
	root, consumed := reconcileFixture(t)
	writeConsumed(t, consumed, "a.json", `{"id":"a"}`)
	if err := os.WriteFile(continuation.RegistryPath(root), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ReconcileConsumedBindings(Options{ProjectRoot: root, Stderr: os.Stderr}); len(got) != 0 {
		t.Fatalf("released = %v, want none when the registry cannot be parsed", got)
	}
	raw, _ := os.ReadFile(continuation.RegistryPath(root))
	if string(raw) != "{not json" {
		t.Errorf("an unreadable registry must be left untouched, got %q", raw)
	}
}

func TestReconcileConsumedBindings_FailedReleaseIsNotReported(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	root, consumed := reconcileFixture(t)
	writeConsumed(t, consumed, "a.json", `{"id":"a"}`)
	bindScope(t, root, "a", 2)
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.Chmod(evolveDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(evolveDir, 0o755) })

	if got := ReconcileConsumedBindings(Options{ProjectRoot: root, Stderr: os.Stderr}); len(got) != 0 {
		t.Fatalf("released = %v, want none when the registry delete fails", got)
	}
	if _, ok, _ := continuation.ReadRegistryEntry(root, "a"); !ok {
		t.Error("the binding must survive a failed release")
	}
}
