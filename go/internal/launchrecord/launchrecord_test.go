package launchrecord

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if got != want {
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
	if got, _ := Read(dir, first.RunID); got != first {
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
