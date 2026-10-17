package inboxbatch

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeUnder(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func itemIDs(items []Item) []string {
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	return ids
}

func TestScanDir_ResurrectedConsumedItem(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		files        map[string]string
		wantIDs      []string
		wantWarnings []string
	}{
		{
			name: "a root copy of a consumed id is skipped and named with its at and via",
			files: map[string]string{
				"a.json":              `{"id":"rolled","created_at":"T1"}`,
				"consumed/old-a.json": `{"id":"rolled","created_at":"T1","consumed":{"at":"2026-10-08T18:49:03Z","via":"ship"}}`,
			},
			wantWarnings: []string{"a.json: skipped resurrected item rolled", "2026-10-08T18:49:03Z", "via ship", "old-a.json", "remove the file"},
		},
		{
			name: "a later created_at under a consumed id is a new occurrence and is offered",
			files: map[string]string{
				"2026-10-17T00-00-00Z-x.json":          `{"id":"x","created_at":"T2"}`,
				"consumed/2026-10-01T00-00-00Z-x.json": `{"id":"x","created_at":"T1","consumed":{"at":"A","via":"ship"}}`,
			},
			wantIDs: []string{"x"},
		},
		{
			name: "a new id is still offered",
			files: map[string]string{
				"fresh.json":        `{"id":"fresh"}`,
				"consumed/old.json": `{"id":"other","consumed":{"at":"2026-10-08T18:49:03Z","via":"ship"}}`,
			},
			wantIDs: []string{"fresh"},
		},
		{
			name: "two consumed records with one id give one skip",
			files: map[string]string{
				"a.json":                    `{"id":"twice","created_at":"T1"}`,
				"consumed/one.json":         `{"id":"twice","created_at":"T1","consumed":{"at":"2026-10-01T00:00:00Z","via":"ship"}}`,
				"consumed/cycle-7/two.json": `{"id":"twice","created_at":"T1","consumed":{"at":"2026-10-02T00:00:00Z","via":"ship"}}`,
			},
			wantWarnings: []string{"a.json: skipped resurrected item twice"},
		},
		{
			name: "a consumed record without a stamp is named with unknown at and via",
			files: map[string]string{
				"a.json":          `{"id":"bare"}`,
				"consumed/a.json": `{"id":"bare"}`,
			},
			wantWarnings: []string{"skipped resurrected item bare", "at unknown", "via unknown", "record a.json"},
		},
		{
			name: "an id-less root file matches by file name stem",
			files: map[string]string{
				"2026-09-30T11-07-44Z-x.json":          `{"title":"t"}`,
				"consumed/2026-09-30T11-07-44Z-x.json": `{"consumed":{"at":"A","via":"ship"}}`,
			},
			wantWarnings: []string{"skipped resurrected item 2026-09-30T11-07-44Z-x"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeUnder(t, dir, tc.files)
			scan, err := ScanDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(itemIDs(scan.Items), ","); got != strings.Join(tc.wantIDs, ",") {
				t.Errorf("items = %q, want %q", got, strings.Join(tc.wantIDs, ","))
			}
			if len(scan.Warnings) != btoi(len(tc.wantWarnings) > 0) {
				t.Fatalf("warnings = %v, want exactly one line", scan.Warnings)
			}
			for _, frag := range tc.wantWarnings {
				if !strings.Contains(scan.Warnings[0].Text, frag) {
					t.Errorf("warning %q lacks %q", scan.Warnings[0].Text, frag)
				}
			}
			if len(tc.wantWarnings) > 0 && scan.HasUnreadable() {
				t.Error("a resurrected skip is a notice, not an unreadable file")
			}
		})
	}
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

func TestScanDir_ResurrectedFileIsNeverDeleted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeUnder(t, dir, map[string]string{
		"a.json":          `{"id":"x"}`,
		"consumed/x.json": `{"id":"x","consumed":{"at":"A","via":"ship"}}`,
	})
	if _, err := ScanDir(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.json")); err != nil {
		t.Errorf("the operator removes the file, the loader never does: %v", err)
	}
}

func TestScanDir_SixteenConsumedRecordsDoNotHideALaterFiling(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	files := map[string]string{"later.json": `{"id":"pipeline-blocker","created_at":"2026-10-17T00:00:00Z"}`}
	for i := 0; i < 16; i++ {
		ts := fmt.Sprintf("2026-09-%02dT00:00:00Z", i+1)
		files[fmt.Sprintf("consumed/%02d.json", i)] = `{"id":"pipeline-blocker","created_at":"` + ts + `","consumed":{"at":"A","via":"ship"}}`
	}
	writeUnder(t, dir, files)
	scan, err := ScanDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := itemIDs(scan.Items); len(got) != 1 || len(scan.Warnings) != 0 {
		t.Errorf("items = %v, warnings = %v", got, scan.Warnings)
	}
}

func TestConsumedRecords_KeepsEveryRecordPerIDInSortedPathOrder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeUnder(t, dir, map[string]string{
		"consumed/b.json":   `{"id":"k","created_at":"T2","consumed":{"at":"B","via":"ship"}}`,
		"consumed/a.json":   `{"id":"k","created_at":"T1","consumed":{"at":"A","via":"console-manual"}}`,
		"consumed/bad.json": `{not json`,
	})
	if err := os.Symlink(filepath.Join(dir, "missing-target"), filepath.Join(dir, "consumed", "dangling.json")); err != nil {
		t.Fatal(err)
	}
	got := ConsumedRecords(dir)["k"]
	want := []ConsumedRecord{
		{ID: "k", At: "A", Via: "console-manual", CreatedAt: "T1", File: "a.json"},
		{ID: "k", At: "B", Via: "ship", CreatedAt: "T2", File: "b.json"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("records = %+v", got)
	}
	if len(ConsumedRecords(t.TempDir())) != 0 {
		t.Error("no consumed directory means no records")
	}
}

func TestCopyOf_MatchesOnCreatedAtElseOnFileName(t *testing.T) {
	t.Parallel()
	recs := []ConsumedRecord{{File: "a.json", CreatedAt: "T1"}, {File: "b.json"}}
	cases := []struct {
		createdAt, file string
		want            bool
	}{
		{"T1", "z.json", true},
		{"T2", "a.json", false},
		{"T2", "b.json", true},
		{"", "a.json", true},
		{"", "z.json", false},
	}
	for _, c := range cases {
		if _, got := CopyOf(recs, c.createdAt, c.file); got != c.want {
			t.Errorf("CopyOf(%q,%q) = %v, want %v", c.createdAt, c.file, got, c.want)
		}
	}
}
