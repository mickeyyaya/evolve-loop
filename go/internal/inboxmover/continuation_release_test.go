package inboxmover

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

// releaseFixtureItem drops an inbox item carrying id into dir and returns its
// path.
func releaseFixtureItem(t *testing.T, dir, id string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("fixture dir %s: %v", dir, err)
	}
	path := filepath.Join(dir, "2026-08-18T00-00-00Z-"+id+".json")
	body, err := json.MarshalIndent(map[string]any{"id": id, "kind": "bug", "title": id}, "", "  ")
	if err != nil {
		t.Fatalf("fixture marshal: %v", err)
	}
	if err := os.WriteFile(path, append(body, '\n'), 0o644); err != nil {
		t.Fatalf("fixture write %s: %v", path, err)
	}
	return path
}

// releaseFixtureBind writes a registry binding, failing loudly if the fixture
// itself did not take.
func releaseFixtureBind(t *testing.T, root, id string, c continuation.Continuation) {
	t.Helper()
	if err := continuation.WriteRegistryEntry(root, id, c); err != nil {
		t.Fatalf("fixture bind %s: %v", id, err)
	}
}

// hasRegistryBinding reports whether id holds a binding.
func hasRegistryBinding(t *testing.T, root, id string) bool {
	t.Helper()
	_, ok, err := continuation.ReadRegistryEntry(root, id)
	if err != nil {
		t.Fatalf("read registry %s: %v", id, err)
	}
	return ok
}

// TestReleaseContinuationBinding covers the ONE release transaction every
// retirement path now shares (the fix for audit cycle-1507's H2: the read-side
// delete used to skip preservation and send the salvage pointer to stderr only).
func TestReleaseContinuationBinding(t *testing.T) {
	t.Run("preserves the pointer into the item file before deleting", func(t *testing.T) {
		root := t.TempDir()
		inbox := filepath.Join(root, ".evolve", "inbox")
		itemPath := releaseFixtureItem(t, inbox, "scope-under-release")
		releaseFixtureItem(t, inbox, "unrelated-live-scope")
		c := continuation.Continuation{Worktree: "/tmp/wt", Branch: "cycle-1484", SnapshotSHA: "snap1484", BaseSHA: "base1484", Cycle: 1484}
		releaseFixtureBind(t, root, "scope-under-release", c)
		releaseFixtureBind(t, root, "unrelated-live-scope", continuation.Continuation{SnapshotSHA: "snap1490", Cycle: 1490})

		var errBuf bytes.Buffer
		got, released, err := ReleaseContinuationBinding(Options{ProjectRoot: root, Stderr: &errBuf}, "scope-under-release", "unit-test")
		if err != nil {
			t.Fatalf("ReleaseContinuationBinding: %v (stderr %s)", err, errBuf.String())
		}
		if !released {
			t.Fatalf("release reported released=false for a binding this cycle owns")
		}
		if got.SnapshotSHA != c.SnapshotSHA {
			t.Errorf("released value lost the snapshot ref: %+v", got)
		}
		if hasRegistryBinding(t, root, "scope-under-release") {
			t.Errorf("the binding survived its release")
		}
		if !hasRegistryBinding(t, root, "unrelated-live-scope") {
			t.Errorf("releasing one scope also dropped the unrelated live binding")
		}

		raw, rerr := os.ReadFile(itemPath)
		if rerr != nil {
			t.Fatalf("read item: %v", rerr)
		}
		for _, want := range []string{c.SnapshotSHA, c.BaseSHA, c.Branch, "unit-test"} {
			if !strings.Contains(string(raw), want) {
				t.Errorf("released_continuations[] in %s does not preserve %q — pointer loss on release\n%s", itemPath, want, raw)
			}
		}
	})

	t.Run("an unbound scope is a clean miss, not a failure", func(t *testing.T) {
		root := t.TempDir()
		var errBuf bytes.Buffer
		_, released, err := ReleaseContinuationBinding(Options{ProjectRoot: root, Stderr: &errBuf}, "never-bound", "unit-test")
		if err != nil || released {
			t.Fatalf("want (released=false, err=nil) for an unbound scope, got released=%v err=%v", released, err)
		}
	})

	t.Run("an empty scope id is refused rather than interpreted", func(t *testing.T) {
		var errBuf bytes.Buffer
		if _, _, err := ReleaseContinuationBinding(Options{ProjectRoot: t.TempDir(), Stderr: &errBuf}, "  ", "unit-test"); err == nil {
			t.Fatalf("want an error for an empty scope id, got nil")
		}
	})
}

// TestFindScopeItemFile pins the liveness search order the preserved pointer
// depends on: the pending root, then a processing claim, then the retirement
// subtrees — so the annotation always lands on the copy an operator opens.
func TestFindScopeItemFile(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	opts := Options{ProjectRoot: root}

	if got := FindScopeItemFile(opts, "absent-scope"); got != "" {
		t.Errorf("want no path for a scope with no item anywhere, got %q", got)
	}

	quarantined := releaseFixtureItem(t, filepath.Join(inbox, "quarantine"), "retired-scope")
	if got := FindScopeItemFile(opts, "retired-scope"); got != quarantined {
		t.Errorf("retired copy: got %q, want %q", got, quarantined)
	}

	pending := releaseFixtureItem(t, inbox, "live-scope")
	if got := FindScopeItemFile(opts, "live-scope"); got != pending {
		t.Errorf("pending copy: got %q, want %q", got, pending)
	}

	claimed := releaseFixtureItem(t, filepath.Join(inbox, "processing", "cycle-1515"), "claimed-scope")
	if got := FindScopeItemFile(opts, "claimed-scope"); got != claimed {
		t.Errorf("processing claim: got %q, want %q", got, claimed)
	}
}

// TestResolveContinuationForScopeRecency is the regression test for audit
// cycle-1507's H1. The read-side guard deletes a ROOT-OWNED binding on
// agent-writable evidence, so the evidence has to be recent: a retired copy
// OLDER than the binding means the item was re-filed and rebound after that
// retirement, and releasing on it destroys live preserved work. Same-or-newer
// retirement (and the unstamped ordinary quarantine copy) is still evidence —
// a guard that refuses to act on the common case is no guard at all.
func TestResolveContinuationForScopeRecency(t *testing.T) {
	tests := []struct {
		name         string
		retiredDir   []string // path segments under the inbox for the retired copy
		bindingCycle int
		wantResolved bool
	}{
		{
			name:         "retirement older than the binding is stale evidence: adopt, do not release",
			retiredDir:   []string{"processed", "cycle-900"},
			bindingCycle: 1484,
			wantResolved: true,
		},
		{
			name:         "retirement newer than the binding is real evidence: refuse and release",
			retiredDir:   []string{"processed", "cycle-1490"},
			bindingCycle: 1484,
			wantResolved: false,
		},
		{
			name:         "unstamped quarantine copy is evidence: refuse and release",
			retiredDir:   []string{"quarantine"},
			bindingCycle: 1484,
			wantResolved: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			inbox := filepath.Join(root, ".evolve", "inbox")
			if err := os.MkdirAll(inbox, 0o755); err != nil {
				t.Fatalf("fixture inbox: %v", err)
			}
			itemPath := releaseFixtureItem(t, filepath.Join(append([]string{inbox}, tc.retiredDir...)...), "ghost-scope")
			c := continuation.Continuation{Branch: "cycle-1484", SnapshotSHA: "snapGhost", BaseSHA: "baseGhost", Cycle: tc.bindingCycle}
			releaseFixtureBind(t, root, "ghost-scope", c)

			var errBuf bytes.Buffer
			got := ResolveContinuationForScope(Options{ProjectRoot: root, Stderr: &errBuf}, 1515, []string{"ghost-scope"})

			if tc.wantResolved {
				if got == nil {
					t.Fatalf("binding was refused on STALE retirement evidence — live preserved work released. stderr: %s", errBuf.String())
				}
				if got.SnapshotSHA != c.SnapshotSHA {
					t.Errorf("resolved the wrong binding: %+v", got)
				}
				if !hasRegistryBinding(t, root, "ghost-scope") {
					t.Errorf("the guard released a binding it had decided to adopt")
				}
				return
			}
			if got != nil {
				t.Fatalf("ghost binding was adopted (snapshot %s) — the parked scope re-dispatches forever", got.SnapshotSHA)
			}
			if hasRegistryBinding(t, root, "ghost-scope") {
				t.Errorf("the ghost binding survived the refusal — it re-arms on the next wave")
			}
			raw, rerr := os.ReadFile(itemPath)
			if rerr != nil {
				t.Fatalf("read retired item: %v", rerr)
			}
			if !strings.Contains(string(raw), c.SnapshotSHA) {
				t.Errorf("the read-side release did not preserve the salvage pointer into %s (H2) — it is lost, not released\n%s", itemPath, raw)
			}
		})
	}
}
