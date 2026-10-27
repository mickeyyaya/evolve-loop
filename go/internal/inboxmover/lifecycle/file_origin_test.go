package lifecycle

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func filedFields(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestMover_File_StampsTheFilersOriginOnAnItemThatNamesNone(t *testing.T) {
	res, err := newFiler(newInbox(t), nil, WithOrigin(inboxbatch.OriginCycle)).File([]byte(validItem))
	if err != nil {
		t.Fatal(err)
	}
	if got := filedFields(t, res.Path)["origin"]; got != "cycle" {
		t.Errorf("origin = %v, want the filer's origin cycle", got)
	}
}

func TestMover_File_KeepsAnAuthoredOrigin(t *testing.T) {
	body := strings.Replace(validItem, `"priority_class":"debuggability"`, `"priority_class":"debuggability","origin":"retro"`, 1)

	res, err := newFiler(newInbox(t), nil, WithOrigin(inboxbatch.OriginCycle)).File([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if got := filedFields(t, res.Path)["origin"]; got != "retro" {
		t.Errorf("origin = %v, want the authored retro", got)
	}
}

func TestMover_File_RefusesAnItemWithNoOriginOrAnUnknownOne(t *testing.T) {
	unknown := strings.Replace(validItem, `"priority_class":"debuggability"`, `"priority_class":"debuggability","origin":"lane"`, 1)
	for name, tc := range map[string]struct {
		filer *Mover
		body  string
	}{
		"no origin anywhere": {New(newInbox(t), nil, WithNow(filingClock), WithPriorityClasses(filingClassOrder)), validItem},
		"an unknown origin":  {newFiler(newInbox(t), nil), unknown},
	} {
		if _, err := tc.filer.File([]byte(tc.body)); !errors.Is(err, ErrInvalidItem) || !errors.Is(err, inboxbatch.ErrUnknownOrigin) {
			t.Errorf("%s: err = %v, want ErrInvalidItem and ErrUnknownOrigin", name, err)
		}
	}
}

func TestMover_File_StampsTheIntakeRecordAndRefusesAnAuthoredOne(t *testing.T) {
	record := json.RawMessage(`{"reason":"two pools","overlaps":["a"]}`)
	res, err := newFiler(newInbox(t), nil, WithIntakeRecord(record)).File([]byte(validItem))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := filedFields(t, res.Path)[inboxbatch.IntakeOverrideField].(map[string]any)
	if got["reason"] != "two pools" {
		t.Errorf("intake_override = %v, want the record", got)
	}

	forged := strings.Replace(validItem, `"priority_class":"debuggability"`, `"priority_class":"debuggability","intake_override":{"reason":"x"}`, 1)
	if _, err := newFiler(newInbox(t), nil).File([]byte(forged)); !errors.Is(err, ErrInvalidItem) || !strings.Contains(err.Error(), "intake_override") {
		t.Errorf("an authored intake_override: err = %v, want ErrInvalidItem naming it", err)
	}
}

func TestMover_Withdraw_AnItemFiledWithAnIntakeRecordIsStillAsFiled(t *testing.T) {
	m := newFiler(newInbox(t), &recordingAppender{}, WithBinding(unbound), WithIntakeRecord(json.RawMessage(`{"reason":"two pools"}`)))
	filed, err := m.File([]byte(validItem))
	if err != nil {
		t.Fatal(err)
	}

	if got, err := m.Withdraw("cli-inbox-show", "filed twice"); err != nil || got != filed.Path {
		t.Fatalf("Withdraw = (%q, %v), want the item withdrawn: intake_override is written at filing", got, err)
	}
}

func TestNew_AnEmptyOriginOrIntakeRecordNeverClobbersAnInstalledOne(t *testing.T) {
	m := newFiler(newInbox(t), nil, WithOrigin(inboxbatch.OriginRetro), WithOrigin(""),
		WithIntakeRecord(json.RawMessage(`{"reason":"kept"}`)), WithIntakeRecord(nil))

	res, err := m.File([]byte(validItem))
	if err != nil {
		t.Fatal(err)
	}
	doc := filedFields(t, res.Path)
	if record, _ := doc[inboxbatch.IntakeOverrideField].(map[string]any); doc["origin"] != "retro" || record["reason"] != "kept" {
		t.Errorf("filed = %v, want origin retro and the kept record", doc)
	}
}
