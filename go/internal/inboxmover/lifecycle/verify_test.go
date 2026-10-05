package lifecycle

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const verifiedSHA = "0123456789abcdef0123456789abcdef01234567"

func mainAt(sha string) func() (string, error) { return func() (string, error) { return sha, nil } }

func TestMover_VerifyPremise_StampsThePendingItemAndRecordsIt(t *testing.T) {
	inbox := newInbox(t)
	path := filepath.Join(inbox, "2026-09-26T07-10-00Z-stale.json")
	writeItem(t, path, `{"id":"stale","weight":0.4,"created_at":"2026-09-26T07:10:00Z"}`)
	rec := &recordingAppender{}
	at := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	m := New(inbox, rec, WithNow(func() time.Time { return at }), WithMainHead(mainAt(verifiedSHA)))

	got, err := m.VerifyPremise("stale", "  go/internal/x.go:12 still reads the old field  ")

	if err != nil || got != path {
		t.Fatalf("VerifyPremise = (%q, %v)", got, err)
	}
	item := readItem(t, path)
	if item["premise_verified_at"] != "2026-10-01T09:00:00Z" || item["premise_verified_sha"] != verifiedSHA ||
		item["premise_verified_evidence"] != "go/internal/x.go:12 still reads the old field" ||
		item["created_at"] != "2026-09-26T07:10:00Z" || item["weight"] != 0.4 {
		t.Errorf("item = %v", item)
	}
	if len(rec.records) != 1 || rec.records[0].Action != "verify-premise" || rec.records[0].TaskID != "stale" ||
		rec.records[0].GitHead != verifiedSHA || rec.records[0].Message != "go/internal/x.go:12 still reads the old field" {
		t.Errorf("ledger = %+v", rec.records)
	}
}

func TestMover_VerifyPremise_RefusesBlankEvidenceAndAnyItemNotPending(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, procPath(inbox, 12, "held.json"), `{"id":"held"}`)
	writeItem(t, filepath.Join(inbox, "consumed", "gone.json"), `{"id":"gone"}`)
	writeItem(t, filepath.Join(inbox, "x.json"), `{"id":"x"}`)
	m := New(inbox, nil, WithMainHead(mainAt(verifiedSHA)))

	for _, id := range []string{"held", "gone", "absent"} {
		if _, err := m.VerifyPremise(id, "checked"); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: err = %v, want ErrNotFound", id, err)
		}
	}
	for name, args := range map[string][2]string{"blank evidence": {"x", " "}, "an empty id": {"", "checked"}} {
		if _, err := m.VerifyPremise(args[0], args[1]); !errors.Is(err, ErrBadArgs) {
			t.Errorf("%s: err = %v, want ErrBadArgs", name, err)
		}
	}
	if item := readItem(t, filepath.Join(inbox, "x.json")); len(item) != 1 {
		t.Errorf("item = %v, want it unstamped", item)
	}
}

func TestMover_VerifyPremise_AMainHeadFaultStampsNothing(t *testing.T) {
	inbox := newInbox(t)
	path := filepath.Join(inbox, "x.json")
	writeItem(t, path, `{"id":"x"}`)
	before, _ := os.ReadFile(path)
	rec := &recordingAppender{}

	for name, m := range map[string]*Mover{
		"no reader wired":   New(inbox, rec),
		"a failing reader":  New(inbox, rec, WithMainHead(func() (string, error) { return "", errors.New("no origin/main") })),
		"an empty sha read": New(inbox, rec, WithMainHead(mainAt(""))),
	} {
		if _, err := m.VerifyPremise("x", "checked"); err == nil || errors.Is(err, ErrNotFound) || errors.Is(err, ErrBadArgs) {
			t.Errorf("%s: err = %v, want the main-head fault", name, err)
		}
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) || len(rec.records) != 0 {
		t.Errorf("a stamp without a sha was written or recorded")
	}
}

func TestMover_VerifyPremise_ARewriteFaultIsNotAMissingItem(t *testing.T) {
	inbox := newInbox(t)
	path := filepath.Join(inbox, "x.json")
	writeItem(t, path, `{"id":"x"}`)
	rec := &recordingAppender{}
	if err := os.Chmod(inbox, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(inbox, 0o755) })

	_, err := New(inbox, rec, WithMainHead(mainAt(verifiedSHA))).VerifyPremise("x", "checked")

	if err == nil || errors.Is(err, ErrNotFound) || errors.Is(err, ErrBadArgs) {
		t.Errorf("err = %v, want the rewrite fault itself", err)
	}
	if len(rec.records) != 0 {
		t.Errorf("a failed stamp was recorded: %+v", rec.records)
	}
}
