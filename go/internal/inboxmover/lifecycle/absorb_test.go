package lifecycle

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMoverAbsorbRootCopies_DropsAnEqualCopyAndParksADifferentOne(t *testing.T) {
	inbox, m, rec := claimFixture(t)
	stamped := filepath.Join(inbox, "processing", "cycle-1836", "b.json")
	writeItem(t, stamped, `{"id":"b","failure_count":2}`)
	writeItem(t, filepath.Join(inbox, "2026-10-08T00-00-00Z-b.json"), `{"id":"b"}`)

	got, err := m.AbsorbRootCopies()

	want := []Absorbed{
		{ID: "b", Cycle: 1836, RootPath: filepath.Join(inbox, "2026-10-08T00-00-00Z-b.json"), Outcome: AbsorbParked, ParkedPath: filepath.Join(inbox, "origin-conflicts", "cycle-1836", "2026-10-08T00-00-00Z-b.json")},
		{ID: "dup", Cycle: 1836, RootPath: filepath.Join(inbox, "dup.json"), Outcome: AbsorbDropped},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("AbsorbRootCopies = %+v, %v; want %+v", got, err, want)
	}
	if body, _ := os.ReadFile(stamped); string(body) != `{"id":"b","failure_count":2}` {
		t.Errorf("the lane's stamped claim copy was changed: %s", body)
	}
	if body, _ := os.ReadFile(want[0].ParkedPath); string(body) != `{"id":"b"}` {
		t.Errorf("origin's copy must be parked beside the claim, got %q", body)
	}
	for _, p := range []string{want[0].RootPath, want[1].RootPath} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s must leave the root: %v", p, err)
		}
	}
	if len(rec.records) != 2 || rec.records[0].Action != "park" || rec.records[1].Action != "absorb" || !strings.Contains(rec.records[0].Message, "origin-conflicts/cycle-1836") {
		t.Errorf("ledger = %+v; want one park and one absorb line", rec.records)
	}
	again, err := m.AbsorbRootCopies()
	if err != nil || len(again) != 0 || len(rec.records) != 2 {
		t.Errorf("a second absorb moves nothing: %+v %v", again, err)
	}
}

func TestMoverAbsorbRootCopies_ReportsEachFileSystemFault(t *testing.T) {
	cases := []struct {
		name, lock, root, want string
		mode                   os.FileMode
	}{
		{"the claim list fails", "processing/cycle-1836", `{"id":"dup"}`, "list claims", 0o000},
		{"the equal root copy cannot be removed", ".", `{"id":"dup"}`, "remove the root copy", 0o555},
		{"origin's copy cannot be parked", ".", `{"id":"dup","weight":1}`, "park origin's copy", 0o555},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inbox, m, rec := claimFixture(t)
			writeItem(t, filepath.Join(inbox, "dup.json"), tc.root)
			chmod(t, filepath.Join(inbox, tc.lock), tc.mode)

			_, err := m.AbsorbRootCopies()

			if err == nil || !strings.Contains(err.Error(), tc.want) || len(rec.records) != 0 {
				t.Errorf("err = %v, records = %d; want an error containing %q and no ledger line", err, len(rec.records), tc.want)
			}
		})
	}
}

func TestMoveExclusive_NeverOverwritesAndRollsBackAFailedRemoval(t *testing.T) {
	dir := t.TempDir()
	src, dest := filepath.Join(dir, "src", "a.json"), filepath.Join(dir, "a.json")
	writeItem(t, src, "src")
	writeItem(t, dest, "dest")
	if err := moveExclusive(src, dest); !errors.Is(err, os.ErrExist) {
		t.Errorf("an existing destination: err = %v, want ErrExist", err)
	}
	if body, _ := os.ReadFile(dest); string(body) != "dest" {
		t.Errorf("the destination was overwritten: %q", body)
	}
	if err := os.Remove(dest); err != nil {
		t.Fatal(err)
	}
	chmod(t, filepath.Dir(src), 0o555)
	if err := moveExclusive(src, dest); err == nil {
		t.Error("a source that cannot be removed must fail the move")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("the failed move must roll back the new link: %v", err)
	}
}
