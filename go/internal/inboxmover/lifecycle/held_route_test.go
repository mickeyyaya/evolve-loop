package lifecycle

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMover_Release_ARoutedIDIsNotBumpedAndTheOthersAre(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, procPath(inbox, 5, "routed.json"), `{"id":"routed","title":"R"}`)
	writeItem(t, procPath(inbox, 5, "failed.json"), `{"id":"failed","title":"F"}`)

	if _, err := New(inbox, &recordingAppender{}).Release(5, "", &Policy{Ceiling: 3, RoutedIDs: map[string]bool{"routed": true}}); err != nil {
		t.Fatal(err)
	}

	if _, bumped := readItem(t, filepath.Join(inbox, "routed.json"))["failure_count"]; bumped {
		t.Error("the routed id got a failure bump")
	}
	if got := readItem(t, filepath.Join(inbox, "failed.json"))["failure_count"]; got != float64(1) {
		t.Errorf("the id that failed to route has failure_count %v, want 1", got)
	}
}

func TestMover_EditHeld_EditsTheCyclesOwnClaimInPlace(t *testing.T) {
	inbox := newInbox(t)
	held := procPath(inbox, 7, "held.json")
	writeItem(t, held, `{"id":"held","title":"H","kind":"fix","summary":"s","fix":"f"}`)
	writeItem(t, filepath.Join(inbox, "companion.json"), `{"id":"companion"}`)
	writeItem(t, procPath(inbox, 8, "other.json"), `{"id":"other"}`)
	m := New(inbox, &recordingAppender{})

	path, err := m.EditHeld("held", 7, []FieldEdit{{Op: EditAdd, Field: "deps", Value: "companion"}})
	_, otherErr := m.EditHeld("other", 7, []FieldEdit{{Op: EditAdd, Field: "deps", Value: "companion"}})
	_, missingErr := m.EditHeld("missing", 7, []FieldEdit{{Op: EditAdd, Field: "deps", Value: "companion"}})

	if err != nil || path != held || !reflect.DeepEqual(readItem(t, held)["deps"], []any{"companion"}) {
		t.Errorf("EditHeld = (%q, %v), item %v; want the deps entry written in the claim dir", path, err, readItem(t, held))
	}
	if !errors.Is(otherErr, ErrNotFound) || !errors.Is(missingErr, ErrNotFound) {
		t.Errorf("another cycle's claim = %v, a missing item = %v; want ErrNotFound for both", otherErr, missingErr)
	}
}

func TestMover_EditHeld_RefusesAnEmptyRequest(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, procPath(inbox, 7, "held.json"), `{"id":"held"}`)

	if _, err := New(inbox, &recordingAppender{}).EditHeld("held", 7, nil); err == nil {
		t.Error("an edit with no field edit succeeded")
	}
}

func TestMover_EditHeld_AnUnreadableInboxIsNotANotFound(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), "inbox")
	writeItem(t, inbox, "not a directory")

	_, err := New(inbox, &recordingAppender{}).EditHeld("held", 7, []FieldEdit{{Op: EditAdd, Field: "deps", Value: "x"}})

	if err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want a locate fault that is not ErrNotFound", err)
	}
}
