package lifecycle

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestLedgerLine_NilLedgerIsSilent_AppendFailureIsTheVerbatimLine(t *testing.T) {
	var stderr strings.Builder
	rc := newRecordingCenter()
	silent := New(t.TempDir(), nil, WithStderr(&stderr), WithSignals(rc.accessor()))
	silent.ledgerLine(ledgerEntry{Action: "claim", TaskID: "t1"})
	if stderr.Len() != 0 || len(rc.events) != 0 {
		t.Errorf("a nil ledger appends nothing and says nothing: %q %+v", stderr.String(), rc.events)
	}
	failing := &recordingAppender{fail: errors.New("disk on fire")}
	m := New(t.TempDir(), failing, WithStderr(&stderr), WithSignals(rc.accessor()))
	m.ledgerLine(ledgerEntry{Action: "claim", TaskID: "t1"})
	if stderr.String() != "[inbox-mover] WARN: ledger append (inbox-lifecycle claim t1): disk on fire\n" {
		t.Errorf("stderr = %q", stderr.String())
	}
	if len(rc.events) != 0 {
		t.Errorf("no INBOX_ code doubles the ledger adapter's: %+v", rc.events)
	}
	rec := &recordingAppender{}
	cycle, sha := 7, "abc"
	clock := time.Date(2026, 5, 24, 13, 0, 0, 0, time.FixedZone("plus2", 2*3600))
	m = New(t.TempDir(), rec, WithNow(func() time.Time { return clock }))
	m.ledgerLine(ledgerEntry{Action: "promote", TaskID: "t1", From: "a", To: "b", Cycle: &cycle, GitSHA: &sha, Reason: "why"})
	m.ledgerLine(ledgerEntry{Action: "recover", TaskID: "t2", Reason: "bare"})
	if len(rec.records) != 2 || rec.records[0].TS != "2026-05-24T11:00:00Z" || rec.records[0].Cycle != 7 || rec.records[0].GitHead != "abc" || rec.records[0].Message != "a → b: why" ||
		rec.records[1].Cycle != 0 || rec.records[1].GitHead != "" || rec.records[1].Message != "bare" {
		t.Errorf("records = %+v", rec.records)
	}
}

func TestIntPtr_StrPtr_FoldLifecycleMessage(t *testing.T) {
	if intPtr("") != nil || intPtr("garbage") != nil {
		t.Error("intPtr: empty and unparseable ⇒ nil")
	}
	if v := intPtr("42"); v == nil || *v != 42 {
		t.Errorf("intPtr('42') = %v", v)
	}
	if v := intPtr("12x"); v == nil || *v != 12 {
		t.Errorf("intPtr accepts leading digits (Sscanf): %v", v)
	}
	if cycleOf("12x") != 12 || cycleOf("x") != 0 || cycleOf("") != 0 {
		t.Error("cycleOf projects intPtr onto the event's cycle, 0 when nil")
	}
	if strPtr("") != nil {
		t.Error("strPtr('') = non-nil")
	}
	if v := strPtr("hi"); v == nil || *v != "hi" {
		t.Errorf("strPtr('hi') = %v", v)
	}
	if foldLifecycleMessage("", "", "r") != "r" || foldLifecycleMessage("a", "b", "r") != "a → b: r" {
		t.Error("the arrow is dropped only when no paths are involved")
	}
}
