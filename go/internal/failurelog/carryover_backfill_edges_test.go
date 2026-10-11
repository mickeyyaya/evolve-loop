package failurelog

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type carryoverOp struct {
	name string
	run  func(path string) (int, error)
	verb string
}

func carryoverOps() []carryoverOp {
	now := time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC)
	return []carryoverOp{
		{"backfill", func(p string) (int, error) { return BackfillLegacyCarryoverExpiry(p, time.Hour, now) }, "backfill carryover write"},
		{"increment", IncrementCarryoverUnpicked, "increment carryover write"},
	}
}

func TestCarryoverOps_NoOpInputsLeaveTheFileUntouched(t *testing.T) {
	for _, op := range carryoverOps() {
		for name, body := range map[string]string{
			"no carryover key":       `{"other":1}`,
			"empty carryover":        `{"carryoverTodos":[]}`,
			"carryover not an array": `{"carryoverTodos":"x"}`,
			"only non-object items":  `{"carryoverTodos":[1,"two",null,[3]]}`,
		} {
			t.Run(op.name+"/"+name, func(t *testing.T) {
				path := mustWrite(t, filepath.Join(t.TempDir(), "state.json"), body)

				n, err := op.run(path)

				if err != nil || n != 0 {
					t.Fatalf("= (%d, %v), want (0, nil)", n, err)
				}
				if got := mustRead(t, path); got != body {
					t.Errorf("state rewritten to %q, want %q untouched", got, body)
				}
			})
		}
	}
}

func TestCarryoverOps_MissingStateIsANoOp(t *testing.T) {
	for _, op := range carryoverOps() {
		t.Run(op.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "absent.json")

			n, err := op.run(path)

			if err != nil || n != 0 {
				t.Fatalf("= (%d, %v), want (0, nil)", n, err)
			}
			if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
				t.Errorf("stat = %v, want the state file still absent", statErr)
			}
		})
	}
}

func TestCarryoverOps_ReadAndParseFailuresAreErrors(t *testing.T) {
	for _, op := range carryoverOps() {
		t.Run(op.name+"/state is a directory", func(t *testing.T) {
			_, err := op.run(t.TempDir())

			if err == nil || !strings.HasPrefix(err.Error(), "failurelog: read state:") {
				t.Errorf("err = %v, want a read-state error", err)
			}
		})
		t.Run(op.name+"/state is not JSON", func(t *testing.T) {
			path := mustWrite(t, filepath.Join(t.TempDir(), "state.json"), "{nope")

			_, err := op.run(path)

			if err == nil || !strings.HasPrefix(err.Error(), "failurelog: parse state:") {
				t.Errorf("err = %v, want a parse-state error", err)
			}
		})
	}
}

func TestCarryoverOps_WriteFailureIsWrappedAndReportsZero(t *testing.T) {
	for _, op := range carryoverOps() {
		t.Run(op.name, func(t *testing.T) {
			path := mustWrite(t, filepath.Join(t.TempDir(), "state.json"), `{"carryoverTodos":[{"id":"a"}]}`)
			prev := atomicWriteJSON
			t.Cleanup(func() { atomicWriteJSON = prev })
			cause := errors.New("disk full")
			atomicWriteJSON = func(string, map[string]any) error { return cause }

			n, err := op.run(path)

			if n != 0 || !errors.Is(err, cause) || !strings.Contains(err.Error(), "failurelog: "+op.verb) {
				t.Errorf("= (%d, %v), want (0, failurelog: %s: disk full)", n, err, op.verb)
			}
		})
	}
}

func TestCarryoverOps_SkipNonObjectItemsButCountObjects(t *testing.T) {
	for _, op := range carryoverOps() {
		t.Run(op.name, func(t *testing.T) {
			path := mustWrite(t, filepath.Join(t.TempDir(), "state.json"), `{"carryoverTodos":["legacy-string",{"id":"a"},7,{"id":"b"}]}`)

			n, err := op.run(path)

			if err != nil || n != 2 {
				t.Fatalf("= (%d, %v), want (2, nil)", n, err)
			}
			todos := brReadState(t, path)
			if !strings.Contains(todos, `"legacy-string"`) || !strings.Contains(todos, "7") {
				t.Errorf("state = %s, want non-object items kept verbatim", todos)
			}
		})
	}
}

func TestBackfillLegacyCarryoverExpiry_ZeroNowUsesTheWallClock(t *testing.T) {
	path := brWriteState(t, t.TempDir(), `{"carryoverTodos":[{"id":"a"},{"id":"b","expiresAt":""}]}`)
	lo := time.Now().UTC().Add(time.Hour).Truncate(time.Second)

	n, err := BackfillLegacyCarryoverExpiry(path, time.Hour, time.Time{})

	hi := time.Now().UTC().Add(time.Hour)
	if err != nil || n != 2 {
		t.Fatalf("= (%d, %v), want (2, nil): an empty expiresAt counts as legacy", n, err)
	}
	for _, todo := range brParseTodos(t, path) {
		got, perr := time.Parse(time.RFC3339, todo["expiresAt"].(string))
		if perr != nil || got.Before(lo) || got.After(hi) {
			t.Errorf("todo %v expiresAt = %v, want wall-clock now+1h in [%v, %v]", todo["id"], todo["expiresAt"], lo, hi)
		}
	}
}

func TestIncrementCarryoverUnpicked_NonNumericCounterRestartsAtOne(t *testing.T) {
	path := brWriteState(t, t.TempDir(), `{"carryoverTodos":[{"id":"s","cycles_unpicked":"3"},{"id":"n","cycles_unpicked":null}]}`)

	n, err := IncrementCarryoverUnpicked(path)

	if err != nil || n != 2 {
		t.Fatalf("= (%d, %v), want (2, nil)", n, err)
	}
	for _, todo := range brParseTodos(t, path) {
		if todo["cycles_unpicked"] != float64(1) {
			t.Errorf("todo %v cycles_unpicked = %v, want 1", todo["id"], todo["cycles_unpicked"])
		}
	}
}

func TestIncrementCarryoverUnpicked_RepeatedCallsCountCycles(t *testing.T) {
	path := brWriteState(t, t.TempDir(), `{"carryoverTodos":[{"id":"a"}]}`)

	for i := 0; i < 3; i++ {
		if _, err := IncrementCarryoverUnpicked(path); err != nil {
			t.Fatal(err)
		}
	}

	if got := brParseTodos(t, path)[0]["cycles_unpicked"]; got != float64(3) {
		t.Errorf("cycles_unpicked after 3 calls = %v, want 3", got)
	}
}
