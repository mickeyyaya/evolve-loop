package core

// console_lease_test.go — ADR-0080 S4, defense in depth: an EXPLICIT,
// time-bounded operator lease can waive tree-diff attribution for named
// paths. Review BLOCK hardening pinned here: the lease is HUB-resident
// (git common dir — outside every worktree, unreachable by the .evolve/
// legitimacy blanket), adopted once at cycle start, exact-path, expiring,
// and LOUD per waiver.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

// leaseRoot builds a primary-checkout fixture; the lease then lives at
// <root>/.git/evolve-console-lease.json (the hub).
func leaseRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeLease(t *testing.T, root string, lease consoleLease) {
	t.Helper()
	b, err := json.Marshal(lease)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", plane.ConsoleLeaseFileName), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadConsoleLease_ActiveLeaseYieldsExactPaths(t *testing.T) {
	root := leaseRoot(t)
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	writeLease(t, root, consoleLease{
		Paths:     []string{"go/internal/core/x.go", "docs/y.md"},
		ExpiresAt: now.Add(30 * time.Minute).Format(time.RFC3339),
		Reason:    "console hotfix in flight",
	})
	leased := readConsoleLease(root, now)
	if !leased["go/internal/core/x.go"] || !leased["docs/y.md"] {
		t.Fatalf("active lease paths missing: %v", leased)
	}
	if leased["go/internal/core/other.go"] {
		t.Fatal("lease must be exact-path, never a prefix waiver")
	}
}

// TestReadConsoleLease_WorktreeResidentLeaseIsIgNORED is the review-BLOCK
// pin: a lease file written INSIDE the checkout (the old .evolve/ location —
// exactly what a lane phase could author) waives nothing. Only the hub copy
// counts.
func TestReadConsoleLease_WorktreeResidentLeaseIsIgnored(t *testing.T) {
	root := leaseRoot(t)
	now := time.Now()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(consoleLease{
		Paths:     []string{"go/internal/core/evil.go"},
		ExpiresAt: now.Add(time.Hour).Format(time.RFC3339),
	})
	if err := os.WriteFile(filepath.Join(root, ".evolve", "console-lease.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if leased := readConsoleLease(root, now); len(leased) != 0 {
		t.Fatalf("a checkout-resident lease must never waive (agent-writable surface): %v", leased)
	}
}

func TestReadConsoleLease_ExpiredIsEmpty(t *testing.T) {
	root := leaseRoot(t)
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	writeLease(t, root, consoleLease{
		Paths:     []string{"go/internal/core/x.go"},
		ExpiresAt: now.Add(-time.Minute).Format(time.RFC3339),
	})
	if leased := readConsoleLease(root, now); len(leased) != 0 {
		t.Fatalf("expired lease must waive nothing, got %v", leased)
	}
}

func TestReadConsoleLease_AbsentMalformedOrNoRepoKeepsGuardArmed(t *testing.T) {
	if leased := readConsoleLease(t.TempDir(), time.Now()); len(leased) != 0 {
		t.Fatalf("non-repo root must be empty, got %v", leased)
	}
	root := leaseRoot(t)
	if leased := readConsoleLease(root, time.Now()); len(leased) != 0 {
		t.Fatalf("absent lease must be empty, got %v", leased)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", plane.ConsoleLeaseFileName), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if leased := readConsoleLease(root, time.Now()); len(leased) != 0 {
		t.Fatalf("malformed lease must keep the guard armed, got %v", leased)
	}
}

func TestReadConsoleLease_MissingExpiryNeverWaives(t *testing.T) {
	root := leaseRoot(t)
	writeLease(t, root, consoleLease{Paths: []string{"go/x.go"}})
	if leased := readConsoleLease(root, time.Now()); len(leased) != 0 {
		t.Fatalf("a lease without an expiry is not a lease — unbounded waivers are exactly the blind spot the ADR rejected: %v", leased)
	}
}

// TestFilterRealLeaks_LeaseWaiverIsExactAndLoud is the WIRING proof (review
// MEDIUM): the classifier chain itself — a leaked SOURCE path is waived only
// by an exact lease entry, the waiver WARNs with the ADR reference, and an
// unleased source leak stays real.
func TestFilterRealLeaks_LeaseWaiverIsExactAndLoud(t *testing.T) {
	var warn strings.Builder
	leaked := []string{"go/internal/core/foo.go", "go/internal/core/bar.go"}
	leased := map[string]bool{"go/internal/core/foo.go": true}
	real, waived := filterRealLeaks(PhaseBuild, leaked, nil, leased, &warn)
	if len(real) != 1 || real[0] != "go/internal/core/bar.go" || waived != 1 {
		t.Fatalf("realLeaks = %v waived = %d, want only the unleased bar.go with one waiver", real, waived)
	}
	if !strings.Contains(warn.String(), "ADR-0080") || !strings.Contains(warn.String(), "foo.go") {
		t.Errorf("waiver must WARN loudly with the ADR reference: %q", warn.String())
	}
	if v, w := filterRealLeaks(PhaseBuild, leaked, nil, nil, &warn); len(v) != 2 || w != 0 {
		t.Fatalf("no lease ⇒ both source leaks real and zero waivers, got %v (%d)", v, w)
	}
}
