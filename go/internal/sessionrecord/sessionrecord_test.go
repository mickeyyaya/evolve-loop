package sessionrecord

import (
	"os"
	"testing"
)

func TestAppendReadRoundTrip(t *testing.T) {
	t.Parallel()
	path := PathIn(t.TempDir())
	want := Record{Session: "evolve-bridge-rAAAA0000-c1-build-pid9-7", RunID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Cycle: 1, Agent: "build", PID: 9}
	if err := Append(path, want); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got, err := ReadAll(path)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(got) != 1 || got[0] != want {
		t.Errorf("ReadAll=%+v, want [%+v]", got, want)
	}
}

func TestReadAllMissingFileIsEmpty(t *testing.T) {
	t.Parallel()
	got, err := ReadAll(PathIn(t.TempDir()))
	if err != nil || got != nil {
		t.Errorf("ReadAll(missing) = (%v, %v), want (nil, nil) — no sessions is success", got, err)
	}
}

// TestReadAllSkipsMalformedLines: a crash can half-write a line; the reaper
// must still reap the well-formed remainder rather than abandoning the whole
// registry (which would leak every session of the run).
func TestReadAllSkipsMalformedLines(t *testing.T) {
	t.Parallel()
	path := PathIn(t.TempDir())
	if err := Append(path, Record{Session: "evolve-bridge-rAAAA0000-c1-tdd-pid9-7"}); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"session":"evolve-bridge-truncat`); err != nil {
		t.Fatal(err)
	}
	f.Close()
	got, err := ReadAll(path)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(got) != 1 || got[0].Session != "evolve-bridge-rAAAA0000-c1-tdd-pid9-7" {
		t.Errorf("ReadAll=%+v, want exactly the well-formed record", got)
	}
}
