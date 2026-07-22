package dispositionrouter_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
)

// TestDecide_FloorsForceConsole pins both deterministic floors and their
// boundary: guard-abort at any recurrence, and recurrence >= 3 for any class.
func TestDecide_FloorsForceConsole(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		preClass   string
		recurrence int
		llmRoute   string
		wantRoute  string
		wantForced bool
	}{
		{"guard-abort floors", dispositionrouter.GuardAbortClass, 1, dispositionrouter.RouteQueue, dispositionrouter.RouteConsole, true},
		{"recurrence 3 floors", "verdict-fail", 3, dispositionrouter.RouteQueue, dispositionrouter.RouteConsole, true},
		{"recurrence 2 does not", "verdict-fail", 2, dispositionrouter.RouteQueue, dispositionrouter.RouteQueue, false},
		{"advisory may raise", "verdict-fail", 1, dispositionrouter.RouteConsole, dispositionrouter.RouteConsole, false},
		{"advisory may not lower", "verdict-fail", 4, dispositionrouter.RouteQueue, dispositionrouter.RouteConsole, true},
		{"empty advisory is a no-op", "verdict-fail", 1, "", dispositionrouter.RouteQueue, false},
		{"unknown advisory is a no-op", "verdict-fail", 1, "nonsense", dispositionrouter.RouteQueue, false},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := dispositionrouter.Decide(tc.preClass, tc.recurrence, tc.llmRoute)
			if got.Route != tc.wantRoute || got.Forced != tc.wantForced {
				t.Fatalf("Decide(%q,%d,%q) = %+v, want {Route:%s Forced:%v}",
					tc.preClass, tc.recurrence, tc.llmRoute, got, tc.wantRoute, tc.wantForced)
			}
			if got.Forced && got.Reason == "" {
				t.Errorf("forced decision carries no Reason: %+v", got)
			}
		})
	}
}

// TestStageIntent_AppendsJSONLAndCreatesDir pins the staging contract: the dir
// is created on first use, each call appends exactly one parseable line, and
// the returned path is PendingActionsPath.
func TestStageIntent_AppendsJSONLAndCreatesDir(t *testing.T) {
	t.Parallel()
	escDir := filepath.Join(t.TempDir(), "escalations") // deliberately absent
	in := dispositionrouter.Intent{
		Cycle: 7, Pattern: "pattern:x", ItemID: "id-x",
		Action: dispositionrouter.ActionEscalate, Route: dispositionrouter.RouteQueue,
		Recurrence: 4, Weight: 0.8, Reason: "why",
	}
	path, err := dispositionrouter.StageIntent(escDir, in)
	if err != nil {
		t.Fatalf("StageIntent: %v", err)
	}
	if want := dispositionrouter.PendingActionsPath(escDir); path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	second := in
	second.Action = dispositionrouter.ActionAutofile
	if _, err := dispositionrouter.StageIntent(escDir, second); err != nil {
		t.Fatalf("StageIntent (second): %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read staged: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 {
		t.Fatalf("staged %d lines, want 2 (append, not overwrite)", len(lines))
	}
	var first dispositionrouter.Intent
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("line 0 is not JSON: %v", err)
	}
	if first != in {
		t.Errorf("round-trip = %+v, want %+v", first, in)
	}
}

// TestLoadIntents_MissingFileIsNotAnError pins the empty-staging case and the
// happy path in one place: nothing staged must no-op, and staged records must
// come back in append order.
func TestLoadIntents_MissingFileIsNotAnError(t *testing.T) {
	t.Parallel()
	escDir := filepath.Join(t.TempDir(), "escalations")
	got, err := dispositionrouter.LoadIntents(dispositionrouter.PendingActionsPath(escDir))
	if err != nil || got != nil {
		t.Fatalf("LoadIntents(missing) = (%v, %v), want (nil, nil)", got, err)
	}
	for _, id := range []string{"a", "b"} {
		if _, err := dispositionrouter.StageIntent(escDir, dispositionrouter.Intent{ItemID: id, Action: dispositionrouter.ActionEscalate}); err != nil {
			t.Fatalf("StageIntent(%s): %v", id, err)
		}
	}
	got, err = dispositionrouter.LoadIntents(dispositionrouter.PendingActionsPath(escDir))
	if err != nil {
		t.Fatalf("LoadIntents: %v", err)
	}
	if len(got) != 2 || got[0].ItemID != "a" || got[1].ItemID != "b" {
		t.Fatalf("LoadIntents = %+v, want [a b] in append order", got)
	}
}

// TestLoadIntents_MalformedLineErrors pins that garbage is surfaced, never
// silently treated as "nothing staged".
func TestLoadIntents_MalformedLineErrors(t *testing.T) {
	t.Parallel()
	escDir := t.TempDir()
	path := dispositionrouter.PendingActionsPath(escDir)
	if err := os.WriteFile(path, []byte("{not json}\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := dispositionrouter.LoadIntents(path); err == nil {
		t.Fatal("LoadIntents(malformed) returned nil error; a corrupt staging file must be surfaced")
	}
}

// TestStageIntent_UnwritableDirErrors pins that a staging failure is surfaced,
// never swallowed — a silently dropped intent is a lost escalation.
func TestStageIntent_UnwritableDirErrors(t *testing.T) {
	t.Parallel()
	blocked := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := dispositionrouter.StageIntent(filepath.Join(blocked, "escalations"), dispositionrouter.Intent{ItemID: "a"}); err == nil {
		t.Fatal("StageIntent into an unwritable path returned nil error")
	}
}

// TestLoadIntents_UnreadablePathErrors pins the read-error branch (a directory
// where the staging file should be) as a surfaced failure.
func TestLoadIntents_UnreadablePathErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(dispositionrouter.PendingActionsPath(dir), 0o755); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := dispositionrouter.LoadIntents(dispositionrouter.PendingActionsPath(dir)); err == nil {
		t.Fatal("LoadIntents on a directory returned nil error")
	}
}
