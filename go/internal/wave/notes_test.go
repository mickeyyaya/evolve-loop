package wave_test

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func TestStore_NotesKeepTheirOrderAndRemoveOnlyTheConsumedOnes(t *testing.T) {
	t.Parallel()
	s := wave.NewStore(t.TempDir())
	at := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	for i, text := range []string{"watch PR 814", "  the quota pause names its wall  "} {
		if _, err := s.AddNote(text, at.Add(time.Duration(i)*time.Nanosecond)); err != nil {
			t.Fatalf("AddNote(%q): %v", text, err)
		}
	}
	consumed, err := s.Notes()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddNote("added after the compose", at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	if err := s.RemoveNotes(consumed); err != nil {
		t.Fatal(err)
	}
	left, err := s.Notes()

	if err != nil {
		t.Fatal(err)
	}
	if len(consumed) != 2 || consumed[0].Text != "watch PR 814" || consumed[1].Text != "the quota pause names its wall" {
		t.Errorf("Notes() before removal = %+v, want the two notes in order, trimmed", consumed)
	}
	if len(left) != 1 || left[0].Text != "added after the compose" {
		t.Errorf("Notes() after removal = %+v, want only the note added after the compose", left)
	}
}

func TestStore_AddNoteRefusesBlankText(t *testing.T) {
	t.Parallel()
	s := wave.NewStore(t.TempDir())

	_, err := s.AddNote(" \n\t", time.Now())
	notes, _ := s.Notes()

	if err == nil || len(notes) != 0 {
		t.Errorf("AddNote(blank) = %v with %d notes stored, want an error and no note", err, len(notes))
	}
}

func TestStore_NotesOnAnEmptyStoreIsEmpty(t *testing.T) {
	t.Parallel()
	notes, err := wave.NewStore(t.TempDir()).Notes()

	if err != nil || len(notes) != 0 {
		t.Errorf("Notes() = %+v, %v; want none and no error", notes, err)
	}
}
