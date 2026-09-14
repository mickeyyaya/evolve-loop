package lifecycle

// route_test.go — RouteConsole's contract through the leaf: the FAIL closeout's
// per-item breaker for a deterministic triage refusal (a top_n card naming a
// protected surface). The item is rewritten IN PLACE — at the inbox root or
// inside processing/cycle-N/ where the lane's claim left it — with
// route:console-manual and the refusal as routed_reason, so the drain
// releases it already operator-owned and the ADR-0074 claim floor refuses
// every later lane. Incident: docs/incidents/2026-09-14-triage-refusal-poison-loop.md.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func readItem(t *testing.T, path string) map[string]any {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return m
}

// Test 25 — a root-resident item is routed in place: the four route fields
// are written atomically, the INFO line and the chained ledger line name the
// reason, and the Center receives INBOX_ITEM_ROUTED_CONSOLE WARN with the
// cycle, the reason and the path.
func TestMover_RouteConsole_RootItem_RewritesInPlaceAndReports(t *testing.T) {
	inbox := newInbox(t)
	path := filepath.Join(inbox, "2026-07-30T13-04-00Z-poison.json")
	writeItem(t, path, `{"id":"poison","priority":"P2","weight":0.86}`)
	rc := newRecordingCenter()
	var stderr strings.Builder
	rec := &recordingAppender{}
	fixed := time.Date(2026, 9, 14, 4, 20, 0, 0, time.UTC)
	m := New(inbox, rec, WithSignals(rc.accessor()), WithStderr(&stderr), WithNow(func() time.Time { return fixed }))

	res, err := m.RouteConsole("poison", "triage refused: top_n card names protected surface go/internal/bridge/x.go", 1675)
	if err != nil {
		t.Fatalf("RouteConsole: %v", err)
	}
	if res.Path != path {
		t.Errorf("Path = %q, want %q", res.Path, path)
	}
	item := readItem(t, path)
	if item["route"] != "console-manual" || item["routed_reason"] != "triage refused: top_n card names protected surface go/internal/bridge/x.go" ||
		item["routed_cycle"] != float64(1675) || item["routed_at"] != "2026-09-14T04:20:00Z" || item["priority"] != "P2" {
		t.Errorf("item = %v", item)
	}
	if !strings.Contains(stderr.String(), LegacyPrefix+"routed console-manual: 2026-07-30T13-04-00Z-poison.json (cycle 1675)") {
		t.Errorf("stderr = %q", stderr.String())
	}
	if len(rec.records) != 1 || rec.records[0].Action != "route-console" || rec.records[0].TaskID != "poison" ||
		!strings.Contains(rec.records[0].Message, "triage refused") {
		t.Errorf("ledger = %+v", rec.records)
	}
	if len(rc.events) != 1 {
		t.Fatalf("events = %+v", rc.events)
	}
	e := rc.events[0]
	if e.Code != CodeItemRoutedConsole || e.Cycle != 1675 || e.Origin != "Mover.RouteConsole" || e.Severity != "WARN" ||
		e.Fields["task_id"] != "poison" || e.Fields["path"] != path || e.Fields["step"] != "route" ||
		!strings.Contains(e.Fields["reason"], "protected surface") || !strings.Contains(e.Reason, "poison") {
		t.Errorf("event = %+v", e)
	}
}

// Test 26 — the closeout case: the lane's claim already moved the item into
// processing/cycle-N/; RouteConsole finds it there (Locate is the one walk)
// and rewrites it in place — it does NOT move it; the drain still does.
func TestMover_RouteConsole_ProcessingItem_RewrittenWhereItLies(t *testing.T) {
	inbox := newInbox(t)
	path := procPath(inbox, 1675, "poison.json")
	writeItem(t, path, `{"id":"poison"}`)
	m := New(inbox, nil)
	res, err := m.RouteConsole("poison", "refused", 1675)
	if err != nil || res.Path != path {
		t.Fatalf("res = %+v err = %v", res, err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Error("the item stays in processing/ — the drain owns the move")
	}
	if item := readItem(t, path); item["route"] != "console-manual" || item["routed_reason"] != "refused" {
		t.Errorf("item = %v", item)
	}
}

// Test 27 — an id nowhere in the inbox is ErrNotFound and reports
// INBOX_ROUTE_NOT_FOUND (step=locate) — a closeout that cannot find the item
// must say so rather than let the poison loop continue silently.
func TestMover_RouteConsole_NotFound_EmitsRouteNotFound(t *testing.T) {
	inbox := newInbox(t)
	rc := newRecordingCenter()
	var stderr strings.Builder
	m := New(inbox, nil, WithSignals(rc.accessor()), WithStderr(&stderr))
	if _, err := m.RouteConsole("ghost", "refused", 3); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
	if len(rc.events) != 1 || rc.events[0].Code != CodeRouteNotFound || rc.events[0].Cycle != 3 ||
		rc.events[0].Fields["task_id"] != "ghost" || rc.events[0].Fields["step"] != "locate" || rc.events[0].Fields["inbox_dir"] != inbox {
		t.Errorf("events = %+v", rc.events)
	}
	if stderr.String() != "" {
		t.Errorf("a wired Center takes the fault; stderr = %q", stderr.String())
	}
	if _, err := m.RouteConsole("", "refused", 3); !errors.Is(err, ErrBadArgs) {
		t.Errorf("empty id: %v", err)
	}
}

// Test 28 — a rewrite that cannot happen (the located file is replaced by a
// directory between Locate and the rewrite) reports INBOX_ITEM_REWRITE_FAILED
// with step=route and returns the fault; nothing is claimed to be routed.
func TestMover_RouteConsole_RewriteFault_ReportsAndFails(t *testing.T) {
	inbox := newInbox(t)
	path := filepath.Join(inbox, "p.json")
	writeItem(t, path, `{"id":"poison"}`)
	rc := newRecordingCenter()
	m := New(inbox, nil, WithSignals(rc.accessor()))
	// Make the atomic rewrite's temp file un-creatable: the inbox dir is read-only.
	if err := os.Chmod(inbox, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(inbox, 0o755) })
	if _, err := m.RouteConsole("poison", "refused", 9); err == nil {
		t.Fatal("a rewrite fault must surface")
	}
	if len(rc.events) != 1 || rc.events[0].Code != CodeItemRewriteFailed || rc.events[0].Fields["step"] != "route" || rc.events[0].Fields["task_id"] != "poison" {
		t.Errorf("events = %+v", rc.events)
	}
	_ = os.Chmod(inbox, 0o755)
	if item := readItem(t, path); item["route"] != nil {
		t.Errorf("the item must be untouched after a failed rewrite: %v", item)
	}
}

// Test 29 — the breaker closes: once routed, the very next lane claim is
// refused by the ADR-0074 floor (INBOX_CLAIM_REFUSED, reason
// route:console-manual) and the item stays at the root.
func TestMover_RouteConsole_ThenClaimIsRefused(t *testing.T) {
	inbox := newInbox(t)
	path := filepath.Join(inbox, "p.json")
	writeItem(t, path, `{"id":"poison"}`)
	rc := newRecordingCenter()
	m := New(inbox, nil, WithSignals(rc.accessor()))
	if _, err := m.RouteConsole("poison", "refused", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Claim("poison", "2"); !errors.Is(err, ErrConsoleRouted) {
		t.Fatalf("claim after route: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Error("a refused item stays at the root")
	}
	if len(rc.events) != 2 || rc.events[1].Code != CodeClaimRefused || rc.events[1].Fields["reason"] != "route:console-manual" {
		t.Errorf("events = %+v", rc.events)
	}
}

// Test 30 — a Location held by ANOTHER cycle's claim is not this closeout's to
// route (a mis-copied id could name a live lane's item): INBOX_ROUTE_NOT_FOUND
// with the holding cycle, ErrNotFound, the file untouched.
func TestMover_RouteConsole_RefusesAnotherCyclesClaim(t *testing.T) {
	inbox := newInbox(t)
	path := procPath(inbox, 42, "live.json")
	writeItem(t, path, `{"id":"live"}`)
	rc := newRecordingCenter()
	m := New(inbox, nil, WithSignals(rc.accessor()))
	if _, err := m.RouteConsole("live", "refused", 7); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
	if len(rc.events) != 1 || rc.events[0].Code != CodeRouteNotFound || rc.events[0].Fields["held_by_cycle"] != "42" || rc.events[0].Fields["step"] != "locate" {
		t.Errorf("events = %+v", rc.events)
	}
	if item := readItem(t, path); item["route"] != nil {
		t.Errorf("another lane's item must be untouched: %v", item)
	}
}

// Test 31 — Policy.Routed: the closeout already routed the refused item, so
// the drain releases the cycle's items plain — no bump, no park — without
// pretending the failure was system-level (architecture review MEDIUM-1).
func TestMover_Release_RoutedPolicyReleasesPlain(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, procPath(inbox, 7, "a.json"), `{"id":"a","route":"console-manual"}`)
	writeItem(t, procPath(inbox, 7, "b.json"), `{"id":"b"}`)
	m := New(inbox, nil)
	res, err := m.Release(7, "cycle-failure-release", &Policy{Ceiling: 1, Committed: map[string]bool{"a": true, "b": true}, Routed: true})
	if err != nil || res.Recovered != 2 {
		t.Fatalf("res = %+v err = %v", res, err)
	}
	for _, id := range []string{"a", "b"} {
		item := readItem(t, filepath.Join(inbox, id+".json"))
		if _, bumped := item["failure_count"]; bumped {
			t.Errorf("%s: routed cycle bumps nothing: %v", id, item)
		}
	}
	if _, err := os.Stat(filepath.Join(inbox, "quarantine")); err == nil {
		t.Error("routed cycle parks nothing")
	}
}
