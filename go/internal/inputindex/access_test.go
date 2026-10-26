package inputindex

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRecordAccess_WritesOneRecordForEachOpenedEntryAndOneForTheCell(t *testing.T) {
	f := retryFixture(t)
	idx := f.build(7, "build")
	scout := mustEntry(t, idx, "scout-report", 7)
	unopened, err := RecordAccess(f.root, idx, "run1/7/build/p1n2", AccessSourceSandbox, []string{scout.ID})
	if err != nil {
		t.Fatal(err)
	}
	want := slices.DeleteFunc(slices.Clone(idx.Required), func(id string) bool { return id == scout.ID })
	if !slices.Equal(unopened, want) || len(want) == 0 {
		t.Fatalf("unopened = %v, want the required ids except %s: %v", unopened, scout.ID, want)
	}
	if _, err := RecordAccess(f.root, idx, "run1/7/build/p1n3", AccessSourceNone, nil); err != nil {
		t.Fatal(err)
	}
	records, err := ReadAccess(f.root)
	if err != nil || len(records) != 3 {
		t.Fatalf("records = %+v err = %v, want an open, a cell and a second cell", records, err)
	}
	open, cell, none := records[0], records[1], records[2]
	if open.Type != AccessOpen || open.Entry != scout.ID || open.Kind != "scout-report" || open.Layer != LayerUpstream || !open.Required || open.Cycle != 7 || open.Phase != "build" || open.Source != AccessSourceSandbox {
		t.Fatalf("open record = %+v", open)
	}
	if cell.Type != AccessCell || len(cell.Listed) != len(idx.Entries) || !slices.Equal(cell.Opened, []string{scout.ID}) || !slices.Equal(cell.Unopened, want) {
		t.Fatalf("cell record = %+v", cell)
	}
	if none.Source != AccessSourceNone || len(none.Opened) != 0 || none.Dispatch != "run1/7/build/p1n3" {
		t.Fatalf("a cell with no report source = %+v", none)
	}
	if accessLogPath(f.root, 7) != filepath.Join(AccessDir(f.root), "cycle-7.jsonl") || !strings.HasSuffix(AccessDir(f.root), "/.evolve/context-access") {
		t.Fatalf("accessLogPath = %s", accessLogPath(f.root, 7))
	}
}

func TestRecordAccess_FailsLoudlyAndReadAccessSkipsABadLine(t *testing.T) {
	f := retryFixture(t)
	idx := f.build(7, "build")
	f.write(".evolve/context-access", "a file where the directory belongs")
	if _, err := RecordAccess(f.root, idx, "d", AccessSourceNone, nil); err == nil {
		t.Fatal("an unwritable access log must fail")
	}
	if err := os.Remove(f.root + "/.evolve/context-access"); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadAccess(f.root); err != nil || len(got) != 0 {
		t.Fatalf("no log yet: %v %v", got, err)
	}
	if err := os.MkdirAll(accessLogPath(f.root, 7), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordAccess(f.root, idx, "d", AccessSourceNone, nil); err == nil || !strings.Contains(err.Error(), "write the access log") {
		t.Fatalf("a log path that is a directory must fail: %v", err)
	}
	if err := os.Remove(accessLogPath(f.root, 7)); err != nil {
		t.Fatal(err)
	}
	f.write(".evolve/context-access/cycle-7.jsonl", "not json\n\n{\"type\":\"cell\",\"cycle\":7}\n")
	if got, err := ReadAccess(f.root); err != nil || len(got) != 1 || got[0].Type != AccessCell {
		t.Fatalf("a bad line is skipped: %+v %v", got, err)
	}
	f.write(".evolve/context-access/cycle-8.jsonl/x", "")
	if _, err := ReadAccess(f.root); err == nil {
		t.Fatal("an unreadable log file must fail")
	}
}
