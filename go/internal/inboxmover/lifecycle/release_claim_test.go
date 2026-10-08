package lifecycle

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func claimFixture(t *testing.T) (string, *Mover, *recordingAppender) {
	t.Helper()
	inbox := filepath.Join(t.TempDir(), ".evolve", "inbox")
	writeItem(t, filepath.Join(inbox, "processing", "cycle-1836", "a.json"), `{"id":"a"}`)
	writeItem(t, filepath.Join(inbox, "processing", "cycle-1836", "dup.json"), `{"id":"dup"}`)
	writeItem(t, filepath.Join(inbox, "dup.json"), `{"id":"dup"}`)
	if err := os.MkdirAll(filepath.Join(inbox, "processing", "cycle-1700"), 0o755); err != nil {
		t.Fatal(err)
	}
	rec := &recordingAppender{}
	return inbox, New(inbox, rec), rec
}

func TestListClaims_NamesEveryClaimItsCycleItsDuplicateAndTheEmptyDirs(t *testing.T) {
	inbox, _, _ := claimFixture(t)

	list, err := ListClaims(inbox)

	want := ClaimList{
		Items: []ClaimedItem{
			{ID: "a", Location: Location{Path: filepath.Join(inbox, "processing", "cycle-1836", "a.json"), Cycle: 1836}},
			{ID: "dup", Location: Location{Path: filepath.Join(inbox, "processing", "cycle-1836", "dup.json"), Cycle: 1836}, Duplicate: true, RootPath: filepath.Join(inbox, "dup.json")},
		},
		EmptyDirs: []Location{{Path: filepath.Join(inbox, "processing", "cycle-1700"), Cycle: 1700}},
	}
	if err != nil || !reflect.DeepEqual(list, want) {
		t.Errorf("ListClaims = %+v, %v; want %+v", list, err, want)
	}
	if list, err := ListClaims(filepath.Join(t.TempDir(), "absent")); err != nil || len(list.Items)+len(list.EmptyDirs) != 0 {
		t.Errorf("an absent inbox lists nothing: %+v %v", list, err)
	}
}

func TestMoverReleaseClaim_MovesTheClaimOrDropsAnEqualDuplicate(t *testing.T) {
	inbox, m, rec := claimFixture(t)
	a := Location{Path: filepath.Join(inbox, "processing", "cycle-1836", "a.json"), Cycle: 1836}

	res, err := m.ReleaseClaim("a", a, "stale")

	if err != nil || res != (ClaimReleaseResult{Path: filepath.Join(inbox, "a.json")}) {
		t.Fatalf("ReleaseClaim(a) = %+v, %v", res, err)
	}
	dup := Location{Path: filepath.Join(inbox, "processing", "cycle-1836", "dup.json"), Cycle: 1836}
	res, err = m.ReleaseClaim("dup", dup, "stale")
	if err != nil || res != (ClaimReleaseResult{Path: filepath.Join(inbox, "dup.json"), Duplicate: true}) {
		t.Fatalf("ReleaseClaim(dup) = %+v, %v", res, err)
	}
	if _, err := os.Stat(dup.Path); !os.IsNotExist(err) {
		t.Errorf("the duplicate claim copy must be removed: %v", err)
	}
	if len(rec.records) != 2 || rec.records[0].Message != ".evolve/inbox/processing/cycle-1836/a.json → .evolve/inbox/a.json: stale" ||
		!strings.HasSuffix(rec.records[1].Message, "stale; duplicate of the root copy removed") {
		t.Errorf("ledger = %+v", rec.records)
	}
}

func TestMoverReleaseClaim_RefusesARootFileOfTheSameNameAndAnotherID(t *testing.T) {
	inbox, m, rec := claimFixture(t)
	writeItem(t, filepath.Join(inbox, "a.json"), `{"id":"other"}`)
	a := Location{Path: filepath.Join(inbox, "processing", "cycle-1836", "a.json"), Cycle: 1836}

	_, err := m.ReleaseClaim("a", a, "stale")

	if !errors.Is(err, ErrClaimConflict) || len(rec.records) != 0 {
		t.Errorf("err = %v, records = %d; want ErrClaimConflict and no ledger line", err, len(rec.records))
	}
	if _, statErr := os.Stat(a.Path); statErr != nil {
		t.Errorf("the claim must stay: %v", statErr)
	}
}

func TestListClaims_ReportsAnUnreadableInboxOrClaimDir(t *testing.T) {
	inbox, _, _ := claimFixture(t)
	notADir := filepath.Join(t.TempDir(), "inbox-file")
	writeItem(t, notADir, "x")
	if _, err := ListClaims(notADir); err == nil || !strings.Contains(err.Error(), "scan the inbox root") {
		t.Errorf("a root that is not a dir: err = %v", err)
	}
	locked := filepath.Join(inbox, "processing", "cycle-1836")
	chmod(t, locked, 0o000)
	if _, err := ListClaims(inbox); err == nil || !strings.Contains(err.Error(), "cycle-1836") {
		t.Errorf("an unreadable claim dir: err = %v", err)
	}
}

func TestMoverReleaseClaim_ReportsEachFileSystemFault(t *testing.T) {
	cases := []struct {
		name, lock string
		mode       os.FileMode
		id, want   string
	}{
		{"the root cannot be scanned", ".", 0o000, "a", "scan the inbox root"},
		{"the root refuses the move", ".", 0o555, "a", "mv failed"},
		{"the claim copy cannot be read", "processing/cycle-1836/dup.json", 0o000, "dup", "compare the claim copy"},
		{"the claim dir refuses the removal", "processing/cycle-1836", 0o555, "dup", "remove the duplicate claim"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inbox, m, rec := claimFixture(t)
			loc := Location{Path: filepath.Join(inbox, "processing", "cycle-1836", tc.id+".json"), Cycle: 1836}
			chmod(t, filepath.Join(inbox, tc.lock), tc.mode)

			_, err := m.ReleaseClaim(tc.id, loc, "stale")

			if err == nil || !strings.Contains(err.Error(), tc.want) || len(rec.records) != 0 {
				t.Errorf("err = %v, records = %d; want an error containing %q and no ledger line", err, len(rec.records), tc.want)
			}
		})
	}
}

func chmod(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, info.Mode().Perm()) })
}

func TestMoverReleaseClaim_RefusesARootCopyOfTheSameIDWithOtherBytes(t *testing.T) {
	inbox, m, rec := claimFixture(t)
	writeItem(t, filepath.Join(inbox, "dup.json"), `{"id":"dup","weight":0.9}`)
	dup := Location{Path: filepath.Join(inbox, "processing", "cycle-1836", "dup.json"), Cycle: 1836}

	_, err := m.ReleaseClaim("dup", dup, "stale")

	if !errors.Is(err, ErrClaimConflict) || len(rec.records) != 0 {
		t.Errorf("err = %v, records = %d; want ErrClaimConflict and no ledger line", err, len(rec.records))
	}
	if _, statErr := os.Stat(dup.Path); statErr != nil {
		t.Errorf("the claim copy must stay: %v", statErr)
	}
}
