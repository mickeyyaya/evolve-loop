package attemptpostmortem

import (
	"encoding/json"
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
		if err := Write(dir, r); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	writeFile(t, dir, "tdd-attempt-1-postmortem.json", "{}")
	writeFile(t, dir, "build-attempt-3-postmortem.json.tmp", "{}")
	got, err := ReadAll(dir, "build")
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
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
	got, err := ReadAll(t.TempDir(), "build")
	if err != nil || len(got) != 0 {
		t.Fatalf("ReadAll = %v, %v; want no records and no error", got, err)
	}
}

func TestWrite_RefusesAnInvalidRecordAndReportsEachWriteFailure(t *testing.T) {
	dir := t.TempDir()
	bad := validRecord(t)
	bad.Phase = ""
	if err := Write(dir, bad); err == nil {
		t.Error("an invalid record: err = nil")
	}
	far := validRecord(t)
	far.EndedAt = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := Write(dir, far); err == nil {
		t.Error("a time that JSON cannot encode: err = nil")
	}
	if err := os.Mkdir(Path(dir, "build", 1), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Write(dir, validRecord(t)); err == nil {
		t.Error("a directory at the record path: err = nil")
	}
}

func TestReadAll_ReportsABadPatternAnUnreadableFileBadJSONAndAnInvalidRecord(t *testing.T) {
	if _, err := ReadAll(filepath.Join(t.TempDir(), "["), "build"); err == nil {
		t.Error("a bad glob pattern: err = nil")
	}
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
			if _, err := ReadAll(dir, "build"); err == nil || !strings.Contains(err.Error(), "build-attempt-1-postmortem.json") {
				t.Fatalf("ReadAll = %v, want an error that names the file", err)
			}
		})
	}
}
