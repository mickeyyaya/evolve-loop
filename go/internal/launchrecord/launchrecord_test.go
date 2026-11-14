package launchrecord

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cloudlaunch"
)

func sample() Record {
	return Record{
		RunID: "agent-go-reviewer-20261027T090000Z-00ab", Profile: "go-reviewer", CLI: "agy-tmux", Family: "agy",
		Model: "gemini-3-pro", Verdict: "PASS", TreeBefore: "abc", TreeAfter: "abc", ArtifactSHA256: "f00", TS: "2026-10-27T09:00:00Z",
	}
}

func TestWriteThenRead_ReturnsTheSameRecord(t *testing.T) {
	dir := t.TempDir()
	want := sample()
	path, err := Write(dir, want)
	if err != nil {
		t.Fatalf("Write = %v", err)
	}
	if path != filepath.Join(dir, Subdir, want.RunID+".json") {
		t.Errorf("Write path = %q, want the record file of the run", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the record file is missing: %v", err)
	}
	got, err := Read(dir, want.RunID)
	if err != nil {
		t.Fatalf("Read = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Read = %+v, want %+v", got, want)
	}
}

func TestRead_AMissingRecordIsNotExist(t *testing.T) {
	_, err := Read(t.TempDir(), "agent-x-1")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Read of a missing record = %v, want fs.ErrNotExist", err)
	}
}

func TestRead_ARecordThatIsNotJSONFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, Subdir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, Subdir, "agent-x-1.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(dir, "agent-x-1"); err == nil || !strings.Contains(err.Error(), "agent-x-1") {
		t.Errorf("Read of a bad record = %v, want an error that names the run", err)
	}
}

func TestRead_ARecordOfAnotherRunIDFails(t *testing.T) {
	dir := t.TempDir()
	r := sample()
	if _, err := Write(dir, r); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dir, Subdir, r.RunID+".json"), filepath.Join(dir, Subdir, "agent-other-1.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(dir, "agent-other-1"); err == nil || !strings.Contains(err.Error(), r.RunID) {
		t.Errorf("Read of a renamed record = %v, want an error that names the recorded run id", err)
	}
}

func TestPath_RefusesARunIDThatIsNotOneName(t *testing.T) {
	for _, id := range []string{"", "..", "a/b", `a\b`, "../x", "a b", ".hidden"} {
		if _, err := Path(t.TempDir(), id); err == nil {
			t.Errorf("Path(%q) = nil error, want a refusal", id)
		}
		if _, err := Write(t.TempDir(), Record{RunID: id}); err == nil {
			t.Errorf("Write(%q) = nil error, want a refusal", id)
		}
		if _, err := Read(t.TempDir(), id); err == nil {
			t.Errorf("Read(%q) = nil error, want a refusal", id)
		}
	}
}

func TestBoundTo_HoldsOnlyWhenTheTreeDidNotChangeAndMatches(t *testing.T) {
	cases := []struct {
		name          string
		before, after string
		current       string
		want          bool
	}{
		{"same tree", "abc", "abc", "abc", true},
		{"the tree changed during the run", "abc", "abd", "abd", false},
		{"another tree now", "abc", "abc", "xyz", false},
		{"no tree recorded", "", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := Record{TreeBefore: tc.before, TreeAfter: tc.after}
			if got := r.BoundTo(tc.current); got != tc.want {
				t.Errorf("BoundTo(%q) = %v, want %v", tc.current, got, tc.want)
			}
		})
	}
}

func TestWrite_RefusesToReplaceARecord(t *testing.T) {
	dir := t.TempDir()
	first := sample()
	if _, err := Write(dir, first); err != nil {
		t.Fatal(err)
	}
	second := first
	second.TreeAfter = "another"
	if _, err := Write(dir, second); !errors.Is(err, fs.ErrExist) {
		t.Errorf("a second Write = %v, want fs.ErrExist", err)
	}
	if got, _ := Read(dir, first.RunID); !reflect.DeepEqual(got, first) {
		t.Errorf("the record changed to %+v", got)
	}
}

func TestWrite_ADirectoryThatCannotBeMadeFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, Subdir), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(dir, sample()); err == nil {
		t.Error("Write under a file = nil error, want a refusal")
	}
}

func TestWriteThenRead_KeepsTheCloudSession(t *testing.T) {
	dir := t.TempDir()
	want := sample()
	want.Cloud = &cloudlaunch.Session{Schema: cloudlaunch.Schema, Outcome: cloudlaunch.OutcomeLaunched, ID: "session_01", URL: "https://claude.ai/code/session_01", Title: "t", Repo: "acme/w", Branch: "main", CLI: "claude-cloud", TS: "2026-11-03T10:00:00Z"}
	if _, err := Write(dir, want); err != nil {
		t.Fatal(err)
	}
	got, err := Read(dir, want.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Cloud == nil || *got.Cloud != *want.Cloud {
		t.Errorf("Read Cloud = %+v, want %+v", got.Cloud, want.Cloud)
	}
}

func TestList_ReturnsEachRecordInTimeOrder(t *testing.T) {
	dir := t.TempDir()
	late, early := sample(), sample()
	late.RunID, late.TS = "agent-a-2", "2026-11-03T11:00:00Z"
	early.RunID, early.TS = "agent-z-1", "2026-11-03T09:00:00Z"
	for _, r := range []Record{late, early} {
		if _, err := Write(dir, r); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, Subdir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, Subdir, "dir.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := List(dir)
	if err != nil {
		t.Fatalf("List = %v", err)
	}
	if len(got) != 2 || got[0].RunID != early.RunID || got[1].RunID != late.RunID {
		t.Errorf("List = %+v, want the early then the late record", got)
	}
}

func TestList_NoRecordDirectoryIsAnEmptyList(t *testing.T) {
	got, err := List(t.TempDir())
	if err != nil || len(got) != 0 {
		t.Errorf("List of no directory = %v, %v; want an empty list", got, err)
	}
}

func TestList_FailsOnABadRecordOrAnUnreadableDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, Subdir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, Subdir, "agent-x-1.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := List(dir); err == nil || !strings.Contains(err.Error(), "agent-x-1") {
		t.Errorf("List with a bad record = %v, want an error that names the run", err)
	}
	file := t.TempDir()
	if err := os.WriteFile(filepath.Join(file, Subdir), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := List(file); err == nil {
		t.Error("List of a file in place of the directory = nil error, want a failure")
	}
}

func TestDeliverablePath_SitsNextToTheRecordAndRefusesABadRunID(t *testing.T) {
	got, err := DeliverablePath("/e", "run-1")
	if err != nil || got != filepath.Join("/e", Subdir, "run-1.deliverable.md") {
		t.Fatalf("DeliverablePath = %q, %v", got, err)
	}
	if _, err := DeliverablePath("/e", "../x"); err == nil {
		t.Error("a run id with a separator must be refused")
	}
	if err := KeepDeliverable("/e", "../x", "b", 1); err == nil {
		t.Error("KeepDeliverable must refuse a bad run id")
	}
}

func TestKeepDeliverable_WritesAtomicallyAndKeepsTheNewestAndNeverTouchesRecords(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, Subdir)
	if err := os.MkdirAll(filepath.Join(sub, "dir.deliverable.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	base := time.Now().Add(-time.Hour)
	for i, name := range []string{"a.deliverable.md", "b.deliverable.md", "c.deliverable.md", "c.json"} {
		path := filepath.Join(sub, name)
		if err := os.WriteFile(path, []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
		at := base.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(path, at, at); err != nil {
			t.Fatal(err)
		}
	}

	if err := KeepDeliverable(dir, "new", "body", 2); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(sub, "new.deliverable.md"))
	if err != nil || string(got) != "body" {
		t.Fatalf("kept = %q, %v", got, err)
	}
	for name, want := range map[string]bool{"a.deliverable.md": false, "b.deliverable.md": false, "c.deliverable.md": true, "c.json": true} {
		if _, err := os.Stat(filepath.Join(sub, name)); (err == nil) != want {
			t.Errorf("%s present = %v, want %v", name, err == nil, want)
		}
	}
	if left, _ := filepath.Glob(filepath.Join(sub, "*.tmp")); len(left) != 0 {
		t.Errorf("temp files left: %v", left)
	}
}

func TestKeepDeliverable_TiesBreakOnNameAndFailuresAreReported(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, Subdir)
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(-time.Hour)
	for _, name := range []string{"a.deliverable.md", "b.deliverable.md"} {
		path := filepath.Join(sub, name)
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, at, at); err != nil {
			t.Fatal(err)
		}
	}
	if err := KeepDeliverable(dir, "n", "b", 2); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(sub, "a.deliverable.md")); err == nil {
		t.Error("the lower name of an equal pair must be pruned first")
	}

	blocked := t.TempDir()
	if err := os.WriteFile(filepath.Join(blocked, Subdir), []byte("f"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := KeepDeliverable(blocked, "n", "b", 1); err == nil {
		t.Error("a blocked directory must be reported")
	}

	ro := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ro, Subdir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(ro, Subdir), 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(ro, Subdir), 0o755) })
	if err := KeepDeliverable(ro, "n", "b", 1); err == nil {
		t.Error("an unwritable directory must be reported")
	}
}

func TestPruneDeliverables_ReportsAMissingDirectory(t *testing.T) {
	if err := pruneDeliverables(filepath.Join(t.TempDir(), "none"), 1); err == nil {
		t.Error("a missing directory must be reported")
	}
}

func TestPruneDeliverables_ReportsAStaleFileThatCannotBeRemoved(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.deliverable.md", "b.deliverable.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	err := pruneDeliverables(dir, 1)

	if err == nil {
		t.Error("a stale file that cannot be removed must be reported")
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "*.deliverable.md")); len(left) != 2 {
		t.Fatalf("the fixture must keep both files, got %v", left)
	}
}

type brokenEntry struct{ fs.DirEntry }

func (brokenEntry) Name() string               { return "x.deliverable.md" }
func (brokenEntry) IsDir() bool                { return false }
func (brokenEntry) Info() (fs.FileInfo, error) { return nil, errors.New("gone") }

func TestStaleDeliverables_ReportsAnEntryThatCannotBeStatted(t *testing.T) {
	if _, err := staleDeliverables([]fs.DirEntry{brokenEntry{}}, 1); err == nil {
		t.Error("an entry without info must be reported")
	}
}

func TestKeepDeliverable_ARenameThatFailsIsReportedAndLeavesNoStagingFile(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, Subdir)
	if err := os.MkdirAll(filepath.Join(sub, "n.deliverable.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := KeepDeliverable(dir, "n", "b", 1); err == nil {
		t.Fatal("a rename onto a directory must be reported")
	}
	if _, err := os.Stat(filepath.Join(sub, "n.deliverable.md.tmp")); err == nil {
		t.Error("the staging file must be removed")
	}
}

func TestViolation_RoundTripsInARecordAndIsOmittedWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	rec := sample()
	rec.Violations = []Violation{{Code: "missing_section", Message: `required section "## Simplifications" is missing`}}
	if _, err := Write(dir, rec); err != nil {
		t.Fatal(err)
	}
	got, err := Read(dir, rec.RunID)
	if err != nil || !reflect.DeepEqual(got.Violations, rec.Violations) {
		t.Fatalf("Violations = %+v, %v; want %+v", got.Violations, err, rec.Violations)
	}
	raw, _ := json.Marshal(sample())
	if strings.Contains(string(raw), "violations") {
		t.Errorf("an empty Violations must be omitted: %s", raw)
	}
}
