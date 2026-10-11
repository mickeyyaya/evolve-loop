package attemptpostmortem

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAttemptAbnormal_IsTrueForACauseOrANonZeroExit(t *testing.T) {
	cases := []struct {
		cause string
		exit  int
		want  bool
	}{{"", 0, false}, {"pane_lost", 0, true}, {"", 81, true}, {"review_pause", 81, true}}
	for _, tc := range cases {
		if got := (Attempt{CauseCode: tc.cause, ExitCode: tc.exit}).Abnormal(); got != tc.want {
			t.Errorf("Abnormal(%q, %d) = %v, want %v", tc.cause, tc.exit, got, tc.want)
		}
	}
}

func TestPath_IsThePhaseAttemptFileInTheRunWorkspace(t *testing.T) {
	if got, want := Path("/run/cycle-1853", "build", 2), filepath.Join("/run/cycle-1853", "build-attempt-2-postmortem.json"); got != want {
		t.Fatalf("Path = %q, want %q", got, want)
	}
}

func validRecord(t *testing.T) Record {
	t.Helper()
	return collectAttempt1(t, DefaultConfig())
}

func TestRecordValidate_RefusesEachBrokenField(t *testing.T) {
	cases := map[string]func(*Record){
		"schema":         func(r *Record) { r.Schema = "attempt-postmortem/0.9" },
		"phase":          func(r *Record) { r.Phase = "" },
		"cycle":          func(r *Record) { r.Cycle = 0 },
		"attempt":        func(r *Record) { r.Number = 0 },
		"ended_at":       func(r *Record) { r.EndedAt = r.StartedAt.Add(-time.Second) },
		"command_source": func(r *Record) { r.CommandSource = Source("guess") },
		"suspect":        func(r *Record) { r.Suspect.Reason = "hunch" },
	}
	if err := validRecord(t).Validate(); err != nil {
		t.Fatalf("a collected record: Validate() = %v, want nil", err)
	}
	for field, mutate := range cases {
		t.Run(field, func(t *testing.T) {
			r := validRecord(t)
			mutate(&r)
			err := r.Validate()
			if err == nil || !strings.Contains(err.Error(), field) {
				t.Fatalf("Validate() = %v, want an error that names %s", err, field)
			}
		})
	}
}

func TestWriteReadAll_RoundTripsTheRecordsInAttemptOrder(t *testing.T) {
	dir := t.TempDir()
	first := validRecord(t)
	first.Number = 2
	second := validRecord(t)
	second.Number = 10
	second.Suspect = nil
	for _, r := range []Record{second, first} {
		if _, err := Write(dir, r); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	writeFile(t, dir, "tdd-attempt-1-postmortem.json", "{}")
	writeFile(t, dir, "build-attempt-3-postmortem.json.tmp", "{}")
	got, bad, err := ReadAll(dir, "build")
	if err != nil || len(bad) != 0 {
		t.Fatalf("ReadAll: %v, bad records %v", err, bad)
	}
	if len(got) != 2 || got[0].Number != 2 || got[1].Number != 10 {
		t.Fatalf("ReadAll = %d records, want attempts 2 and 10 in number order", len(got))
	}
	if got[0].Suspect == nil || got[0].Suspect.Command.Text != first.Suspect.Command.Text || !got[0].EndedAt.Equal(first.EndedAt) {
		t.Fatalf("round trip lost data: %+v", got[0])
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".*.tmp")); len(left) != 0 {
		t.Fatalf("temp files stayed: %v", left)
	}
	raw, err := os.ReadFile(Path(dir, "build", 2))
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"schema", "phase", "cycle", "attempt", "cli", "session", "dispatch_id", "started_at", "ended_at", "cause_code", "exit_code", "last_activity_at", "command_source", "commands", "suspect", "pane_tail", "worktree_delta", "evidence_paths"} {
		if _, ok := wire[key]; !ok {
			t.Errorf("the record has no %q key", key)
		}
	}
}

func TestReadAll_AnEmptyWorkspaceHasNoRecords(t *testing.T) {
	got, bad, err := ReadAll(t.TempDir(), "build")
	if err != nil || len(got) != 0 || len(bad) != 0 {
		t.Fatalf("ReadAll = %v, %v; want no records and no error", got, err)
	}
}

func TestWrite_RefusesAnInvalidRecordAndReportsEachWriteFailure(t *testing.T) {
	dir := t.TempDir()
	bad := validRecord(t)
	bad.Phase = ""
	if _, err := Write(dir, bad); err == nil {
		t.Error("an invalid record: err = nil")
	}
	far := validRecord(t)
	far.EndedAt = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := Write(dir, far); err == nil {
		t.Error("a time that JSON cannot encode: err = nil")
	}
	if err := os.Mkdir(Path(dir, "build", 1), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(filepath.Join(dir, "absent", "run"), validRecord(t)); err != nil {
		t.Errorf("a workspace that does not exist yet: err = %v, want the directory made", err)
	}
	if _, err := Write(writeFile(t, dir, "a-file", "x"), validRecord(t)); err == nil {
		t.Error("a workspace that is a file: err = nil")
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if _, err := Write(dir, validRecord(t)); err == nil {
		t.Error("a read-only workspace: err = nil")
	}
}

func TestReadAll_ReportsABadPattern(t *testing.T) {
	if _, _, err := ReadAll(filepath.Join(t.TempDir(), "["), "build"); err == nil {
		t.Error("a bad glob pattern: err = nil")
	}
	if _, _, err := ReadAll(t.TempDir(), "../build"); err == nil {
		t.Error("a phase that is not a bare name: err = nil")
	}
}

func TestReadAll_SkipsOnlyTheBadRecordAndNamesItsPath(t *testing.T) {
	cases := map[string]func(t *testing.T, dir string){
		"unreadable": func(t *testing.T, dir string) {
			if err := os.Mkdir(filepath.Join(dir, "build-attempt-1-postmortem.json"), 0o755); err != nil {
				t.Fatal(err)
			}
		},
		"bad json": func(t *testing.T, dir string) { writeFile(t, dir, "build-attempt-1-postmortem.json", "{") },
		"invalid":  func(t *testing.T, dir string) { writeFile(t, dir, "build-attempt-1-postmortem.json", `{"schema":"x"}`) },
	}
	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			arrange(t, dir)
			good := validRecord(t)
			good.Number = 2
			if _, err := Write(dir, good); err != nil {
				t.Fatal(err)
			}

			got, bad, err := ReadAll(dir, "build")

			if err != nil {
				t.Fatalf("ReadAll err = %v, want nil: one bad record must not drop the section", err)
			}
			if len(got) != 1 || got[0].Number != 2 {
				t.Fatalf("records = %+v, want only the valid record 2", got)
			}
			wantPath := Path(dir, "build", 1)
			if len(bad) != 1 || bad[0].Path != wantPath || !strings.Contains(bad[0].Error(), "build-attempt-1-postmortem.json") {
				t.Fatalf("bad = %+v, want one bad record at %s", bad, wantPath)
			}
			var asError error = bad[0]
			var asBad BadRecord
			if !errors.As(asError, &asBad) || asBad.Path != wantPath || asBad.Err == nil {
				t.Fatalf("a BadRecord as an error = %v, want it to keep its path and its cause", asError)
			}
		})
	}
}

func TestWrite_TakesTheNextFreeNumberWhenTheRecordExists(t *testing.T) {
	dir := t.TempDir()
	first := validRecord(t)
	if _, err := Write(dir, first); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "build-attempt-2-postmortem.json", "claimed by another dispatch")

	got, err := Write(dir, first)

	if err != nil || got.Number != 3 {
		t.Fatalf("Write = record %d, %v; want record 3 after the taken 1 and 2", got.Number, err)
	}
	raw, err := os.ReadFile(Path(dir, "build", 2))
	if err != nil || string(raw) != "claimed by another dispatch" {
		t.Fatalf("record 2 = %q, %v; want it unchanged", raw, err)
	}
	records, _, err := ReadAll(dir, "build")
	if err != nil || len(records) != 2 || records[1].Number != 3 {
		t.Fatalf("records = %+v, %v; want records 1 and 3", records, err)
	}
}

func TestCreate_RefusesAnExistingRecordAndReplaceOverwritesIt(t *testing.T) {
	dir := t.TempDir()
	r := validRecord(t)
	if err := Create(dir, r); err != nil {
		t.Fatalf("Create: %v", err)
	}
	changed := r
	changed.CauseCode = "pane_lost"

	err := Create(dir, changed)

	if !errors.Is(err, fs.ErrExist) || !strings.Contains(err.Error(), "build-attempt-1-postmortem.json") {
		t.Fatalf("Create over a record = %v, want fs.ErrExist that names the file", err)
	}
	if got := readOne(t, dir); got.CauseCode != "review_pause" {
		t.Fatalf("cause = %q, want the first record unchanged", got.CauseCode)
	}
	if err := Replace(dir, changed); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if got := readOne(t, dir); got.CauseCode != "pane_lost" {
		t.Fatalf("cause = %q, want the replaced record", got.CauseCode)
	}
	bad := r
	bad.Phase = ""
	if Replace(dir, bad) == nil || Create(dir, bad) == nil {
		t.Fatal("an invalid record: want a Validate error from Create and Replace")
	}
	if err := Replace(writeFile(t, dir, "a-file", "x"), r); err == nil {
		t.Fatal("Replace into a workspace that is a file: err = nil")
	}
}

func TestCreate_AFailedWriteLeavesNoClaimedFile(t *testing.T) {
	dir := t.TempDir()
	r := validRecord(t)
	r.Number = 4
	failing := errors.New("disk full")
	prev := writeRecordBody
	writeRecordBody = func(*os.File, []byte) error { return failing }
	t.Cleanup(func() { writeRecordBody = prev })

	err := Create(dir, r)

	if !errors.Is(err, failing) {
		t.Fatalf("Create = %v, want the write error", err)
	}
	if _, statErr := os.Stat(Path(dir, "build", 4)); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("stat = %v, want no record file", statErr)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".*")); len(left) != 0 {
		t.Fatalf("temp files stayed: %v", left)
	}
}

func readOne(t *testing.T, dir string) Record {
	t.Helper()
	records, bad, err := ReadAll(dir, "build")
	if err != nil || len(bad) != 0 || len(records) != 1 {
		t.Fatalf("ReadAll = %+v, %v, %v; want one record", records, bad, err)
	}
	return records[0]
}

func TestCreate_TheRecordPathAppearsOnlyWithItsWholeBody(t *testing.T) {
	dir := t.TempDir()
	r := validRecord(t)
	target := Path(dir, "build", 1)
	prev := writeRecordBody
	var seenDuringWrite error
	writeRecordBody = func(f *os.File, data []byte) error {
		_, seenDuringWrite = os.Stat(target)
		return prev(f, data)
	}
	t.Cleanup(func() { writeRecordBody = prev })

	if err := Create(dir, r); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if !errors.Is(seenDuringWrite, fs.ErrNotExist) {
		t.Fatalf("stat of the record path during the write = %v, want not-exist: a crash must not leave a partial record", seenDuringWrite)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".*")); len(left) != 0 {
		t.Fatalf("temp files stayed: %v", left)
	}
	if got := readOne(t, dir); got.Number != 1 {
		t.Fatalf("record = %+v, want record 1", got)
	}
}
