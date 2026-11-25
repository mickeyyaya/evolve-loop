package lifecycle

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

var unsourcedItem = strings.Replace(validItem, `,"source":"console"`, "", 1)

func TestMover_File_StampsTheSourceOfTheFiler(t *testing.T) {
	res, err := newFiler(newInbox(t), nil, WithSource("request:7/inbox-add-1")).File([]byte(unsourcedItem))
	if err != nil {
		t.Fatal(err)
	}
	if got := filedFields(t, res.Path)[inboxbatch.SourceField]; got != "request:7/inbox-add-1" {
		t.Errorf("source = %v, want the filer's source", got)
	}
	plain, err := newFiler(newInbox(t), nil).File([]byte(unsourcedItem))
	if err != nil {
		t.Fatal(err)
	}
	if _, has := filedFields(t, plain.Path)[inboxbatch.SourceField]; has {
		t.Error("a filer with no source stamps none")
	}
}

func TestMover_File_RefusesAnAuthoredSourceWhenTheFilerStampsOne(t *testing.T) {
	inbox := newInbox(t)

	_, err := newFiler(inbox, nil, WithSource("request:7/inbox-add-1")).File([]byte(validItem))

	if !errors.Is(err, ErrInvalidItem) {
		t.Fatalf("err = %v, want ErrInvalidItem", err)
	}
	if filed, _ := filepath.Glob(filepath.Join(inbox, "*.json")); len(filed) != 0 {
		t.Errorf("a refused item is not filed: %v", filed)
	}
}

func TestMover_Withdraw_AnItemFiledWithASourceIsStillAsFiled(t *testing.T) {
	m := newFiler(newInbox(t), &recordingAppender{}, WithBinding(unbound), WithSource("request:7/inbox-add-1"))
	filed, err := m.File([]byte(unsourcedItem))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := m.Withdraw("cli-inbox-show", "filed twice"); err != nil || got != filed.Path {
		t.Fatalf("Withdraw = (%q, %v), want the item withdrawn: source is written at filing", got, err)
	}
}

func TestMover_File_AFilerWithNoSourceKeepsAnAuthoredSource(t *testing.T) {
	res, err := newFiler(newInbox(t), nil).File([]byte(validItem))
	if err != nil {
		t.Fatal(err)
	}
	if got := filedFields(t, res.Path)[inboxbatch.SourceField]; got != "console" {
		t.Errorf("source = %v, want the authored console: the field is free text for every other filer", got)
	}
}
