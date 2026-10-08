package wave_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func blockWithFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func wantErrContaining(t *testing.T, what string, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("%s error = %v, want one that contains %q", what, err, want)
	}
}

func TestStore_ErrorsWhenTheWavesDirIsAFile(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	blockWithFile(t, filepath.Join(evolveDir, "waves"))
	s := wave.NewStore(evolveDir)

	_, recordsErr := s.Records()
	_, goalErr := s.WriteGoal(4, "goal")
	_, noteErr := s.AddNote("a note", time.Now())
	_, notesErr := s.Notes()

	wantErrContaining(t, "Records", recordsErr, "list wave records")
	wantErrContaining(t, "WriteGoal", goalErr, "write wave 4 goal")
	wantErrContaining(t, "AddNote", noteErr, "make the notes dir")
	wantErrContaining(t, "Notes", notesErr, "list notes")
}

func TestStore_SaveRefusesAPathThatIsADirectory(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(evolveDir, "waves", "wave-5.json", "inner"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := wave.NewStore(evolveDir).Save(wave.Record{Number: 5})

	wantErrContaining(t, "Save", err, "write wave record 5")
}

func TestStore_RecordsSkipsOtherFilesAndNamesAnUnreadableRecord(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	s := wave.NewStore(evolveDir)
	saveRecords(t, s, 1)
	if _, err := s.WriteGoal(2, "goal"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddNote("n", time.Now()); err != nil {
		t.Fatal(err)
	}

	records, err := s.Records()
	if err != nil || len(records) != 1 || records[0].Number != 1 {
		t.Fatalf("Records() = %+v, %v; want only record 1 beside the goal file and the notes dir", records, err)
	}
	if os.Geteuid() == 0 {
		t.Skip("root reads a file with mode 000")
	}
	path := filepath.Join(evolveDir, "waves", "wave-1.json")
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	_, err = s.Records()

	wantErrContaining(t, "Records(unreadable)", err, "read wave record 1")
}

func TestStore_NoteErrors(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	s := wave.NewStore(evolveDir)
	at := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	first, err := s.AddNote("first", at)
	if err != nil {
		t.Fatal(err)
	}
	notesDir := filepath.Join(evolveDir, "waves", "notes")
	if err := os.MkdirAll(filepath.Join(notesDir, "sub.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	blockWithFile(t, filepath.Join(notesDir, "stray.txt"))
	blockWithFile(t, filepath.Join(notesDir, "busy.md", "inside"))

	_, collision := s.AddNote("same instant", at)
	notes, listErr := s.Notes()
	removeErr := s.RemoveNotes([]wave.Note{{Name: "busy.md"}})

	wantErrContaining(t, "AddNote(same instant)", collision, "add note "+first.Name)
	if listErr != nil || len(notes) != 1 || notes[0].Name != first.Name {
		t.Errorf("Notes() = %+v, %v; want only the first note, skipping a dir and a non-.md file", notes, listErr)
	}
	wantErrContaining(t, "RemoveNotes(non-empty dir)", removeErr, "remove note busy.md")
}

func TestStore_NotesNamesAnUnreadableNote(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads a file with mode 000")
	}
	evolveDir := t.TempDir()
	s := wave.NewStore(evolveDir)
	n, err := s.AddNote("secret", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(evolveDir, "waves", "notes", n.Name)
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	_, err = s.Notes()

	wantErrContaining(t, "Notes", err, "read note "+n.Name)
}

func TestReadCycles_WarnsOnAnUnreadableLandingIntentAndSignalStream(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/landing/cycle-7.json", `{`)
	if err := os.MkdirAll(filepath.Join(p.root, ".evolve", "runs", "cycle-8", "signals.ndjson"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, warnings := wave.ReadCycles(p.root, 0, 0)

	joined := strings.Join(warnings, "\n")
	if len(got) != 2 || len(warnings) != 2 || !strings.Contains(joined, "cycle-7: landing intent") || !strings.Contains(joined, "cycle-8: signalcenter: read stream") {
		t.Errorf("ReadCycles = %+v, warnings %q; want cycles 7 and 8 with one warning each", got, warnings)
	}
}

func TestLastCycleNumber_NamesAnUnreadableStateFile(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(evolveDir, "state.json"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := wave.LastCycleNumber(evolveDir)

	wantErrContaining(t, "LastCycleNumber", err, "read state.json")
}
