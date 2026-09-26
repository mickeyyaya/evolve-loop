package lifecycle

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReadFailureCount_RootAndProcessing_NotQuarantine(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, filepath.Join(inbox, "a.json"), `{"id":"a","failure_count":2}`)
	writeItem(t, procPath(inbox, 3, "b.json"), `{"id":"b","failure_count":4}`)
	writeItem(t, procPath(inbox, 3, "c.json"), `{"id":"c"}`)
	writeItem(t, filepath.Join(inbox, "quarantine", "q.json"), `{"id":"q","failure_count":9}`)
	writeItem(t, filepath.Join(inbox, "retry", "r.json"), `{"id":"r","failure_count":9}`)
	m := New(inbox, nil)
	for id, want := range map[string]struct {
		n  int
		ok bool
	}{"a": {2, true}, "b": {4, true}, "c": {0, true}, "q": {0, false}, "r": {0, false}, "ghost": {0, false}} {
		if n, ok := m.ReadFailureCount(id); n != want.n || ok != want.ok {
			t.Errorf("ReadFailureCount(%s) = %d, %v; want %d, %v", id, n, ok, want.n, want.ok)
		}
	}
	if n, ok := readFailureCountAt(filepath.Join(inbox, "missing.json")); n != 0 || ok {
		t.Error("a missing record reads as not-found")
	}
	writeItem(t, filepath.Join(inbox, "bad.json"), `{"id":"bad"`)
	if n, ok := readFailureCountAt(filepath.Join(inbox, "bad.json")); n != 0 || ok {
		t.Error("a malformed record reads as not-found")
	}
}

func TestFindFileByTaskID_ReadDirError_IgnoresNonJSON_SkipsMalformed(t *testing.T) {
	inbox := newInbox(t)
	if _, err := FindFileByTaskID(filepath.Join(inbox, "nope"), "x"); err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("a missing dir is the ReadDir error, not ErrNotFound: %v", err)
	}
	writeItem(t, filepath.Join(inbox, "readme.md"), `{"id":"target"}`)
	writeItem(t, filepath.Join(inbox, "bad.json"), `{not json`)
	mkdirAll(t, filepath.Join(inbox, "dir.json"))
	writeItem(t, filepath.Join(inbox, "good.json"), `{"id":"target"}`)
	got, err := FindFileByTaskID(inbox, "target")
	if err != nil || filepath.Base(got) != "good.json" {
		t.Errorf("got %q, %v", got, err)
	}
	if _, err := FindFileByTaskID(inbox, "absent"); err != ErrNotFound {
		t.Errorf("absent id: %v", err)
	}
}

func TestLocate_ProcessingOutranksRootAndAbsentIsNotFound(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, filepath.Join(inbox, "a.json"), `{"id":"a"}`)
	writeItem(t, procPath(inbox, 4, "a.json"), `{"id":"a"}`)
	writeItem(t, filepath.Join(inbox, "b.json"), `{"id":"b"}`)
	if loc, err := Locate(inbox, "a"); err != nil || loc.Cycle != 4 || loc.Path != procPath(inbox, 4, "a.json") {
		t.Errorf("processing first: %+v %v", loc, err)
	}
	if loc, err := Locate(inbox, "b"); err != nil || loc.Cycle != 0 || loc.Path != filepath.Join(inbox, "b.json") {
		t.Errorf("root: %+v %v", loc, err)
	}
	if _, err := Locate(inbox, "ghost"); !errors.Is(err, ErrNotFound) {
		t.Errorf("absent: %v", err)
	}
	if _, err := Locate(filepath.Join(inbox, "no-such-inbox"), "a"); !errors.Is(err, ErrNotFound) {
		t.Errorf("no inbox at all: %v", err)
	}
	file := filepath.Join(inbox, "not-a-dir")
	writeItem(t, file, "x")
	if _, err := Locate(file, "a"); err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("a read fault is returned as itself: %v", err)
	}
}

func TestJsonEntries_SkipsDirsAndNonJSON(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, filepath.Join(inbox, "b.json"), `{}`)
	writeItem(t, filepath.Join(inbox, "a.json"), `{}`)
	writeItem(t, filepath.Join(inbox, "notes.txt"), `x`)
	mkdirAll(t, filepath.Join(inbox, "sub.json"))
	entries, err := jsonEntries(inbox)
	if err != nil || len(entries) != 2 || entries[0].Name() != "a.json" || entries[1].Name() != "b.json" {
		t.Errorf("entries = %v, %v", entries, err)
	}
	if _, err := jsonEntries(filepath.Join(inbox, "missing")); err == nil {
		t.Error("a missing dir is the caller's error")
	}
}

func TestBumpWith_ShedsContinuationOnlyAtCeiling_OneRename(t *testing.T) {
	inbox := newInbox(t)
	item := filepath.Join(inbox, "a.json")
	writeItem(t, item, `{"id":"a","failure_count":"weird","continuation":{"cycle":1}}`)
	n, err := bumpWith(item, "r1", func(c int) bool { return c >= 2 })
	if err != nil || n != 1 {
		t.Fatalf("n = %d, err = %v (a non-numeric count reads as 0)", n, err)
	}
	if body, _ := os.ReadFile(item); string(body) != `{"continuation":{"cycle":1},"failure_count":1,"id":"a","last_failure_reason":"r1"}` {
		t.Errorf("below the ceiling: %s", body)
	}
	n, err = bumpWith(item, "r2", func(c int) bool { return c >= 2 })
	if err != nil || n != 2 {
		t.Fatalf("n = %d, err = %v", n, err)
	}
	if body, _ := os.ReadFile(item); string(body) != `{"failure_count":2,"id":"a","last_failure_reason":"r2"}` {
		t.Errorf("at the ceiling the continuation is shed in the SAME bytes: %s", body)
	}
	writeItem(t, item, `{"id":"a","continuation":{"cycle":1}}`)
	if n, err := BumpFailureCount(item, ""); err != nil || n != 1 {
		t.Fatalf("n = %d, err = %v", n, err)
	}
	if body, _ := os.ReadFile(item); string(body) != `{"continuation":{"cycle":1},"failure_count":1,"id":"a"}` {
		t.Errorf("BumpFailureCount never sheds and an empty reason stamps nothing: %s", body)
	}
	mkdirAll(t, tmpPathOf(item))
	if _, err := BumpFailureCount(item, "x"); err == nil {
		t.Error("a directory at the tmp path is the rewrite's error")
	}
	if body, _ := os.ReadFile(item); string(body) != `{"continuation":{"cycle":1},"failure_count":1,"id":"a"}` {
		t.Errorf("the item is untouched on a failed rewrite: %s", body)
	}
	if _, err := BumpFailureCount(filepath.Join(inbox, "missing.json"), "x"); err == nil {
		t.Error("a missing item is the read error")
	}
}

func TestUpdateItemJSON_ErrorArmsAndCommitTmp(t *testing.T) {
	inbox := newInbox(t)
	bad := filepath.Join(inbox, "bad.json")
	writeItem(t, bad, `[]`)
	if err := UpdateItemJSON(bad, func(map[string]json.RawMessage) {}); err == nil {
		t.Error("a JSON array body cannot be updated as a field map")
	}
	item := filepath.Join(inbox, "a.json")
	writeItem(t, item, `{"id":"a"}`)
	if err := UpdateItemJSON(item, func(m map[string]json.RawMessage) { m["x"] = json.RawMessage("{bad") }); err == nil {
		t.Error("an invalid raw message is the marshal error")
	}
	if body, _ := os.ReadFile(item); string(body) != `{"id":"a"}` {
		t.Errorf("untouched: %s", body)
	}
	if err := UpdateItemJSON(item, func(m map[string]json.RawMessage) { m["b"] = json.RawMessage(`true`) }); err != nil {
		t.Fatal(err)
	}
	if body, _ := os.ReadFile(item); string(body) != `{"b":true,"id":"a"}` {
		t.Errorf("sorted keys, no indent: %s", body)
	}
	tmp := filepath.Join(inbox, "x.tmp")
	writeItem(t, tmp, "x")
	dirAsPath := filepath.Join(inbox, "target-dir")
	mkdirAll(t, dirAsPath)
	if err := commitTmp(tmp, dirAsPath); err == nil {
		t.Error("renaming over a directory fails")
	}
	if _, statErr := os.Stat(tmp); !os.IsNotExist(statErr) {
		t.Error("the tmp file is removed after a failed commit")
	}
}

func TestReadTaskIDOrUnknown_Fallbacks(t *testing.T) {
	dir := t.TempDir()
	if got := readTaskIDOrUnknown(filepath.Join(dir, "missing.json")); got != "unknown" {
		t.Errorf("missing file: got %q, want unknown", got)
	}
	writeItem(t, filepath.Join(dir, "malformed.json"), "{not json")
	if got := readTaskIDOrUnknown(filepath.Join(dir, "malformed.json")); got != "unknown" {
		t.Errorf("malformed: got %q, want unknown", got)
	}
	writeItem(t, filepath.Join(dir, "no-id.json"), `{"payload":"x"}`)
	if got := readTaskIDOrUnknown(filepath.Join(dir, "no-id.json")); got != "unknown" {
		t.Errorf("empty id: got %q, want unknown", got)
	}
	writeItem(t, filepath.Join(dir, "ok.json"), `{"id":"ok"}`)
	if got := readTaskIDOrUnknown(filepath.Join(dir, "ok.json")); got != "ok" {
		t.Errorf("ok: got %q", got)
	}
}
