package signalcenter

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func streamEvent(seq uint64, ts string) Event {
	return Event{SchemaVersion: "signal/1.0", Seq: seq, TS: ts, Module: ModuleLoop, Origin: "Batch.run", Kind: KindPhaseOutcome, Severity: SeverityInfo, Reason: "r"}
}

func streamLines(t *testing.T, events ...Event) string {
	t.Helper()
	var b strings.Builder
	for _, e := range events {
		raw, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(raw)
		b.WriteByte('\n')
	}
	return b.String()
}

func TestSignalsTailReadStreamDecodesCompleteLinesAndHoldsTheFragment(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), StreamFileName)
	first, second := streamEvent(1, "2026-10-05T10:00:01Z"), streamEvent(2, "2026-10-05T10:00:02.5Z")
	complete := streamLines(t, first) + "{broken\n \t\n" + streamLines(t, streamEvent(9, "noon"), second)
	pending := streamLines(t, streamEvent(3, "2026-10-05T10:00:03Z"))
	if err := os.WriteFile(path, []byte(complete+pending[:len(pending)-1]), 0o644); err != nil {
		t.Fatal(err)
	}
	var chunk StreamChunk
	chunk, err := ReadStream(path, 0)
	if err != nil || chunk.Skipped != 2 || chunk.Next != int64(len(complete)) || !reflect.DeepEqual(chunk.Events, []Event{first, second}) {
		t.Fatalf("ReadStream(0) = %+v, %v", chunk, err)
	}
	held, err := ReadStream(path, chunk.Next)
	if err != nil || len(held.Events) != 0 || held.Next != chunk.Next {
		t.Fatalf("a fragment-only remainder must not be consumed: %+v, %v", held, err)
	}
	for _, from := range []int64{-5, int64(len(complete) + len(pending) + 10)} {
		again, err := ReadStream(path, from)
		if err != nil || again.Next != int64(len(complete)) || len(again.Events) != 2 {
			t.Errorf("ReadStream(%d) must re-read from the top: %+v, %v", from, again, err)
		}
	}
}

func TestSignalsTailReadStreamSplitsMissingFromIOFaults(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	missing, err := ReadStream(filepath.Join(dir, "absent.ndjson"), 11)
	if !errors.Is(err, fs.ErrNotExist) || missing.Next != 11 || len(missing.Events) != 0 {
		t.Errorf("a missing stream is fs.ErrNotExist keeping Next: %+v, %v", missing, err)
	}
	isDir, err := ReadStream(dir, 11)
	if err == nil || errors.Is(err, fs.ErrNotExist) || isDir.Next != 11 || !strings.Contains(err.Error(), "signalcenter: read stream "+dir) {
		t.Errorf("a directory is an I/O fault, not a missing stream: %+v, %v", isDir, err)
	}
}

func TestSignalsTailMergeByTSOrdersByInstantStablyAndPurely(t *testing.T) {
	t.Parallel()
	a1, a2 := streamEvent(1, "2026-10-05T10:00:05.1Z"), streamEvent(2, "2026-10-05T10:00:07Z")
	b1, b2 := streamEvent(3, "2026-10-05T10:00:05Z"), streamEvent(4, "2026-10-05T10:00:07Z")
	bad := streamEvent(5, "garbled")
	in := [][]Event{{a1, a2}, {b1, bad, b2}}
	before := [][]Event{{a1, a2}, {b1, bad, b2}}
	merged := MergeByTS(in...)
	if want := []Event{bad, b1, a1, a2, b2}; !reflect.DeepEqual(merged, want) {
		t.Fatalf("MergeByTS = %v, want %v", merged, want)
	}
	merged[0].Reason = "mutated"
	if !reflect.DeepEqual(in, before) {
		t.Error("MergeByTS must not touch or alias its inputs")
	}
	if empty := MergeByTS(); len(empty) != 0 {
		t.Errorf("MergeByTS() = %v", empty)
	}
}
