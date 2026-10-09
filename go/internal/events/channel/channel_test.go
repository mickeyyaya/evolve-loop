package channel

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
)

func TestAppend_RepairsATornTailBeforeTheBatch(t *testing.T) {
	t.Parallel()
	l, root := newTestLog(t, 1<<20)
	first := recordLine(t, signalRecord(1, "first"))
	mustAppend(t, l, signalRecord(1, "first"))
	torn := `{"source":"loop","sig`
	appendRaw(t, segmentPath(root, 0), torn)

	cursor := mustAppend(t, l, signalRecord(2, "second"))

	repaired := int64(len(first) + len(torn) + 1)
	if cursor != repaired {
		t.Fatalf("Append after a torn tail = cursor %d, want %d: the batch starts after the repaired line", cursor, repaired)
	}
	got := describe(mustRead(t, l, 0).Records)
	want := []string{"at 0 first", "watch gap malformed " + itoa(int64(len(first))) + " " + itoa(repaired), "at " + itoa(repaired) + " second"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Read(0) = %q, want %q", got, want)
	}
}

func TestAppend_AnIntactTailGetsNoRepair(t *testing.T) {
	t.Parallel()
	l, _ := newTestLog(t, 1<<20)
	first := recordLine(t, signalRecord(1, "first"))
	mustAppend(t, l, signalRecord(1, "first"))

	cursor := mustAppend(t, l, signalRecord(2, "second"))

	if cursor != int64(len(first)) {
		t.Fatalf("Append after a whole line = cursor %d, want %d", cursor, len(first))
	}
}

func TestAppend_RotatesPastTheCapAndCursorsKeepIncreasing(t *testing.T) {
	t.Parallel()
	line := int64(len(recordLine(t, signalRecord(1, "r1"))))
	l, root := newTestLog(t, line)
	var cursors []int64

	for i := uint64(1); i <= 4; i++ {
		cursors = append(cursors, mustAppend(t, l, signalRecord(i, "r"+itoa(int64(i)))))
	}

	if want := []int64{0, line, 2 * line, 3 * line}; !reflect.DeepEqual(cursors, want) {
		t.Fatalf("Append cursors = %v, want %v", cursors, want)
	}
	if got, want := segmentBases(t, root), []int64{0, 2 * line, 4 * line}; !reflect.DeepEqual(got, want) {
		t.Fatalf("segment bases = %v, want %v: a segment at the cap stays, one past it rotates to base+size", got, want)
	}
	b := mustRead(t, l, 0)
	if got, want := describe(b.Records), []string{"at 0 r1", "at " + itoa(line) + " r2", "at " + itoa(2*line) + " r3", "at " + itoa(3*line) + " r4"}; !reflect.DeepEqual(got, want) || b.Next != 4*line {
		t.Fatalf("Read(0) = %q next %d, want %q next %d", got, b.Next, want, 4*line)
	}
}

func TestAppend_OneBatchIsOneRunOfLines(t *testing.T) {
	t.Parallel()
	l, _ := newTestLog(t, 1<<20)
	one := int64(len(recordLine(t, signalRecord(1, "a"))))

	cursor := mustAppend(t, l, signalRecord(1, "a"), signalRecord(2, "b"))

	got := describe(mustRead(t, l, 0).Records)
	if want := []string{"at 0 a", "at " + itoa(one) + " b"}; cursor != 0 || !reflect.DeepEqual(got, want) {
		t.Fatalf("a batch of two = cursor %d, %q, want cursor 0, %q", cursor, got, want)
	}
}

func TestRead_ACursorFindsItsSegmentAcrossRotation(t *testing.T) {
	t.Parallel()
	line := int64(len(recordLine(t, signalRecord(1, "r1"))))
	l, _ := newTestLog(t, line)
	var cursors []int64
	for i := uint64(1); i <= 5; i++ {
		cursors = append(cursors, mustAppend(t, l, signalRecord(i, "r"+itoa(int64(i)))))
	}

	for k, from := range cursors {
		b := mustRead(t, l, from)

		if len(b.Records) != len(cursors)-k || b.Records[0].Cursor != from || b.Records[0].Signal.Seq != uint64(k+1) {
			t.Fatalf("Read(%d) = %q, want records %d to 5 starting at that cursor", from, describe(b.Records), k+1)
		}
		if b.Next != 5*line {
			t.Fatalf("Read(%d).Next = %d, want %d", from, b.Next, 5*line)
		}
	}
}

func TestRead_LastIsTheLastRecordOfTheTailOrTheSegmentBefore(t *testing.T) {
	t.Parallel()
	line := int64(len(recordLine(t, signalRecord(1, "r1"))))
	t.Run("the last line of the tail", func(t *testing.T) {
		l, root := newTestLog(t, 1<<20)
		mustAppend(t, l, signalRecord(1, "r1"), signalRecord(2, "r2"))
		appendRaw(t, segmentPath(root, 0), `{"torn`)

		got, err := l.Last()

		if err != nil || got != line {
			t.Fatalf("Last() = %d, %v, want %d: the last complete line of the tail", got, err, line)
		}
	})
	t.Run("the segment before an empty tail", func(t *testing.T) {
		l, root := newTestLog(t, line)
		mustAppend(t, l, signalRecord(1, "r1"))
		mustAppend(t, l, signalRecord(2, "r2"))
		if bases := segmentBases(t, root); len(bases) != 2 {
			t.Fatalf("segment bases = %v, want an empty tail after a rotation", bases)
		}

		got, err := l.Last()

		if err != nil || got != line {
			t.Fatalf("Last() = %d, %v, want %d: the last line of the segment before the empty tail", got, err, line)
		}
	})
	t.Run("the end of a channel with no record", func(t *testing.T) {
		l, root := newTestLog(t, 1<<20)
		if err := os.MkdirAll(filepath.Join(root, testChannel), 0o755); err != nil {
			t.Fatal(err)
		}

		got, err := l.Last()

		if err != nil || got != 0 {
			t.Fatalf("Last() on an empty channel = %d, %v, want 0", got, err)
		}
	})
	t.Run("only the tail and the segment before count", func(t *testing.T) {
		l, root := newTestLog(t, 1<<20)
		writeSegment(t, root, 0, recordLine(t, signalRecord(1, "old")))
		writeSegment(t, root, 100, "")
		writeSegment(t, root, 200, `{"torn`)

		got, err := l.Last()

		if err != nil || got != 200 {
			t.Fatalf("Last() = %d, %v, want 200: the end when the tail and the segment before have no line", got, err)
		}
	})
}

func TestRead_ACursorPastTheTailMovesToTheTrueEndWithOneResetGap(t *testing.T) {
	t.Parallel()
	l, root := newTestLog(t, 1<<20)
	end := mustAppend(t, l, signalRecord(1, "r1"), signalRecord(2, "r2")) + int64(2*len(recordLine(t, signalRecord(1, "r1"))))
	appendRaw(t, segmentPath(root, 0), `{"torn`)
	from := end + 50

	b := mustRead(t, l, from)

	if got, want := describe(b.Records), []string{"watch gap reset " + itoa(from) + " " + itoa(end)}; !reflect.DeepEqual(got, want) || b.Next != end {
		t.Fatalf("Read(%d) = %q next %d, want %q next %d", from, got, b.Next, want, end)
	}
}

func TestRead_ACursorPastASegmentEndMovesToTheNextBase(t *testing.T) {
	t.Parallel()
	l, root := newTestLog(t, 1<<20)
	early := recordLine(t, signalRecord(1, "early"))
	writeSegment(t, root, 0, early)
	writeSegment(t, root, 1000, recordLine(t, signalRecord(2, "late")))

	past := mustRead(t, l, int64(len(early))+5)
	whole := mustRead(t, l, 0)

	if got, want := describe(past.Records), []string{"watch gap reset " + itoa(int64(len(early))+5) + " 1000", "at 1000 late"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Read past the end of a segment = %q, want %q", got, want)
	}
	if got, want := describe(whole.Records), []string{"at 0 early", "watch gap reset " + itoa(int64(len(early))) + " 1000", "at 1000 late"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Read(0) over a segment that ends below the next base = %q, want %q", got, want)
	}
}

func TestRead_AMalformedLineYieldsAGapAndTheCursorMovesPastIt(t *testing.T) {
	t.Parallel()
	l, root := newTestLog(t, 1<<20)
	good := recordLine(t, signalRecord(1, "good"))
	gap := recordLine(t, Record{Source: "loop", Gap: &Gap{Reason: "queue_full", PID: 7, FirstSeq: 3, LastSeq: 4, Dropped: 2}})
	bad := []string{"not json\n", `{"source":"loop"}` + "\n", strings.TrimSuffix(good, "}\n") + `,"gap":{"reason":"x"}}` + "\n"}
	writeSegment(t, root, 0, bad[0]+bad[1]+gap+bad[2]+good)

	b := mustRead(t, l, 0)

	at := func(n int) int64 { return int64(n) }
	c1, c2 := at(len(bad[0])), at(len(bad[0])+len(bad[1]))
	c3 := c2 + at(len(gap))
	c4 := c3 + at(len(bad[2]))
	want := []string{
		"watch gap malformed 0 " + itoa(c1),
		"watch gap malformed " + itoa(c1) + " " + itoa(c2),
		"loop gap queue_full 0 0",
		"watch gap malformed " + itoa(c3) + " " + itoa(c4),
		"at " + itoa(c4) + " good",
	}
	if got := describe(b.Records); !reflect.DeepEqual(got, want) || b.Next != c4+at(len(good)) {
		t.Fatalf("Read(0) = %q next %d, want %q next %d", got, b.Next, want, c4+at(len(good)))
	}
	if g := b.Records[2]; g.Cursor != c2 || g.Gap.PID != 7 || g.Gap.FirstSeq != 3 || g.Gap.LastSeq != 4 || g.Gap.Dropped != 2 {
		t.Fatalf("a stored gap record = %+v at %d, want pid 7, seq 3 to 4, 2 dropped at %d", *g.Gap, g.Cursor, c2)
	}
}

func TestRead_ACursorBelowTheOldestSegmentYieldsARetentionGap(t *testing.T) {
	t.Parallel()
	l, root := newTestLog(t, 1<<20)
	writeSegment(t, root, 100, recordLine(t, signalRecord(1, "kept")))

	b := mustRead(t, l, 0)

	if got, want := describe(b.Records), []string{"watch gap retention 0 100", "at 100 kept"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Read(0) below retention = %q, want %q", got, want)
	}
}

func TestRead_AChannelWithNoSegmentEndsAtZero(t *testing.T) {
	t.Parallel()
	l, root := newTestLog(t, 1<<20)
	if err := os.MkdirAll(filepath.Join(root, testChannel), 0o755); err != nil {
		t.Fatal(err)
	}

	atZero := mustRead(t, l, 0)
	past := mustRead(t, l, 40)

	if len(atZero.Records) != 0 || atZero.Next != 0 {
		t.Fatalf("Read(0) on an empty channel = %q next %d, want nothing at 0", describe(atZero.Records), atZero.Next)
	}
	if got, want := describe(past.Records), []string{"watch gap reset 40 0"}; !reflect.DeepEqual(got, want) || past.Next != 0 {
		t.Fatalf("Read(40) on an empty channel = %q next %d, want %q next 0", got, past.Next, want)
	}
}

func TestRead_AMissingChannelDirectoryIsNotExist(t *testing.T) {
	t.Parallel()
	l, _ := newTestLog(t, 1<<20)

	b, err := l.Read(17)
	_, lastErr := l.Last()

	if !errors.Is(err, fs.ErrNotExist) || b.Next != 17 || b.Records != nil {
		t.Fatalf("Read on a missing directory = %+v, %v, want fs.ErrNotExist and the cursor kept", b, err)
	}
	if !errors.Is(lastErr, fs.ErrNotExist) {
		t.Fatalf("Last on a missing directory = %v, want fs.ErrNotExist", lastErr)
	}
}

func TestRead_AnUnreadableSegmentFailsTheRead(t *testing.T) {
	t.Parallel()
	l, root := newTestLog(t, 1<<20)
	good := recordLine(t, signalRecord(1, "good"))
	writeSegment(t, root, 0, good)
	if err := os.MkdirAll(segmentPath(root, int64(len(good))), 0o755); err != nil {
		t.Fatal(err)
	}

	b, err := l.Read(0)
	_, lastErr := l.Last()

	if err == nil || b.Next != 0 || b.Records != nil || !strings.Contains(err.Error(), "channel loop") {
		t.Fatalf("Read of a segment that is a directory = %+v, %v, want an error naming the channel and the cursor kept", b, err)
	}
	if lastErr == nil || !strings.Contains(lastErr.Error(), "channel loop") {
		t.Fatalf("Last of a segment that is a directory = %v, want an error naming the channel", lastErr)
	}
}

func TestRead_IgnoresFilesThatAreNotSegments(t *testing.T) {
	t.Parallel()
	l, root := newTestLog(t, 1<<20)
	writeSegment(t, root, 0, recordLine(t, signalRecord(1, "kept")))
	dir := filepath.Join(root, testChannel)
	for _, name := range []string{"seg-1.ndjson", "seg-0000000000000000000x.ndjson", "seg-99999999999999999999.ndjson", "notes.txt", "seg-00000000000000000500.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("junk\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	b := mustRead(t, l, 0)

	if got, want := describe(b.Records), []string{"at 0 kept"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Read(0) with foreign files = %q, want %q", got, want)
	}
}

func TestNew_RefusesANameOutsideTheTokenRule(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range []string{"", ".", "..", "a/b", "Loop", "a..b", ".a", "a.", "ci.*", "ci.>"} {
		if _, err := New(root, name, testConfig(1)); err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("New(%q) = %v, want a refusal that names it", name, err)
		}
	}
	for _, name := range []string{"loop", "ci.required", "a_b-1.x9"} {
		if _, err := New(root, name, testConfig(1)); err != nil {
			t.Errorf("New(%q) = %v, want it accepted", name, err)
		}
	}
}

func TestNew_PlacesTheLockBesideTheChannelDirectory(t *testing.T) {
	t.Parallel()
	l, root := newTestLog(t, 1<<20)
	mustAppend(t, l, signalRecord(1, "r1"))

	if _, err := os.Stat(filepath.Join(root, testChannel+".lock")); err != nil {
		t.Fatalf("the lock file is not at ch/%s.lock: %v", testChannel, err)
	}
	if _, err := os.Stat(segmentPath(root, 0)); err != nil {
		t.Fatalf("the first segment is not at ch/%s/%s: %v", testChannel, segmentName(0), err)
	}
}

func TestSegmentName_PadsTheBaseTo20Digits(t *testing.T) {
	t.Parallel()
	if got := segmentName(184467); got != "seg-00000000000000184467.ndjson" {
		t.Fatalf("segmentName(184467) = %q", got)
	}
	if base, ok := segmentBase("seg-00000000000000184467.ndjson"); !ok || base != 184467 {
		t.Fatalf("segmentBase = %d, %v, want 184467", base, ok)
	}
}

func TestAppend_RefusesARecordLongerThanTheLimit(t *testing.T) {
	t.Parallel()
	l, root := newTestLog(t, 1<<20)
	base := recordLine(t, signalRecord(1, ""))
	pad := MaxRecordBytes - (len(base) - 1)
	atLimit, overLimit := signalRecord(1, strings.Repeat("a", pad)), signalRecord(2, strings.Repeat("a", pad+1))

	_, overErr := l.Append([]Record{overLimit})
	if _, err := os.Stat(filepath.Join(root, testChannel)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a refused batch touched the channel: %v", err)
	}
	cursor, atErr := l.Append([]Record{atLimit})

	if !errors.Is(overErr, ErrRecordTooLong) || !strings.Contains(overErr.Error(), "channel loop") {
		t.Fatalf("Append of %d bytes = %v, want ErrRecordTooLong naming the channel", MaxRecordBytes+1, overErr)
	}
	if atErr != nil || cursor != 0 || len(recordLine(t, atLimit)) != MaxRecordBytes+1 {
		t.Fatalf("Append of exactly %d bytes = %v, want it written", MaxRecordBytes, atErr)
	}
}

func TestAppend_ALockPastItsDeadlineWritesNothing(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "ch")
	l, err := New(root, testChannel, Config{SegmentBytes: 1 << 20, LockDeadline: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	holder, err := flock.Lock(filepath.Join(root, testChannel+".lock"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(holder)

	_, appendErr := l.Append([]Record{signalRecord(1, "r1")})

	if !errors.Is(appendErr, flock.ErrLockDeadline) || !strings.Contains(appendErr.Error(), "channel loop") {
		t.Fatalf("Append under a held lock = %v, want flock.ErrLockDeadline naming the channel", appendErr)
	}
	if _, err := os.Stat(filepath.Join(root, testChannel)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("an append past its lock deadline wrote: %v", err)
	}
}

func TestRecord_ConstantsMatchTheSpec(t *testing.T) {
	t.Parallel()
	got := []string{SourceWatch, ReasonReset, ReasonMalformed, ReasonRetention, ReasonQueueFull, ReasonWriteError, ReasonLockDeadline}
	if want := []string{"watch", "reset", "malformed", "retention", "queue_full", "write_error", "lock_deadline"}; !reflect.DeepEqual(got, want) || MaxRecordBytes != 4608 {
		t.Fatalf("constants = %q, %d, want %q, 4608", got, MaxRecordBytes, want)
	}
}
