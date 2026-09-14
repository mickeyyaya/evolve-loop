package lifecycle

// promote_test.go — Promote's contract through the leaf (§6 tests 25-30).

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Test 25 — a promote of an id nobody holds is (NoOp=true, nil): the code
// fires with task_id/state, no ledger line, no retire call.
func TestMover_Promote_NotFound_NoOp_EmitsPromoteNotFound(t *testing.T) {
	inbox := newInbox(t)
	rc := newRecordingCenter()
	rec := &recordingAppender{}
	retired := 0
	m := New(inbox, rec, WithSignals(rc.accessor()), WithRetire(func(string, string, string) { retired++ }))
	res, err := m.Promote("ghost", "processed", PromoteOpts{Cycle: "3"})
	if err != nil || !res.NoOp || res.SrcPath != "" || res.DestPath != "" {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if len(rc.events) != 1 || rc.events[0].Code != CodePromoteNotFound || rc.events[0].Fields["state"] != "processed" || rc.events[0].Fields["task_id"] != "ghost" {
		t.Errorf("events = %+v", rc.events)
	}
	if len(rec.records) != 0 || retired != 0 {
		t.Errorf("no ledger line (%d) and no retire (%d) on a NoOp", len(rec.records), retired)
	}
}

// Test 26 — an unlanded sha reroutes to retry/ with the overridden reason on
// both the ledger and the retire hook; the code carries sha and state=retry.
func TestMover_Promote_Unlanded_ReroutesToRetry_RetireGetsTheOverriddenReason(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, procPath(inbox, 598, "t1.json"), `{"id":"t1"}`)
	rc := newRecordingCenter()
	rec := &recordingAppender{}
	var hook []string
	m := New(inbox, rec, WithSignals(rc.accessor()), WithLanded(func(sha string) (bool, error) { return sha != "deadbeef00", nil }),
		WithRetire(func(path, id, reason string) { hook = append(hook, filepath.Base(path)+" "+id+" "+reason) }))
	res, err := m.Promote("t1", "processed", PromoteOpts{Cycle: "598", CommitSHA: "deadbeef00"})
	if err != nil || res.NoOp || res.DestPath != filepath.Join(inbox, "retry", "t1.json") {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if _, statErr := os.Stat(filepath.Join(inbox, "processed", "cycle-598", "t1.json")); statErr == nil {
		t.Error("an unlanded sha must never reach processed/")
	}
	if len(rec.records) != 1 || rec.records[0].Message != ".evolve/inbox/processing/t1.json → "+res.DestPath+": ship-promote-retry-unlanded-sha" || rec.records[0].GitHead != "deadbeef00" || rec.records[0].Cycle != 598 {
		t.Errorf("ledger = %+v", rec.records)
	}
	if len(hook) != 1 || hook[0] != "t1.json t1 ship-promote-retry-unlanded-sha" {
		t.Errorf("retire hook = %v", hook)
	}
	if len(rc.events) != 1 || rc.events[0].Code != CodePromoteUnlandedSHA || rc.events[0].Fields["sha"] != "deadbeef00" || rc.events[0].Fields["state"] != "retry" || rc.events[0].Fields["step"] != "landing" {
		t.Errorf("events = %+v", rc.events)
	}
}

// Test 27 — an erroring landing probe fails OPEN (the item lands in processed/)
// and reports INBOX_LANDED_CHECK_FAILED once; the gate is not consulted for a
// non-processed state or a promote without a sha.
func TestMover_Promote_LandedCheckError_FailsOpenWithACode(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, filepath.Join(inbox, "t1.json"), `{"id":"t1"}`)
	writeItem(t, filepath.Join(inbox, "t2.json"), `{"id":"t2"}`)
	writeItem(t, filepath.Join(inbox, "t3.json"), `{"id":"t3"}`)
	rc := newRecordingCenter()
	consulted := 0
	m := New(inbox, nil, WithSignals(rc.accessor()), WithLanded(func(string) (bool, error) {
		consulted++
		return false, errors.New("git exploded")
	}))
	res, err := m.Promote("t1", "processed", PromoteOpts{Cycle: "3", CommitSHA: "deadbeef00"})
	if err != nil || res.NoOp || res.DestPath != filepath.Join(inbox, "processed", "cycle-3", "deadbeef-t1.json") {
		t.Fatalf("fail-open: res = %+v, err = %v", res, err)
	}
	if len(rc.events) != 1 || rc.events[0].Code != CodeLandedCheckFailed || rc.events[0].Origin != "Mover.Promote" || rc.events[0].Fields["sha"] != "deadbeef00" ||
		rc.events[0].Fields["err"] != "git exploded" || rc.events[0].Fields["step"] != "landing" {
		t.Errorf("events = %+v", rc.events)
	}
	if _, err := m.Promote("t2", "retry", PromoteOpts{CommitSHA: "deadbeef00"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Promote("t3", "processed", PromoteOpts{}); err != nil {
		t.Fatal(err)
	}
	if consulted != 1 {
		t.Errorf("the gate is consulted only for processed+sha; consulted %d times", consulted)
	}
}

// Test 28 — a destination mkdir failure is ErrMvFailed with NoOp false and a
// promote-warn/mkdir-failed ledger line; a rename failure is (NoOp=true, nil)
// with promote-warn/mv-failed; neither retires.
func TestMover_Promote_MkdirFails_ErrMvFailed_RenameFails_NoOp(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, procPath(inbox, 5, "t1.json"), `{"id":"t1"}`)
	writeItem(t, filepath.Join(inbox, "processed"), "x")
	rc := newRecordingCenter()
	rec := &recordingAppender{}
	retired := 0
	m := New(inbox, rec, WithSignals(rc.accessor()), WithRetire(func(string, string, string) { retired++ }))
	res, err := m.Promote("t1", "processed", PromoteOpts{Cycle: "5", CommitSHA: "cafef00d00"})
	if !errors.Is(err, ErrMvFailed) || res.NoOp {
		t.Fatalf("mkdir: res = %+v, err = %v", res, err)
	}
	if _, statErr := os.Stat(procPath(inbox, 5, "t1.json")); statErr != nil {
		t.Error("the item must stay where it was")
	}
	if err := os.Remove(filepath.Join(inbox, "processed")); err != nil {
		t.Fatal(err)
	}
	mkdirAll(t, filepath.Join(inbox, "processed", "cycle-5", "cafef00d-t1.json"))
	res, err = m.Promote("t1", "processed", PromoteOpts{Cycle: "5", CommitSHA: "cafef00d00"})
	if err != nil || !res.NoOp {
		t.Fatalf("rename: res = %+v, err = %v", res, err)
	}
	if len(rec.records) != 2 || rec.records[0].Action != "promote-warn" || !strings.HasSuffix(rec.records[0].Message, ": mkdir-failed") ||
		rec.records[1].Action != "promote-warn" || !strings.HasSuffix(rec.records[1].Message, ": mv-failed") || rec.records[1].GitHead != "cafef00d00" {
		t.Errorf("ledger = %+v", rec.records)
	}
	if got := rc.codes(); len(got) != 2 || got[0] != CodePromoteMoveFailed || got[1] != CodePromoteMoveFailed || rc.events[0].Fields["step"] != "mkdir" || rc.events[1].Fields["step"] != "rename" {
		t.Errorf("events = %+v", rc.events)
	}
	if rc.events[1].Fields["src_rel"] != "processing" || rc.events[1].Fields["dest"] == "" {
		t.Errorf("fields = %v", rc.events[1].Fields)
	}
	if retired != 0 {
		t.Errorf("no retire on a failed delivery (%d)", retired)
	}
}

// Test 29 — the promote tail's order: the INFO line, the retire hook, the
// ledger append.
func TestMover_Promote_Order_InfoThenRetireThenLedger(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, filepath.Join(inbox, "t1.json"), `{"id":"t1"}`)
	var trace strings.Builder
	rec := &recordingAppender{onEach: func() { trace.WriteString("<ledger>\n") }}
	m := New(inbox, rec, WithStderr(&trace), WithRetire(func(string, string, string) { trace.WriteString("<retire>\n") }))
	if _, err := m.Promote("t1", "rejected", PromoteOpts{Cycle: "2"}); err != nil {
		t.Fatal(err)
	}
	if got := trace.String(); got != "[inbox-mover] promoted: t1.json → rejected/\n<retire>\n<ledger>\n" {
		t.Errorf("order:\n%s", got)
	}
}

// Test 30 — the destination layout per state (moved from
// inboxmover_test.go:429-453) plus cycleOrZero and the unreachable default.
func TestPromoteDestPath_AndCycleOrZero(t *testing.T) {
	cases := []struct {
		name     string
		state    string
		opts     PromoteOpts
		wantPath string
	}{
		{"processed default cycle", "processed", PromoteOpts{}, "processed/cycle-0/task-x.json"},
		{"processed with cycle", "processed", PromoteOpts{Cycle: "12"}, "processed/cycle-12/task-x.json"},
		{"processed with sha", "processed", PromoteOpts{Cycle: "12", CommitSHA: "deadbeef1234567890"}, "processed/cycle-12/deadbeef-task-x.json"},
		{"processed short sha (no truncate)", "processed", PromoteOpts{Cycle: "12", CommitSHA: "abc"}, "processed/cycle-12/abc-task-x.json"},
		{"rejected default cycle", "rejected", PromoteOpts{}, "rejected/cycle-0/task-x.json"},
		{"retry no cycle", "retry", PromoteOpts{}, "retry/task-x.json"},
		{"quarantine flat", "quarantine", PromoteOpts{Cycle: "12"}, "quarantine/task-x.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, dest := promoteDestPath("/inbox", "task-x.json", tc.state, tc.opts)
			if !strings.HasSuffix(dest, tc.wantPath) || filepath.Dir(dest) != dir {
				t.Errorf("dest = %q (dir %q), want suffix %q", dest, dir, tc.wantPath)
			}
		})
	}
	if dir, dest := promoteDestPath("/inbox", "x.json", "bogus", PromoteOpts{}); dir != "" || dest != "" {
		t.Errorf("an unknown state yields no path: %q %q", dir, dest)
	}
	if cycleOrZero("") != "0" || cycleOrZero("7") != "7" {
		t.Error("cycleOrZero: empty ⇒ 0, else itself")
	}
}

// The usage and bad-state arms keep their verbatim lines; the ledger From path
// keeps its quirks (Q1); a happy promote ledgers the sha, the cycle and the
// state-derived reason.
func TestMover_Promote_UsageBadStateAndLedgerShape(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, filepath.Join(inbox, "r.json"), `{"id":"r"}`)
	var stderr strings.Builder
	rec := &recordingAppender{}
	m := New(inbox, rec, WithStderr(&stderr), WithNow(func() time.Time { return fixedClock }))
	if _, err := m.Promote("", "processed", PromoteOpts{}); !errors.Is(err, ErrBadArgs) {
		t.Errorf("usage: %v", err)
	}
	if _, err := m.Promote("r", "bogus", PromoteOpts{}); !errors.Is(err, ErrBadState) {
		t.Errorf("bad state: %v", err)
	}
	res, err := m.Promote("r", "processed", PromoteOpts{Cycle: "9", CommitSHA: "abcdef1234567890"})
	if err != nil || res.NoOp {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	want := "[inbox-mover] ERROR: usage: promote <task_id> <new_state> [<cycle>] [--commit-sha <sha>]\n[inbox-mover] ERROR: promote: invalid state 'bogus'; must be processed|rejected|retry\n[inbox-mover] promoted: r.json → processed/\n"
	if stderr.String() != want {
		t.Errorf("stderr:\n got %q\nwant %q", stderr.String(), want)
	}
	if len(rec.records) != 1 || rec.records[0].Message != ".evolve/inbox/inbox/r.json → "+res.DestPath+": ship-promote-processed" || rec.records[0].Cycle != 9 || rec.records[0].GitHead != "abcdef1234567890" || rec.records[0].TS != "2026-05-24T12:00:00Z" {
		t.Errorf("ledger = %+v", rec.records)
	}
	body, _ := os.ReadFile(res.DestPath)
	var doc map[string]json.RawMessage
	if json.Unmarshal(body, &doc) != nil || string(doc["id"]) != `"r"` {
		t.Errorf("the item moved intact: %s", body)
	}
}
