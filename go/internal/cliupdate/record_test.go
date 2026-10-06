package cliupdate_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliupdate"
)

var boundary = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

func remember(t *testing.T, evolveDir string, results ...cliupdate.Result) {
	t.Helper()
	if err := cliupdate.Remember(evolveDir, cliupdate.Report{Results: results}, boundary); err != nil {
		t.Fatalf("Remember: %v", err)
	}
}

func loaded(t *testing.T, evolveDir string) []cliupdate.Record {
	t.Helper()
	got, err := cliupdate.LoadRecords(evolveDir)
	if err != nil {
		t.Fatalf("LoadRecords: %v", err)
	}
	return got
}

func TestRemember_RecordsTheVersionsTheBoundaryAccepted(t *testing.T) {
	evolveDir := t.TempDir()

	remember(t, evolveDir,
		cliupdate.Result{Family: "claude", Status: cliupdate.StatusUpdated, OldVersion: "2.1.285", NewVersion: "2.1.286"},
		cliupdate.Result{Family: "agy", Status: cliupdate.StatusUnchanged, OldVersion: "1.0.20", NewVersion: "1.0.20"},
		cliupdate.Result{Family: "ollama", Status: cliupdate.StatusSmokeFailed, OldVersion: "0.9.0", NewVersion: "0.9.1"},
		cliupdate.Result{Family: "codex", Status: cliupdate.StatusUpdateFailed, OldVersion: "0.153.4"},
	)

	want := []cliupdate.Record{
		{Family: "claude", Kind: cliupdate.KindUpdate, Old: "2.1.285", New: "2.1.286", At: boundary},
		{Family: "agy", Kind: cliupdate.KindBaseline, New: "1.0.20", At: boundary},
	}
	if got := loaded(t, evolveDir); !reflect.DeepEqual(got, want) {
		t.Errorf("records = %+v, want %+v (a smoke-failed change and an unknown new version are never accepted)", got, want)
	}
	if cliupdate.RecordsPath(evolveDir) != filepath.Join(evolveDir, cliupdate.RecordsFile) {
		t.Errorf("RecordsPath = %q", cliupdate.RecordsPath(evolveDir))
	}
}

func TestRemember_ASelfUpdateIsRecordedOnlyWhenItsSmokePassed(t *testing.T) {
	evolveDir := t.TempDir()

	remember(t, evolveDir,
		cliupdate.Result{Family: "agy", Status: cliupdate.StatusSelfUpdated, SelfUpdatedFrom: "1.2.16", OldVersion: "1.2.17", NewVersion: "1.2.17"},
		cliupdate.Result{Family: "claude", Status: cliupdate.StatusSmokeFailed, SelfUpdatedFrom: "2.1.285", OldVersion: "2.1.286"},
	)

	want := []cliupdate.Record{{Family: "agy", Kind: cliupdate.KindSelfUpdate, Old: "1.2.16", New: "1.2.17", At: boundary}}
	if got := loaded(t, evolveDir); !reflect.DeepEqual(got, want) {
		t.Errorf("records = %+v, want %+v (claude's unproven change stays unrecorded, so the next boundary smoke-boots it again)", got, want)
	}
}

func TestRemember_AKnownFamilyThatDidNotChangeAddsNoRecord(t *testing.T) {
	evolveDir := t.TempDir()
	unchanged := cliupdate.Result{Family: "claude", Status: cliupdate.StatusUnchanged, OldVersion: "2.1.285", NewVersion: "2.1.285"}
	remember(t, evolveDir, unchanged)

	remember(t, evolveDir, unchanged)

	if got := loaded(t, evolveDir); len(got) != 1 || got[0].Kind != cliupdate.KindBaseline {
		t.Errorf("the second boundary saw nothing new; want the one baseline, got %+v", got)
	}
}

func TestRemember_AReportThatObservedNoVersionWritesNoFile(t *testing.T) {
	evolveDir := filepath.Join(t.TempDir(), ".evolve")

	remember(t, evolveDir, cliupdate.Plan([]cliupdate.Family{{Name: "claude", UpdateArgv: []string{"claude", "update"}}}).Results...)

	if _, err := os.Stat(evolveDir); !os.IsNotExist(err) {
		t.Errorf("a plan observes no version, so nothing is written: stat err=%v", err)
	}
}

func TestRemember_AppendsAcrossBoundariesAndKeepsOnlyTheNewest(t *testing.T) {
	evolveDir := t.TempDir()
	for i := 0; i < 70; i++ {
		rep := cliupdate.Report{Results: []cliupdate.Result{{Family: "claude", Status: cliupdate.StatusUpdated, OldVersion: fmt.Sprint(i), NewVersion: fmt.Sprint(i + 1)}}}
		if err := cliupdate.Remember(evolveDir, rep, boundary.Add(time.Duration(i)*time.Hour)); err != nil {
			t.Fatalf("Remember %d: %v", i, err)
		}
	}

	got := loaded(t, evolveDir)
	if len(got) != 64 || got[0].Old != "6" || got[63].New != "70" {
		t.Errorf("want the newest 64 records (6→7 … 69→70), got %d from %+v to %+v", len(got), got[0], got[len(got)-1])
	}
}

func TestRemember_AnUnreadableRecordFileIsAnErrorAndStaysUntouched(t *testing.T) {
	evolveDir := t.TempDir()
	path := cliupdate.RecordsPath(evolveDir)
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep := cliupdate.Report{Results: []cliupdate.Result{{Family: "claude", Status: cliupdate.StatusUpdated, OldVersion: "1", NewVersion: "2"}}}

	if err := cliupdate.Remember(evolveDir, rep, boundary); err == nil {
		t.Fatal("an unreadable record file must fail loudly")
	}
	if body, _ := os.ReadFile(path); string(body) != "{not json" {
		t.Errorf("the unreadable file was rewritten: %q", body)
	}
}

func TestRemember_AnUncreatableEvolveDirIsAnError(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	rep := cliupdate.Report{Results: []cliupdate.Result{{Family: "claude", Status: cliupdate.StatusUpdated, OldVersion: "1", NewVersion: "2"}}}

	if err := cliupdate.Remember(filepath.Join(blocker, ".evolve"), rep, boundary); err == nil {
		t.Fatal("an evolve dir under a regular file cannot be created; Remember must say so")
	}
}

func TestLoadRecords_AnAbsentFileIsNoRecords(t *testing.T) {
	got, err := cliupdate.LoadRecords(t.TempDir())
	if err != nil || len(got) != 0 {
		t.Fatalf("want no records and no error, got %+v, %v", got, err)
	}
}

func TestLoadRecords_AnUnreadablePathIsAnError(t *testing.T) {
	evolveDir := t.TempDir()
	if err := os.Mkdir(cliupdate.RecordsPath(evolveDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := cliupdate.LoadRecords(evolveDir); err == nil {
		t.Fatal("a directory where the record file belongs is an error")
	}
}

func TestCauseOf_NamesWhatMovedAVersionBetweenTwoBaselines(t *testing.T) {
	records := []cliupdate.Record{
		{Family: "claude", Kind: cliupdate.KindBaseline, New: "2.1.285"},
		{Family: "claude", Kind: cliupdate.KindUpdate, Old: "2.1.285", New: "2.1.286"},
		{Family: "agy", Kind: cliupdate.KindSelfUpdate, Old: "1.2.16", New: "1.2.17"},
		{Family: "claude", Kind: cliupdate.KindUpdate, Old: "2.1.286", New: "2.1.287"},
		{Family: "agy", Kind: cliupdate.KindUpdate, Old: "1.2.17", New: "1.2.18"},
	}
	cases := []struct {
		family, from, to string
		want             cliupdate.Cause
		why              string
	}{
		{"claude", "2.1.285", "2.1.287", cliupdate.CauseBoundaryUpdate, "two recorded boundary updates"},
		{"claude", "2.1.286", "2.1.287", cliupdate.CauseBoundaryUpdate, "the baseline was taken between the two updates"},
		{"agy", "1.2.16", "1.2.17", cliupdate.CauseUnrecorded, "a recorded update moved agy past 1.2.17, so landing back on it was not the boundary"},
		{"agy", "1.2.16", "1.2.18", cliupdate.CauseSelfUpdate, "a chain with a self-update in it"},
		{"agy", "1.2.17", "1.2.18", cliupdate.CauseBoundaryUpdate, "only the boundary update after the self-update"},
		{"claude", "2.1.285", "2.1.286", cliupdate.CauseUnrecorded, "a recorded update moved claude past 2.1.286, so landing back on it was not the boundary"},
		{"claude", "2.1.285", "2.1.288", cliupdate.CauseUnrecorded, "no record reaches 2.1.288"},
		{"claude", "2.1.284", "2.1.286", cliupdate.CauseUnrecorded, "no record starts at 2.1.284"},
		{"claude", "2.1.286", "2.1.286", cliupdate.CauseUnrecorded, "no change needs no cause"},
	}
	for _, tc := range cases {
		if got := cliupdate.CauseOf(records, tc.family, tc.from, tc.to); got != tc.want {
			t.Errorf("CauseOf(%s %s → %s) = %q, want %q: %s", tc.family, tc.from, tc.to, got, tc.want, tc.why)
		}
	}
}

func TestCauseOf_AnUnrecordedHopBetweenBoundaryUpdatesIsUnrecorded(t *testing.T) {
	records := []cliupdate.Record{
		{Family: "claude", Kind: cliupdate.KindUpdate, Old: "2.1.285", New: "2.1.286"},
		{Family: "claude", Kind: cliupdate.KindUpdate, Old: "2.1.287", New: "2.1.288"},
	}
	if got := cliupdate.CauseOf(records, "claude", "2.1.285", "2.1.288"); got != cliupdate.CauseUnrecorded {
		t.Fatalf("2.1.286 → 2.1.287 happened between boundaries and no boundary saw it; got %q", got)
	}
}

func TestRecordKind_SpellsTheOnDiskVocabularyTheDocsName(t *testing.T) {
	for kind, want := range map[cliupdate.RecordKind]string{
		cliupdate.KindBaseline:   `"kind":"baseline"`,
		cliupdate.KindUpdate:     `"kind":"update"`,
		cliupdate.KindSelfUpdate: `"kind":"self-update"`,
	} {
		body, err := json.Marshal(cliupdate.Record{Family: "agy", Kind: kind, New: "1.2.17"})
		if err != nil || !strings.Contains(string(body), want) {
			t.Errorf("a %s record must persist as %s; got %s (%v)", kind, want, body, err)
		}
	}
}

func TestRemember_ASkippedFamilyIsNeverRecorded(t *testing.T) {
	evolveDir := t.TempDir()

	remember(t, evolveDir,
		cliupdate.Result{Family: "agy", Status: cliupdate.StatusSkipped, SelfUpdatedFrom: "1.2.16", OldVersion: "1.2.17", Detail: "interrupted: context canceled"},
		cliupdate.Result{Family: "claude", Status: cliupdate.StatusSkipped, OldVersion: "2.1.285", Detail: "doctor live did not answer"},
	)

	if got := loaded(t, evolveDir); len(got) != 0 {
		t.Errorf("an interrupted smoke or an unsubscribed family proves nothing, so nothing is accepted: %+v", got)
	}
}
