package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

func writeBinding(t *testing.T, root, id string, cycle int) {
	t.Helper()
	if err := continuation.WriteRegistryEntry(root, id, continuation.Continuation{
		Worktree: "/tmp/wt", Branch: "cycle-x", SnapshotSHA: "abc", BaseSHA: "def",
		FindingsPath: "/tmp/f.json", Cycle: cycle,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRunInbox_Consume_ReleasesContinuationBinding(t *testing.T) {
	root := t.TempDir()
	withProjectRoot(t, root)
	evolveDir := filepath.Join(root, ".evolve")
	p := writePendingItem(t, evolveDir, "the-item.json", `{"id":"the-item","title":"x","released_continuations":[{"branch":"older","reason":"earlier-retirement"}]}`)
	writeBinding(t, root, "the-item", 42)

	var out, errb bytes.Buffer
	if code := runInbox([]string{"consume", p}, nil, &out, &errb); code != 0 {
		t.Fatalf("consume rc=%d stderr=%s", code, errb.String())
	}
	if _, ok, _ := continuation.ReadRegistryEntry(root, "the-item"); ok {
		t.Fatal("continuation binding survived the consume — the next wave will mint a lane off it (cycle-1558)")
	}
	raw, err := os.ReadFile(filepath.Join(evolveDir, "inbox", "consumed", "the-item.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if json.Unmarshal(raw, &doc) != nil {
		t.Fatal("consumed item unparseable")
	}
	rel, _ := doc["released_continuations"].([]any)
	if len(rel) != 2 {
		t.Fatalf("released_continuations = %v, want the PRIOR entry preserved plus the new salvage pointer", doc["released_continuations"])
	}
}

func TestReconcileConsumedBindings_ReleasesStrayBinding(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	consumed := filepath.Join(evolveDir, "inbox", "consumed")
	if err := os.MkdirAll(consumed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(consumed, "old-item.json"),
		[]byte(`{"id":"old-item","title":"consumed long ago"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeBinding(t, root, "old-item", 7)

	var errb bytes.Buffer
	reconcileConsumedBindings(root, evolveDir, &errb)
	if _, ok, _ := continuation.ReadRegistryEntry(root, "old-item"); ok {
		t.Fatal("stray binding for an already-consumed item survived the reconcile sweep")
	}
}

func TestReconcileConsumedBindings_LiveBindingUntouched(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(filepath.Join(evolveDir, "inbox", "consumed"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeBinding(t, root, "live-item", 9)
	var errb bytes.Buffer
	reconcileConsumedBindings(root, evolveDir, &errb)
	if _, ok, _ := continuation.ReadRegistryEntry(root, "live-item"); !ok {
		t.Fatal("live binding (no consumed item) was released by the sweep")
	}
}

func TestBlockerBreakerBootPath_RunsBindingReconcile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	consumed := filepath.Join(evolveDir, "inbox", "consumed")
	if err := os.MkdirAll(consumed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(consumed, "boot-item.json"),
		[]byte(`{"id":"boot-item"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeBinding(t, root, "boot-item", 3)
	var errb bytes.Buffer
	_, _ = blockerBreakerHalt(evolveDir, root, 1, &errb, testRootSignals(t, &errb))
	if _, ok, _ := continuation.ReadRegistryEntry(root, "boot-item"); ok {
		t.Fatalf("breaker boot path did not release the stray binding (stderr=%s)", errb.String())
	}
}

func TestReconcileConsumedBindings_NewerBindingSurvivesStaleConsumedCopy(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	consumed := filepath.Join(evolveDir, "inbox", "consumed")
	if err := os.MkdirAll(consumed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(consumed, "refiled-item.json"),
		[]byte(`{"id":"refiled-item","consumed":{"cycle":5}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeBinding(t, root, "refiled-item", 9)
	var errb bytes.Buffer
	reconcileConsumedBindings(root, evolveDir, &errb)
	if _, ok, _ := continuation.ReadRegistryEntry(root, "refiled-item"); !ok {
		t.Fatal("a binding NEWER than the consumed copy was released — the cycle-1507 false-positive class reopened")
	}
	if !strings.Contains(errb.String(), "recency") && !strings.Contains(errb.String(), "NEWER") {
		t.Errorf("the skip must be loud; stderr=%q", errb.String())
	}
}

func TestReconcileConsumedBindings_RefiledLiveCopyKeepsItsBinding(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	consumed := filepath.Join(evolveDir, "inbox", "consumed")
	if err := os.MkdirAll(consumed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(consumed, "refiled-live.json"),
		[]byte(`{"id":"refiled-live"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "inbox", "refiled-live.json"),
		[]byte(`{"id":"refiled-live","title":"fresh re-filing"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeBinding(t, root, "refiled-live", 11)
	var errb bytes.Buffer
	reconcileConsumedBindings(root, evolveDir, &errb)
	if _, ok, _ := continuation.ReadRegistryEntry(root, "refiled-live"); !ok {
		t.Fatal("the re-filed LIVE copy's binding was released on the old consumed copy's evidence")
	}
}

func TestReleaseConsumedItemBinding_ReleasesAnIDLessItemByItsFileName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "consumed", "no-id.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"title":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeBinding(t, root, "no-id", 42)

	var errb bytes.Buffer
	if !releaseConsumedItemBinding(root, path, &errb) {
		t.Fatalf("the binding of an id-less item must be released under the loader's id; stderr=%s", errb.String())
	}
	if _, ok, _ := continuation.ReadRegistryEntry(root, "no-id"); ok {
		t.Fatal("the binding survived")
	}
}

func TestReleaseConsumedItemBinding_ReleasesNothingItCannotName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var errb bytes.Buffer
	if releaseConsumedItemBinding(root, filepath.Join(root, "missing.json"), &errb) {
		t.Error("an unreadable item has no id, so nothing is released")
	}
	path := filepath.Join(root, "item.json")
	if err := os.WriteFile(path, []byte(`{"id":"item"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(continuation.RegistryPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(continuation.RegistryPath(root), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if releaseConsumedItemBinding(root, path, &errb) || !strings.Contains(errb.String(), `binding release "item"`) {
		t.Errorf("an unreadable registry releases nothing and WARNs; stderr=%q", errb.String())
	}
}
