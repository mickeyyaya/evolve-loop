package lifecycle

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMover_Withdraw_RemovesAnAsFiledItemAndTheIDMayBeFiledAgain(t *testing.T) {
	inbox := newInbox(t)
	rec := &recordingAppender{}
	m := New(inbox, rec, WithNow(filingClock), WithBinding(unbound))
	filed, err := m.File([]byte(validItem))
	if err != nil {
		t.Fatal(err)
	}

	body, _ := os.ReadFile(filed.Path)

	got, err := m.Withdraw("cli-inbox-show", "  filed under the wrong scope  ")

	if err != nil || got != filed.Path {
		t.Fatalf("Withdraw = (%q, %v), want (%q, nil)", got, err, filed.Path)
	}
	if _, statErr := os.Stat(filed.Path); !os.IsNotExist(statErr) {
		t.Errorf("the withdrawn item is still on disk: %v", statErr)
	}
	last := rec.records[len(rec.records)-1]
	if last.Action != "withdraw" || last.TaskID != "cli-inbox-show" || !strings.Contains(last.Message, ": filed under the wrong scope; sha256=") ||
		!strings.Contains(last.Message, filepath.Base(filed.Path)) {
		t.Errorf("ledger = %+v, want a withdraw line naming the file and the trimmed reason", last)
	}
	digest := sha256.Sum256(body)
	if !strings.Contains(last.Message, "sha256="+hex.EncodeToString(digest[:])) || !strings.HasSuffix(last.Message, "body="+string(body)) {
		t.Errorf("ledger message = %q, want the withdrawn body and its digest", last.Message)
	}
	if _, err := m.File([]byte(validItem)); err != nil {
		t.Errorf("filing the withdrawn id again: %v, want it filed", err)
	}
}

func unbound(string) (bool, error) { return false, nil }

func TestMover_Withdraw_RefusesAnItemAContinuationBinds(t *testing.T) {
	inbox := newInbox(t)
	path := filepath.Join(inbox, "x.json")
	writeItem(t, path, `{"id":"x"}`)
	rec := &recordingAppender{}
	bound := func(id string) (bool, error) { return id == "x", nil }

	_, err := New(inbox, rec, WithBinding(bound)).Withdraw("x", "mistake")

	if !errors.Is(err, ErrNotWithdrawable) || !strings.Contains(err.Error(), "evolve continuation release x") {
		t.Errorf("err = %v, want ErrNotWithdrawable naming the release command", err)
	}
	if _, statErr := os.Stat(path); statErr != nil || len(rec.records) != 0 {
		t.Errorf("a bound item was withdrawn or recorded: %v %+v", statErr, rec.records)
	}
	for name, m := range map[string]*Mover{
		"no binding reader wired":  New(inbox, rec),
		"a failing binding reader": New(inbox, rec, WithBinding(func(string) (bool, error) { return false, errors.New("registry unreadable") })),
	} {
		if _, err := m.Withdraw("x", "mistake"); err == nil || errors.Is(err, ErrNotWithdrawable) || errors.Is(err, ErrNotFound) {
			t.Errorf("%s: err = %v, want the binding fault, never a withdrawal", name, err)
		}
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("the item is gone: %v", statErr)
	}
}

func TestMover_Withdraw_RefusesAnItemALifecycleVerbHasWritten(t *testing.T) {
	for _, field := range []string{`"routed_reason":"x"`, `"failure_count":1`, `"continuation":{}`, `"premise_verified_at":"2026-09-30T00:00:00Z"`} {
		t.Run(field, func(t *testing.T) {
			inbox := newInbox(t)
			path := filepath.Join(inbox, "x.json")
			writeItem(t, path, `{"id":"x",`+field+`}`)
			before, _ := os.ReadFile(path)
			rec := &recordingAppender{}

			_, err := New(inbox, rec, WithBinding(unbound)).Withdraw("x", "mistake")

			if !errors.Is(err, ErrNotWithdrawable) {
				t.Errorf("err = %v, want ErrNotWithdrawable", err)
			}
			if after, _ := os.ReadFile(path); !bytes.Equal(before, after) || len(rec.records) != 0 {
				t.Errorf("a refused withdrawal changed the item or the ledger")
			}
		})
	}
}

func TestMover_Withdraw_RefusesAnItemAPendingItemDependsOn(t *testing.T) {
	inbox := newInbox(t)
	path := filepath.Join(inbox, "base.json")
	writeItem(t, path, `{"id":"base"}`)
	writeItem(t, procPath(inbox, 12, "user.json"), `{"id":"user","deps":["base"]}`)
	writeItem(t, filepath.Join(inbox, "consumed", "old.json"), `{"id":"old","deps":["base"]}`)
	m := New(inbox, nil, WithBinding(unbound))

	_, err := m.Withdraw("base", "mistake")

	if !errors.Is(err, ErrNotWithdrawable) || !strings.Contains(err.Error(), "user") || strings.Contains(err.Error(), "old") {
		t.Errorf("err = %v, want ErrNotWithdrawable naming the pending dependent only", err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("the item is gone: %v", statErr)
	}
}

func TestMover_Withdraw_LeavesAClaimedRetiredOrMissingItemAlone(t *testing.T) {
	inbox := newInbox(t)
	claimed := procPath(inbox, 1780, "held.json")
	writeItem(t, claimed, `{"id":"held"}`)
	writeItem(t, filepath.Join(inbox, "consumed", "gone.json"), `{"id":"gone"}`)
	m := New(inbox, nil, WithBinding(unbound))

	for _, id := range []string{"held", "gone", "absent"} {
		if _, err := m.Withdraw(id, "mistake"); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: err = %v, want ErrNotFound", id, err)
		}
	}
	if _, err := os.Stat(claimed); err != nil {
		t.Errorf("the claimed item is gone: %v", err)
	}
	for name, args := range map[string][2]string{"an empty id": {"", "mistake"}, "a blank reason": {"held", "  "}} {
		if _, err := m.Withdraw(args[0], args[1]); !errors.Is(err, ErrBadArgs) {
			t.Errorf("%s: err = %v, want ErrBadArgs", name, err)
		}
	}
}

func TestMover_Withdraw_ARemoveFaultIsNotARefusal(t *testing.T) {
	inbox := newInbox(t)
	path := filepath.Join(inbox, "x.json")
	writeItem(t, path, `{"id":"x"}`)
	rec := &recordingAppender{}
	if err := os.Chmod(inbox, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(inbox, 0o755) })

	_, err := New(inbox, rec, WithBinding(unbound)).Withdraw("x", "mistake")

	if err == nil || errors.Is(err, ErrNotWithdrawable) || errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want the remove fault itself", err)
	}
	if len(rec.records) != 2 || rec.records[0].Action != "withdraw" || rec.records[1].Action != "withdraw-aborted" {
		t.Errorf("ledger = %+v, want the withdrawal recorded, then its abort", rec.records)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("the item is gone: %v", statErr)
	}
}

func TestMover_Withdraw_AnUnreadableClaimIsAScanFaultAndUnreadableItemsNameNoDependency(t *testing.T) {
	inbox := newInbox(t)
	path := filepath.Join(inbox, "x.json")
	writeItem(t, path, `{"id":"x"}`)
	writeItem(t, filepath.Join(inbox, "malformed.json"), `{"id":`)
	unreadable := filepath.Join(inbox, "unreadable.json")
	writeItem(t, unreadable, `{"id":"u","deps":["x"]}`)
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Dir(procPath(inbox, 12, "held.json"))
	writeItem(t, procPath(inbox, 12, "held.json"), `{"id":"held"}`)
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755); _ = os.Chmod(unreadable, 0o644) })
	m := New(inbox, &recordingAppender{}, WithBinding(unbound))

	if _, err := m.Withdraw("x", "mistake"); err == nil || errors.Is(err, ErrNotWithdrawable) || errors.Is(err, ErrNotFound) {
		t.Errorf("an unreadable claim dir: err = %v, want the scan fault", err)
	}
	if err := os.Chmod(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Withdraw("x", "mistake"); err != nil {
		t.Errorf("unreadable and malformed items name no dependency: %v, want the item withdrawn", err)
	}
}

func TestMover_Withdraw_NoLedgerMeansNoWithdrawal(t *testing.T) {
	inbox := newInbox(t)
	path := filepath.Join(inbox, "x.json")
	writeItem(t, path, `{"id":"x"}`)

	_, err := New(inbox, nil, WithBinding(unbound)).Withdraw("x", "mistake")

	if err == nil || errors.Is(err, ErrNotWithdrawable) {
		t.Errorf("err = %v, want the missing record reported", err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("the item is gone although nothing recorded it: %v", statErr)
	}
}

func TestMover_Withdraw_AFailedRecordKeepsTheItem(t *testing.T) {
	inbox := newInbox(t)
	path := filepath.Join(inbox, "x.json")
	writeItem(t, path, `{"id":"x","title":"t","kind":"bug","summary":"s","fix":"f","weight":0.5,"acceptance":["a"]}`)
	before, _ := os.ReadFile(path)
	rec := &recordingAppender{fail: errors.New("disk full")}

	_, err := New(inbox, rec, WithBinding(unbound)).Withdraw("x", "mistake")

	if err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Errorf("err = %v, want the failed record reported", err)
	}
	if after, readErr := os.ReadFile(path); readErr != nil || !bytes.Equal(before, after) {
		t.Errorf("the item was removed or changed although its only content record failed: %v", readErr)
	}
}
