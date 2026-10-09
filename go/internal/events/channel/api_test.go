package channel

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDir_IsTheChannelDirectoryUnderTheRoot(t *testing.T) {
	t.Parallel()
	l, root := newTestLog(t, 1<<20)

	if got := l.Dir(); got != filepath.Join(root, testChannel) {
		t.Fatalf("Dir() = %q, want %q", got, filepath.Join(root, testChannel))
	}
}

func TestSegments_ListsTheBasesInOrderWithTheTailLast(t *testing.T) {
	t.Parallel()
	l, root := newTestLog(t, 1<<20)
	writeSegment(t, root, 900, "")
	writeSegment(t, root, 0, "")
	writeSegment(t, root, 45, "")

	segs, err := l.Segments()

	want := []Segment{{Base: 0, Path: segmentPath(root, 0)}, {Base: 45, Path: segmentPath(root, 45)}, {Base: 900, Path: segmentPath(root, 900)}}
	if err != nil || !reflect.DeepEqual(segs, want) {
		t.Fatalf("Segments() = %+v, %v, want %+v", segs, err, want)
	}
}

func TestSegmentOf_FindsTheLargestBaseAtOrBelowTheCursor(t *testing.T) {
	t.Parallel()
	segs := []Segment{{Base: 100, Path: "a"}, {Base: 250, Path: "b"}, {Base: 400, Path: "c"}}
	cases := []struct {
		cursor int64
		want   string
		found  bool
	}{{99, "", false}, {100, "a", true}, {249, "a", true}, {250, "b", true}, {399, "b", true}, {400, "c", true}, {1 << 40, "c", true}}
	for _, tc := range cases {
		got, found := SegmentOf(segs, tc.cursor)
		if found != tc.found || got.Path != tc.want {
			t.Errorf("SegmentOf(%d) = %+v, %v, want %q, %v", tc.cursor, got, found, tc.want, tc.found)
		}
	}
	if _, found := SegmentOf(nil, 0); found {
		t.Error("SegmentOf on no segment found one")
	}
}

func TestEnd_IsTheEndOfTheLastCompleteLine(t *testing.T) {
	t.Parallel()
	l, root := newTestLog(t, 1<<20)
	if _, err := l.End(); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("End() on a missing directory = %v, want fs.ErrNotExist", err)
	}
	if err := os.MkdirAll(filepath.Join(root, testChannel), 0o755); err != nil {
		t.Fatal(err)
	}
	if end, err := l.End(); err != nil || end != 0 {
		t.Fatalf("End() on an empty channel = %d, %v, want 0", end, err)
	}
	line := int64(len(recordLine(t, signalRecord(1, "r1"))))
	mustAppend(t, l, signalRecord(1, "r1"), signalRecord(2, "r2"))
	appendRaw(t, segmentPath(root, 0), `{"torn`)

	end, err := l.End()

	if err != nil || end != 2*line {
		t.Fatalf("End() = %d, %v, want %d: the end of the last complete line", end, err, 2*line)
	}
}

func TestEnd_AnUnreadableTailFails(t *testing.T) {
	t.Parallel()
	l, root := newTestLog(t, 1<<20)
	if err := os.MkdirAll(segmentPath(root, 0), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := l.End(); err == nil {
		t.Fatal("End() of a tail that is a directory = nil, want an error")
	}
}

func TestReadN_ABatchStaysWithinMaxBytesAndCursorsContinue(t *testing.T) {
	t.Parallel()
	line := int64(len(recordLine(t, signalRecord(1, "r1"))))
	l, _ := newTestLog(t, line)
	for i := uint64(1); i <= 5; i++ {
		mustAppend(t, l, signalRecord(i, "r"+itoa(int64(i))))
	}
	whole := mustRead(t, l, 0)

	var got []Record
	from := int64(0)
	for from < whole.Next {
		b, err := l.ReadN(from, 2*line)
		if err != nil {
			t.Fatal(err)
		}
		if want := min(2, int((whole.Next-from)/line)); len(b.Records) != want || b.Records[0].Cursor != from {
			t.Fatalf("ReadN(%d, %d) = %q, want %d record(s) from that cursor", from, 2*line, describe(b.Records), want)
		}
		got = append(got, b.Records...)
		from = b.Next
	}

	if !reflect.DeepEqual(describe(got), describe(whole.Records)) {
		t.Fatalf("records over ReadN calls = %q, want %q", describe(got), describe(whole.Records))
	}
}

func TestReadN_ARecordLargerThanMaxBytesStillComesAlone(t *testing.T) {
	t.Parallel()
	l, _ := newTestLog(t, 1<<20)
	line := int64(len(recordLine(t, signalRecord(1, "r1"))))
	mustAppend(t, l, signalRecord(1, "r1"), signalRecord(2, "r2"))

	b, err := l.ReadN(0, 1)

	if err != nil || len(b.Records) != 1 || b.Next != line {
		t.Fatalf("ReadN(0, 1) = %q next %d, %v, want one record and next %d", describe(b.Records), b.Next, err, line)
	}
}

func TestRead_ASegmentRemovedAfterTheListingYieldsARetentionGap(t *testing.T) {
	t.Parallel()
	l, root := newTestLog(t, 1<<20)
	writeSegment(t, root, 500, recordLine(t, signalRecord(2, "kept")))
	segs := []Segment{{Base: 0, Path: segmentPath(root, 0)}, {Base: 500, Path: segmentPath(root, 500)}}

	b, err := l.readFrom(segs, 10, 1<<40)

	if got, want := describe(b.Records), []string{"watch gap retention 10 500", "at 500 kept"}; err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("readFrom over a removed segment = %q, %v, want %q", got, err, want)
	}
	tailGone, err := l.readFrom([]Segment{{Base: 0, Path: segmentPath(root, 0)}}, 0, 1<<40)
	if !errors.Is(err, fs.ErrNotExist) || tailGone.Next != 0 {
		t.Fatalf("readFrom of a removed tail = %+v, %v, want fs.ErrNotExist", tailGone, err)
	}
}
