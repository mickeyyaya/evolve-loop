package wave_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func saveRecords(t *testing.T, s wave.Store, numbers ...int) {
	t.Helper()
	for _, n := range numbers {
		if err := s.Save(wave.Record{Number: n, RunID: "run-" + string(rune('a'+n%26))}); err != nil {
			t.Fatalf("Save(%d): %v", n, err)
		}
	}
}

func TestNextNumber(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		recorded  []int
		requested int
		want      int
		wantErr   error
	}{
		{"no record and no request starts at one", nil, 0, 1, nil},
		{"no record takes the requested seed", nil, 82, 82, nil},
		{"counts up from the last record", []int{80, 81}, 0, 82, nil},
		{"a request past the last record is kept", []int{81}, 90, 90, nil},
		{"a request equal to the last record is refused", []int{81}, 81, 0, wave.ErrNumberTaken},
		{"a request below the last record is refused", []int{81}, 7, 0, wave.ErrNumberTaken},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := wave.NewStore(t.TempDir())
			saveRecords(t, s, c.recorded...)
			records, rerr := s.Records()
			if rerr != nil {
				t.Fatal(rerr)
			}

			got, err := wave.NextNumber(records, c.requested)

			if got != c.want || !errors.Is(err, c.wantErr) {
				t.Errorf("NextNumber(%d) = %d, %v; want %d, %v", c.requested, got, err, c.want, c.wantErr)
			}
		})
	}
}

func TestStore_RecordsRoundTripInNumberOrder(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	s := wave.NewStore(evolveDir)
	ended := time.Date(2026, 10, 8, 13, 0, 0, 0, time.UTC)
	saveRecords(t, s, 12, 3, 100)
	if err := s.Save(wave.Record{Number: 3, RunID: "rewritten", EndedAt: &ended, Outcome: "1 of 1 cycles shipped"}); err != nil {
		t.Fatal(err)
	}

	got, err := s.Records()

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Number != 3 || got[1].Number != 12 || got[2].Number != 100 {
		t.Fatalf("Records() = %+v, want numbers 3, 12, 100 in order", got)
	}
	if got[0].RunID != "rewritten" || got[0].EndedAt == nil || !got[0].EndedAt.Equal(ended) {
		t.Errorf("record 3 = %+v, want the rewrite with its end time", got[0])
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "waves", "wave-12.json")); err != nil {
		t.Errorf("record 12 is not at waves/wave-12.json: %v", err)
	}
}

func TestStore_RecordsRefusesAMalformedRecord(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	s := wave.NewStore(evolveDir)
	saveRecords(t, s, 1)
	if err := os.WriteFile(filepath.Join(evolveDir, "waves", "wave-2.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := s.Records()

	if err == nil {
		t.Fatal("Records() read a malformed wave-2.json without an error")
	}
}

func TestStore_WriteGoal(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	s := wave.NewStore(evolveDir)

	path, err := s.WriteGoal(82, "the goal\n")

	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(evolveDir, "waves", "wave-82-goal.md"); path != want {
		t.Errorf("WriteGoal path = %q, want %q", path, want)
	}
	if b, _ := os.ReadFile(path); string(b) != "the goal\n" {
		t.Errorf("goal file = %q, want %q", b, "the goal\n")
	}
}
