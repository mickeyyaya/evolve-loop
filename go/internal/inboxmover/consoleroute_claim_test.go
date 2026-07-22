package inboxmover

// consoleroute_claim_test.go — RED contract for ADR-0074 I1 enforcement at the
// physical handoff. Claim is the one operation that hands an inbox item to a
// lane (inbox/ → processing/cycle-N/); a console-routed item must be REFUSED
// here even if a triage LLM names it — prompts advise, the mover enforces.

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func writeClaimItem(t *testing.T, root, name, body string) string {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// Explicit route field: refused with the typed sentinel, file NOT moved.
func TestClaim_RefusesConsoleRoutedItem(t *testing.T) {
	root := t.TempDir()
	src := writeClaimItem(t, root, "x.json", `{"id":"task-x","route":"console-manual"}`)
	_, err := Claim(Options{ProjectRoot: root, Stderr: io.Discard}, "task-x", "7")
	if !errors.Is(err, ErrConsoleRouted) {
		t.Fatalf("want ErrConsoleRouted, got %v", err)
	}
	if _, statErr := os.Stat(src); statErr != nil {
		t.Fatal("refused item must remain in inbox/ (not moved to processing/)")
	}
}

// Derived form: the IsProtectedPath seam routes items whose declared fix
// surface is control-plane, mirroring inboxbatch.ConsoleRouted exactly.
func TestClaim_RefusesProtectedFixSurfaceItem(t *testing.T) {
	root := t.TempDir()
	writeClaimItem(t, root, "y.json", `{"id":"task-y","files":["go/internal/guards/role.go (fix)"]}`)
	opts := Options{ProjectRoot: root, Stderr: io.Discard,
		IsProtectedPath: func(p string) bool { return p == "go/internal/guards/role.go" }}
	_, err := Claim(opts, "task-y", "7")
	if !errors.Is(err, ErrConsoleRouted) {
		t.Fatalf("want ErrConsoleRouted for protected fix surface, got %v", err)
	}
}

// route:"lane" override claims normally even with a protected file declared.
func TestClaim_LaneOverrideClaims(t *testing.T) {
	root := t.TempDir()
	writeClaimItem(t, root, "z.json", `{"id":"task-z","route":"lane","files":["go/internal/guards/role.go"]}`)
	opts := Options{ProjectRoot: root, Stderr: io.Discard,
		IsProtectedPath: func(p string) bool { return true }}
	res, err := Claim(opts, "task-z", "7")
	if err != nil {
		t.Fatalf("route:lane must claim, got %v", err)
	}
	if _, statErr := os.Stat(res.DestPath); statErr != nil {
		t.Fatal("claimed item must exist in processing/")
	}
}

// A malformed item was unclaimable BEFORE the routing floor (findFileByTaskID
// requires parseable JSON to match .id → ErrNotFound) — pin that the floor
// changes nothing about that contract: still ErrNotFound, never
// ErrConsoleRouted, never a new failure mode.
func TestClaim_MalformedItemUnchangedContract(t *testing.T) {
	root := t.TempDir()
	writeClaimItem(t, root, "bad.json", `{"id":"task-bad", MALFORMED`)
	_, err := Claim(Options{ProjectRoot: root, Stderr: io.Discard}, "task-bad", "7")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("malformed item must keep its pre-floor ErrNotFound contract, got %v", err)
	}
}
