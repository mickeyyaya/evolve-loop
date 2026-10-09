package signalcenter

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeLines(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "seg.ndjson")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func appendBytes(t *testing.T, path, content string) {
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

func lineStrings(lines [][]byte) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = string(l)
	}
	return out
}

func TestReadLines_ATornTailIsNotReturnedUntilItsNewlineArrives(t *testing.T) {
	t.Parallel()
	path := writeLines(t, "{\"a\":1}\n{\"b\":")

	torn, err := ReadLines(path, 0)
	if err != nil {
		t.Fatalf("ReadLines with a torn tail: %v", err)
	}
	if got := lineStrings(torn.Lines); !reflect.DeepEqual(got, []string{`{"a":1}`}) {
		t.Fatalf("ReadLines with a torn tail = %q, want only the complete line", got)
	}
	appendBytes(t, path, "2}\n")
	whole, err := ReadLines(path, torn.Next)

	if err != nil {
		t.Fatalf("ReadLines after the newline arrived: %v", err)
	}
	if got := lineStrings(whole.Lines); !reflect.DeepEqual(got, []string{`{"b":2}`}) {
		t.Fatalf("ReadLines after the newline arrived = %q, want the repaired line", got)
	}
}

func TestReadLines_NextIsTheOffsetAfterTheLastCompleteLine(t *testing.T) {
	t.Parallel()
	complete := "one\ntwo\n"
	path := writeLines(t, complete+"thr")

	from4, err := ReadLines(path, 4)

	if err != nil {
		t.Fatal(err)
	}
	if from4.Start != 4 || from4.Next != int64(len(complete)) {
		t.Fatalf("ReadLines(4) = start %d next %d, want start 4 next %d", from4.Start, from4.Next, len(complete))
	}
	if got := lineStrings(from4.Lines); !reflect.DeepEqual(got, []string{"two"}) {
		t.Fatalf("ReadLines(4) lines = %q, want [two]", got)
	}
	atEnd, err := ReadLines(path, from4.Next)
	if err != nil || atEnd.Next != from4.Next || len(atEnd.Lines) != 0 {
		t.Fatalf("ReadLines at the last complete line = %+v, %v, want no lines and the same next", atEnd, err)
	}
	pastEnd, err := ReadLines(path, int64(len(complete)+10))
	if err != nil || pastEnd.Start != 0 || pastEnd.Next != int64(len(complete)) || len(pastEnd.Lines) != 2 {
		t.Fatalf("ReadLines past the end = %+v, %v, want a read from start 0", pastEnd, err)
	}
}

func TestReadLines_AMalformedCompleteLineIsCountedNotFatal(t *testing.T) {
	t.Parallel()
	good := streamLines(t, streamEvent(1, "2026-10-09T10:00:00Z"))
	path := writeLines(t, "{not json\n"+good)

	lines, err := ReadLines(path, 0)
	stream, streamErr := ReadStream(path, 0)

	if err != nil || streamErr != nil {
		t.Fatalf("a malformed complete line must not fail the read: lines %v, stream %v", err, streamErr)
	}
	if got := lineStrings(lines.Lines); !reflect.DeepEqual(got, []string{"{not json", strings.TrimSuffix(good, "\n")}) {
		t.Fatalf("ReadLines = %q, want the malformed line and the good line", got)
	}
	if stream.Skipped != 1 || len(stream.Events) != 1 || stream.Next != lines.Next {
		t.Fatalf("ReadStream = %+v, want one skipped, one event and next %d", stream, lines.Next)
	}
}

func TestReadLines_AMissingFileKeepsTheCursorAndNamesThePath(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "absent.ndjson")

	chunk, err := ReadLines(path, 7)

	if !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), "signalcenter: read lines "+path) {
		t.Fatalf("ReadLines on a missing file err = %v, want fs.ErrNotExist naming the path", err)
	}
	if !reflect.DeepEqual(chunk, LineChunk{Start: 7, Next: 7}) {
		t.Fatalf("ReadLines on a missing file = %+v, want the cursor kept", chunk)
	}
}
