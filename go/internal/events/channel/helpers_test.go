package channel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const testChannel = "loop"

func testConfig(segmentBytes int64) Config {
	return Config{SegmentBytes: segmentBytes, LockDeadline: time.Minute}
}

func newTestLog(t *testing.T, segmentBytes int64) (*Log, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "ch")
	l, err := New(root, testChannel, testConfig(segmentBytes))
	if err != nil {
		t.Fatalf("New(%s, %s) = %v", root, testChannel, err)
	}
	return l, root
}

func signalRecord(seq uint64, reason string) Record {
	return Record{Source: "loop", Signal: &signalcenter.Event{
		SchemaVersion: "signal/1.0", Seq: seq, PID: 4242, TS: "2026-10-09T17:46:02.114Z",
		Module: signalcenter.ModuleLoop, Origin: "test", Kind: signalcenter.KindLoopWave,
		Severity: signalcenter.SeverityInfo, Reason: reason,
	}}
}

func recordLine(t *testing.T, r Record) string {
	t.Helper()
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw) + "\n"
}

func segmentPath(root string, base int64) string {
	return filepath.Join(root, testChannel, segmentName(base))
}

func writeSegment(t *testing.T, root string, base int64, content string) {
	t.Helper()
	path := segmentPath(root, base)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func appendRaw(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
}

func mustAppend(t *testing.T, l *Log, records ...Record) int64 {
	t.Helper()
	cursor, err := l.Append(records)
	if err != nil {
		t.Fatalf("Append = %v", err)
	}
	return cursor
}

func mustRead(t *testing.T, l *Log, from int64) Batch {
	t.Helper()
	b, err := l.Read(from)
	if err != nil {
		t.Fatalf("Read(%d) = %v", from, err)
	}
	return b
}

func segmentBases(t *testing.T, root string) []int64 {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, testChannel))
	if err != nil {
		t.Fatal(err)
	}
	var bases []int64
	for _, e := range entries {
		if base, ok := segmentBase(e.Name()); ok {
			bases = append(bases, base)
		}
	}
	return bases
}

func describe(records []Record) []string {
	out := make([]string, len(records))
	for i, r := range records {
		switch {
		case r.Gap != nil:
			out[i] = strings.Join([]string{r.Source, "gap", r.Gap.Reason, itoa(r.Gap.From), itoa(r.Gap.To)}, " ")
		default:
			out[i] = strings.Join([]string{"at", itoa(r.Cursor), r.Signal.Reason}, " ")
		}
	}
	return out
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
