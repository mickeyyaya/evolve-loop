package lifecycle

// claim_test.go — Claim's contract through the leaf (§6 tests 22-24).

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Test 22 — a not-found claim emits INBOX_CLAIM_NOT_FOUND with the inbox dir
// and the cycle parsed leniently from the string (unparseable ⇒ 0).
func TestMover_Claim_NotFound_EmitsClaimNotFound(t *testing.T) {
	inbox := newInbox(t)
	rc := newRecordingCenter()
	m := New(inbox, nil, WithSignals(rc.accessor()))
	if _, err := m.Claim("t1", "7"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
	if _, err := m.Claim("t1", "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
	if len(rc.events) != 2 || rc.events[0].Cycle != 7 || rc.events[1].Cycle != 0 {
		t.Fatalf("events = %+v", rc.events)
	}
	if f := rc.events[0].Fields; f["inbox_dir"] != inbox || f["task_id"] != "t1" || f["step"] != "locate" {
		t.Errorf("fields = %v", f)
	}
}

// Test 23 — the ADR-0074 claim floor through the leaf: route:console-* refuses
// with a nil predicate; a protected fix surface refuses only when a predicate
// is injected; route:lane on an operator-authored item claims.
func TestMover_Claim_Refused_EmitsClaimRefused(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, filepath.Join(inbox, "x.json"), `{"id":"task-x","route":"console-manual"}`)
	writeItem(t, filepath.Join(inbox, "y.json"), `{"id":"task-y","files":["go/internal/guards/role.go (fix)"]}`)
	writeItem(t, filepath.Join(inbox, "z.json"), `{"id":"task-z","route":"lane","files":["go/internal/guards/role.go"]}`)
	rc := newRecordingCenter()
	plain := New(inbox, nil, WithSignals(rc.accessor()))
	if _, err := plain.Claim("task-x", "7"); !errors.Is(err, ErrConsoleRouted) {
		t.Fatalf("route:console-* with a nil predicate: %v", err)
	}
	if _, err := os.Stat(filepath.Join(inbox, "x.json")); err != nil {
		t.Fatal("a refused item stays at the root")
	}
	if len(rc.events) != 1 || rc.events[0].Code != CodeClaimRefused || rc.events[0].Fields["reason"] != "route:console-manual" || rc.events[0].Fields["step"] != "route" {
		t.Errorf("events = %+v", rc.events)
	}
	protected := New(inbox, nil, WithSignals(rc.accessor()), WithProtectedPath(func(p string) bool { return p == "go/internal/guards/role.go" }))
	if _, err := protected.Claim("task-y", "7"); !errors.Is(err, ErrConsoleRouted) {
		t.Errorf("a protected fix surface refuses with a predicate: %v", err)
	}
	if res, err := protected.Claim("task-z", "7"); err != nil || res.DestPath == "" {
		t.Errorf("route:lane claims: %+v %v", res, err)
	}
	writeItem(t, filepath.Join(inbox, "y2.json"), `{"id":"task-y2","files":["go/internal/guards/role.go (fix)"]}`)
	if _, err := plain.Claim("task-y2", "7"); err != nil {
		t.Errorf("a nil predicate switches the files-derived rule OFF: %v", err)
	}
}

// Test 24 — INBOX_CLAIM_MOVE_FAILED names its step: mkdir (a FILE at
// processing/, legacy ERROR) and rename (a directory at the destination file,
// legacy WARN); both return ErrMvFailed.
func TestMover_Claim_MoveFailed_StepMkdirAndRename(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, filepath.Join(inbox, "t1.json"), `{"id":"t1"}`)
	writeItem(t, filepath.Join(inbox, "processing"), "x")
	rc := newRecordingCenter()
	var stderr strings.Builder
	m := New(inbox, nil, WithSignals(rc.accessor()))
	legacy := New(inbox, nil, WithStderr(&stderr))
	if _, err := m.Claim("t1", "7"); !errors.Is(err, ErrMvFailed) {
		t.Fatalf("mkdir: %v", err)
	}
	_, _ = legacy.Claim("t1", "7")
	if err := os.Remove(filepath.Join(inbox, "processing")); err != nil {
		t.Fatal(err)
	}
	mkdirAll(t, procPath(inbox, 7, "t1.json"))
	if _, err := m.Claim("t1", "7"); !errors.Is(err, ErrMvFailed) {
		t.Fatalf("rename: %v", err)
	}
	_, _ = legacy.Claim("t1", "7")
	if got := rc.codes(); len(got) != 2 || got[0] != CodeClaimMoveFailed || got[1] != CodeClaimMoveFailed {
		t.Fatalf("codes = %v", got)
	}
	if rc.events[0].Fields["step"] != "mkdir" || rc.events[0].Fields["dest_dir"] == "" || rc.events[1].Fields["step"] != "rename" || rc.events[1].Fields["err"] == "" {
		t.Errorf("fields: %v / %v", rc.events[0].Fields, rc.events[1].Fields)
	}
	lines := faultLines(stderr.String())
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "[inbox-mover] ERROR: claim: mkdir -p ") || !strings.HasPrefix(lines[1], "[inbox-mover] WARN: claim: mv failed for 't1'") {
		t.Errorf("legacy tokens: %q", lines)
	}
}

// The usage arms keep their verbatim ERROR lines and ErrBadArgs; a claim that
// succeeds prints the INFO line and ledgers triage-claim with the cycle.
func TestMover_Claim_UsageAndHappyPath(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, filepath.Join(inbox, "t1.json"), `{"id":"t1"}`)
	var stderr strings.Builder
	rec := &recordingAppender{}
	rc := newRecordingCenter()
	m := New(inbox, rec, WithStderr(&stderr), WithNow(func() time.Time { return fixedClock }), WithSignals(rc.accessor()))
	if _, err := m.Claim("", "7"); !errors.Is(err, ErrBadArgs) {
		t.Errorf("usage: %v", err)
	}
	if _, err := m.Claim("t1", "0"); !errors.Is(err, ErrBadArgs) {
		t.Errorf("cycle 0: %v", err)
	}
	res, err := m.Claim("t1", "7")
	if err != nil || res.SrcPath != filepath.Join(inbox, "t1.json") || res.DestPath != procPath(inbox, 7, "t1.json") {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	want := "[inbox-mover] ERROR: usage: claim <task_id> <cycle>\n[inbox-mover] ERROR: claim: cycle \"0\" is not a positive number\n[inbox-mover] claimed: t1.json → processing/cycle-7/\n"
	if stderr.String() != want {
		t.Errorf("stderr:\n got %q\nwant %q", stderr.String(), want)
	}
	if len(rec.records) != 1 || rec.records[0].Action != "claim" || rec.records[0].Cycle != 7 || rec.records[0].TS != "2026-05-24T12:00:00Z" ||
		rec.records[0].Message != ".evolve/inbox/t1.json → .evolve/inbox/processing/cycle-7/t1.json: triage-claim" {
		t.Errorf("ledger = %+v", rec.records)
	}
	if len(rc.events) != 0 {
		t.Errorf("the usage lines are kept lines, never events: %+v", rc.events)
	}
}

// consoleRoutedReason fails open on an unreadable or malformed item (Q13).
func TestConsoleRoutedReason_FailsOpenOnUnreadableOrMalformed(t *testing.T) {
	inbox := newInbox(t)
	if got := consoleRoutedReason(filepath.Join(inbox, "missing.json"), nil); got != "" {
		t.Errorf("missing: %q", got)
	}
	writeItem(t, filepath.Join(inbox, "bad.json"), `{"id":"bad", MALFORMED`)
	if got := consoleRoutedReason(filepath.Join(inbox, "bad.json"), nil); got != "" {
		t.Errorf("malformed: %q", got)
	}
	writeItem(t, filepath.Join(inbox, "ok.json"), `{"id":"ok"}`)
	if got := consoleRoutedReason(filepath.Join(inbox, "ok.json"), func(string) bool { return true }); got != "" {
		t.Errorf("dispatchable: %q", got)
	}
	writeItem(t, filepath.Join(inbox, "routed.json"), `{"id":"r","route":"console-ops"}`)
	if got := consoleRoutedReason(filepath.Join(inbox, "routed.json"), nil); got != "route:console-ops" {
		t.Errorf("routed: %q", got)
	}
}
