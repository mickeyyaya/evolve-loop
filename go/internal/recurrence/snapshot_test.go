package recurrence

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func dirEntryNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names
}

func TestReadSnapshot_DecodesAsLoadDoesWithoutTakingTheLock(t *testing.T) {
	stored := `{"entries":{"p":{"pattern":"p","cycles":[1,2],"count":2,"fix_item_id":"fix-p"}}}`
	cases := []struct {
		name    string
		body    string
		present bool
		want    *Ledger
	}{
		{"a missing file is an empty ledger", "", false, NewLedger()},
		{"an empty file is an empty ledger", "", true, NewLedger()},
		{"a ledger without entries gets an empty map", `{}`, true, NewLedger()},
		{"a null entries map gets an empty map", `{"entries":null}`, true, NewLedger()},
		{"a stored ledger decodes", stored, true, &Ledger{Entries: map[string]*Entry{
			"p": {Pattern: "p", Cycles: []int{1, 2}, Count: 2, FixItemID: "fix-p"},
		}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "recurrence-ledger.json")
			if tc.present {
				if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			before := dirEntryNames(t, dir)

			got, err := ReadSnapshot(path)

			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ReadSnapshot = %+v, %v; want %+v", got, err, tc.want)
			}
			if after := dirEntryNames(t, dir); !reflect.DeepEqual(after, before) {
				t.Errorf("ReadSnapshot changed the directory from %v to %v: it must take no lock", before, after)
			}
			locked, err := Load(path)
			if err != nil || !reflect.DeepEqual(locked, got) {
				t.Errorf("Load = %+v, %v; want the snapshot %+v", locked, err, got)
			}
		})
	}
}

func TestReadSnapshot_AMalformedOrUnreadableLedgerIsAnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "recurrence-ledger.json")
	if err := os.WriteFile(path, []byte(`{"entries":`), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadSnapshot(path)

	if err == nil || got != nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("ReadSnapshot = %+v, %v; want no ledger and an error naming %s", got, err, path)
	}
	if _, lerr := Load(path); lerr == nil || lerr.Error() != err.Error() {
		t.Errorf("Load error = %v, want the snapshot's own %v", lerr, err)
	}
	if got, err := ReadSnapshot(dir); err == nil || got != nil {
		t.Errorf("ReadSnapshot(a directory) = %+v, %v; want a read error", got, err)
	}
}

func TestLedger_ItemCounts_CountsThePatternAnItemCarriesOrFixes(t *testing.T) {
	ledger := &Ledger{Entries: map[string]*Entry{
		"autofiled-pattern": {Pattern: "autofiled-pattern", Count: 4},
		"some-lesson":       {Pattern: "some-lesson", Count: 6, FixItemID: "fix-item"},
		"other-lesson":      {Pattern: "other-lesson", Count: 2, FixItemID: "fix-item"},
		"vocabulary-noise":  {Pattern: "vocabulary-noise", Count: 9, FixItemID: "quiet-item", Generic: true},
		"torn-entry":        nil,
	}}

	got := ledger.ItemCounts()

	want := map[string]int{"autofiled-pattern": 4, "some-lesson": 6, "other-lesson": 2, "fix-item": 6}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ItemCounts = %v, want %v: the largest non-generic count under its pattern and its fix_item_id", got, want)
	}
	if got := (*Ledger)(nil).ItemCounts(); len(got) != 0 {
		t.Errorf("a nil ledger counts %v, want none", got)
	}
}
