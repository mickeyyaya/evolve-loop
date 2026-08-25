package inboxmover

// In-package contract pins for ReconcileConsumedBindings (apicover-enforce:
// exported surface must be named in-package; the breaker-boot wiring pins
// live in cmd/evolve).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

func TestReconcileConsumedBindings(t *testing.T) {
	root := t.TempDir()
	consumed := filepath.Join(root, ".evolve", "inbox", "consumed")
	if err := os.MkdirAll(consumed, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(consumed, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	bind := func(id string, cycle int) {
		t.Helper()
		if err := continuation.WriteRegistryEntry(root, id, continuation.Continuation{
			Branch: "b", SnapshotSHA: "s", Cycle: cycle,
		}); err != nil {
			t.Fatal(err)
		}
	}
	// stray: consumed, no live copy, no newer-binding evidence → released
	write("stray.json", `{"id":"stray"}`)
	bind("stray", 4)
	// recency: consumed at cycle 5, rebound at cycle 9 → kept
	write("refiled.json", `{"id":"refiled","consumed":{"cycle":5}}`)
	bind("refiled", 9)
	// live copy in the pending root → kept
	write("relive.json", `{"id":"relive"}`)
	if err := os.WriteFile(filepath.Join(root, ".evolve", "inbox", "relive.json"), []byte(`{"id":"relive"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	bind("relive", 6)

	released := ReconcileConsumedBindings(Options{ProjectRoot: root, Stderr: os.Stderr})
	if len(released) != 1 || released[0] != "stray" {
		t.Fatalf("released = %v, want exactly [stray]", released)
	}
	for id, want := range map[string]bool{"stray": false, "refiled": true, "relive": true} {
		if _, ok, _ := continuation.ReadRegistryEntry(root, id); ok != want {
			t.Errorf("binding %q present=%v, want %v", id, ok, want)
		}
	}
	// released pointer preserved onto the consumed item
	raw, _ := os.ReadFile(filepath.Join(consumed, "stray.json"))
	if !strings.Contains(string(raw), "released_continuations") {
		t.Errorf("stray.json missing the preserved salvage pointer: %s", raw)
	}
}
