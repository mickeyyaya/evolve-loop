package loopwave

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"
)

func TestFreshen_KeepsAFreshTodoAndDropsAConsumedOne(t *testing.T) {
	h := newHarness(t)
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "fresh")
	lifecycleItem(t, h.evolveDir, inboxmover.StateProcessed, "done")
	freshen := h.e.Freshen()

	kept, ok := freshen(fleet.CycleSpec{Scope: []string{"fresh"}, GoalHash: "g"})
	if !ok || kept.Scope[0] != "fresh" || kept.GoalHash != "g" {
		t.Errorf("freshen(fresh) = %+v, %v, want the spec kept", kept, ok)
	}
	if _, ok := freshen(fleet.CycleSpec{Scope: []string{"done"}}); ok {
		t.Error("freshen(done) kept a consumed todo, want it dropped just before launch")
	}
	if !strings.Contains(h.stderr.String(), "done") {
		t.Errorf("stderr = %q, want fleet's skip line for the consumed todo", h.stderr.String())
	}
}

func TestClaims_ParkRoutesTheTodoToTheConsoleWithTheReason(t *testing.T) {
	h := newHarness(t)
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "slow")
	var claims InboxClaims = h.e.Claims()

	if err := claims.Release("slow", "orchestrator: deadline"); err != nil {
		t.Errorf("release of an unclaimed todo = %v, want nil (nothing to release)", err)
	}
	if err := claims.Park("slow", "deadline twice"); err != nil {
		t.Fatalf("park = %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(h.evolveDir, "inbox", "slow.json"))
	if err != nil {
		t.Fatal(err)
	}
	var item map[string]any
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatal(err)
	}
	if item["route"] != lifecycle.RouteConsoleValue || item["routed_reason"] != "deadline twice" {
		t.Errorf("parked item = %v, want route console with the reason", item)
	}
	if err := claims.Park("missing", "deadline twice"); err == nil {
		t.Error("park of a missing todo returned no error")
	}
	if err := claims.Release("missing", "x"); err == nil {
		t.Error("release of a missing todo returned no error")
	}
}

func TestInboxText_IsTheTitleAndTheSummaryOfEachItem(t *testing.T) {
	h := newHarness(t)
	writeJSON(t, filepath.Join(h.evolveDir, "inbox", "t.json"), map[string]any{"id": "t", "title": "fix go/internal/fleet", "summary": "and more"})
	if got := h.e.InboxText(3)("t"); got != "fix go/internal/fleet\nand more" {
		t.Errorf("InboxText = %q", got)
	}
}

func TestClaims_TheLifecycleLedgerLandsUnderTheEngineEvolveDirNotTheWorkingDir(t *testing.T) {
	h := newHarness(t)
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "slow")
	cwd := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })

	if err := h.e.Claims().Park("slow", "deadline twice"); err != nil {
		t.Fatalf("park = %v", err)
	}

	if _, err := os.Stat(filepath.Join(h.evolveDir, "ledger.jsonl")); err != nil {
		t.Errorf("ledger under the engine evolve dir: %v, want the park recorded there", err)
	}
	if _, err := os.Stat(filepath.Join(cwd, ".evolve")); !os.IsNotExist(err) {
		t.Errorf("stat %s/.evolve = %v, want no ledger written under the working dir", cwd, err)
	}
}
