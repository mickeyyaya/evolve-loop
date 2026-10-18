package channelstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/address"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/broker"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/envelope"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/identity"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var _ broker.Store = (*Store)(nil)

const cycle = 5

const firstSegment = "seg-00000000000000000000.ndjson"

var sentAt = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

func record(id string) broker.Record {
	return broker.Record{
		Envelope: envelope.Envelope{
			Schema: envelope.SchemaVersion, ID: id, TS: sentAt, Kind: envelope.KindNote,
			From:      identity.Principal{Cycle: cycle, Dispatch: "r/5/scout/p1n1", Agent: "scout", Phase: "scout", Role: "scout"},
			To:        []address.Address{{Selector: address.SelectPhase, Value: "audit"}},
			Body:      "the flag is in cmd_loop.go",
			Refs:      []string{"build-report.md"},
			ContextID: cycle,
		},
		Recipients: []identity.Principal{{Cycle: cycle, Phase: "audit"}},
	}
}

func testConfig(segmentBytes int64) channel.Config {
	return channel.Config{SegmentBytes: segmentBytes, LockDeadline: time.Minute}
}

func newStore(t *testing.T, segmentBytes int64) (*Store, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "ch")
	s, err := New(root, testConfig(segmentBytes))
	if err != nil {
		t.Fatalf("New error = %v", err)
	}
	return s, root
}

func rawLog(t *testing.T, root, name string) *channel.Log {
	t.Helper()
	l, err := channel.New(root, name, testConfig(1<<20))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func appendAll(t *testing.T, s *Store, cycle int, ids ...string) []int64 {
	t.Helper()
	var cursors []int64
	for _, id := range ids {
		c, err := s.Append(context.Background(), cycle, record(id))
		if err != nil {
			t.Fatalf("Append(%s) error = %v", id, err)
		}
		cursors = append(cursors, c)
	}
	return cursors
}

func mustRead(t *testing.T, s *Store, cycle int, from int64) broker.Batch {
	t.Helper()
	b, err := s.Read(context.Background(), cycle, from)
	if err != nil {
		t.Fatalf("Read(%d, %d) error = %v", cycle, from, err)
	}
	return b
}

func ids(b broker.Batch) []string {
	var out []string
	for _, r := range b.Records {
		out = append(out, r.Envelope.ID)
	}
	return out
}

func keys(t *testing.T, raw []byte) []string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("%s is not a JSON object: %v", raw, err)
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func appendText(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(text); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Size()
}

func TestChannelStore_AppendsAMessageRecordToTheCycleChannel(t *testing.T) {
	t.Parallel()
	s, root := newStore(t, 1<<20)
	in := record("a")

	cursor, err := s.Append(context.Background(), cycle, in)

	if err != nil || cursor != 0 {
		t.Fatalf("Append = %d, %v, want cursor 0", cursor, err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "msg.5", firstSegment))
	if err != nil {
		t.Fatal(err)
	}
	line := bytes.TrimSuffix(raw, []byte("\n"))
	if got, want := keys(t, line), []string{"dispatch", "message", "source"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("record keys = %v, want %v: exactly one message payload, no signal and no gap", got, want)
	}
	var rec channel.Record
	if err := json.Unmarshal(line, &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Source != "msg" || rec.Dispatch != "r/5/scout/p1n1" || rec.Signal != nil || rec.Gap != nil {
		t.Fatalf("record = %+v, want source msg and the dispatch of the sender", rec)
	}
	if got, want := keys(t, rec.Message), []string{"envelope", "recipients"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("message keys = %v, want %v", got, want)
	}
	var stored broker.Record
	if err := json.Unmarshal(rec.Message, &stored); err != nil || !reflect.DeepEqual(stored, in) {
		t.Fatalf("stored message = %+v, %v, want the appended record", stored, err)
	}
	if _, err := os.Stat(filepath.Join(root, "msg.dead")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("an append touched the dead-letter channel: %v", err)
	}
}

func TestChannelStore_EachCycleHasItsOwnChannelAndIncreasingCursors(t *testing.T) {
	t.Parallel()
	s, root := newStore(t, 1<<20)

	five := appendAll(t, s, 5, "a", "b", "c")
	six := appendAll(t, s, 6, "x")

	if five[0] != 0 || five[1] <= five[0] || five[2] <= five[1] || six[0] != 0 {
		t.Fatalf("cursors = %v and %v, want increasing in cycle 5 and 0 first in cycle 6", five, six)
	}
	b := mustRead(t, s, 6, 0)
	if got := ids(b); !slices.Equal(got, []string{"x"}) {
		t.Fatalf("Read(cycle 6) = %v, want only x", got)
	}
	for _, name := range []string{"msg.5", "msg.6"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("channel %s: %v", name, err)
		}
	}
}

func TestChannelStore_ReadFromACursorReturnsTheRestAndTheNextCursor(t *testing.T) {
	t.Parallel()
	s, root := newStore(t, 1<<20)
	cursors := appendAll(t, s, cycle, "a", "b", "c")
	end := fileSize(t, filepath.Join(root, "msg.5", firstSegment))

	b := mustRead(t, s, cycle, cursors[1])
	again := mustRead(t, s, cycle, b.Next)

	if got := ids(b); !slices.Equal(got, []string{"b", "c"}) || b.Next != end || b.Next <= cursors[2] {
		t.Fatalf("Read(%d) = %v next %d, want [b c] next %d above %d", cursors[1], got, b.Next, end, cursors[2])
	}
	if b.Records[0].Cursor != cursors[1] || b.Records[1].Cursor != cursors[2] || len(b.Gaps) != 0 {
		t.Fatalf("cursors = %d, %d and gaps %v, want %d, %d and none", b.Records[0].Cursor, b.Records[1].Cursor, b.Gaps, cursors[1], cursors[2])
	}
	if len(again.Records) != 0 || again.Next != end || len(again.Gaps) != 0 {
		t.Fatalf("Read(next) = %v next %d gaps %v, want nothing and the same end %d", ids(again), again.Next, again.Gaps, end)
	}
}

func TestChannelStore_ACursorPastTheHeadGivesOneResetGapAndTheEnd(t *testing.T) {
	t.Parallel()
	s, root := newStore(t, 1<<20)
	appendAll(t, s, cycle, "a")
	end := fileSize(t, filepath.Join(root, "msg.5", firstSegment))

	b := mustRead(t, s, cycle, end+50)

	want := []broker.Gap{{From: end + 50, To: end, Reason: channel.ReasonReset}}
	if len(b.Records) != 0 || b.Next != end || !reflect.DeepEqual(b.Gaps, want) {
		t.Fatalf("Read past the head = %v next %d gaps %v, want nothing, next %d and %v: the broker refuses such a cursor before it reads the gaps", ids(b), b.Next, b.Gaps, end, want)
	}
}

func TestChannelStore_ACycleWithNoMessageReadsAsEmptyAndMakesNoChannel(t *testing.T) {
	t.Parallel()
	s, root := newStore(t, 1<<20)

	for _, from := range []int64{0, 9} {
		b := mustRead(t, s, 7, from)
		if len(b.Records) != 0 || len(b.Gaps) != 0 || b.Next != 0 {
			t.Fatalf("Read(7, %d) = %+v, want nothing and next 0", from, b)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "msg.7")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a read made the channel: %v", err)
	}
}

func TestChannelStore_RefusesANegativeCursor(t *testing.T) {
	t.Parallel()
	s, _ := newStore(t, 1<<20)

	_, err := s.Read(context.Background(), cycle, -1)

	if !errors.Is(err, ErrCursor) {
		t.Fatalf("Read(-1) error = %v, want ErrCursor", err)
	}
}

func TestChannelStore_ReadFollowsRotation(t *testing.T) {
	t.Parallel()
	s, root := newStore(t, 1)
	cursors := appendAll(t, s, cycle, "a", "b", "c")
	segments, err := rawLog(t, root, "msg.5").Segments()
	if err != nil {
		t.Fatal(err)
	}
	var bases []int64
	for _, seg := range segments {
		bases = append(bases, seg.Base)
	}
	end := bases[len(bases)-1]
	lastSegment := filepath.Join(root, "msg.5", fmt.Sprintf("seg-%020d.ndjson", cursors[2]))

	whole := mustRead(t, s, cycle, 0)
	rest := mustRead(t, s, cycle, cursors[1])
	last := mustRead(t, s, cycle, cursors[2])
	tail := mustRead(t, s, cycle, end)

	if want := []int64{0, cursors[1], cursors[2], cursors[2] + fileSize(t, lastSegment)}; !slices.Equal(bases, want) {
		t.Fatalf("segment bases = %v, want %v: one segment for each record, and an empty tail", bases, want)
	}
	if got := ids(whole); !slices.Equal(got, []string{"a", "b", "c"}) || whole.Next != end || len(whole.Gaps) != 0 {
		t.Fatalf("Read(0) = %v next %d gaps %v, want [a b c] across 3 segments, next %d", got, whole.Next, whole.Gaps, end)
	}
	for i, r := range whole.Records {
		if r.Cursor != cursors[i] {
			t.Fatalf("record %d has cursor %d, want the cursor %d that Append returned", i, r.Cursor, cursors[i])
		}
	}
	if got := ids(rest); !slices.Equal(got, []string{"b", "c"}) || ids(last)[0] != "c" || len(tail.Records) != 0 || tail.Next != end {
		t.Fatalf("Read from a later segment = %v, %v and %v next %d, want [b c], [c] and nothing", got, ids(last), ids(tail), tail.Next)
	}
}

func TestChannelStore_ARetentionGapIsAGapOfTheBatchAndNotARecord(t *testing.T) {
	t.Parallel()
	s, root := newStore(t, 1)
	cursors := appendAll(t, s, cycle, "a", "b", "c")
	if err := os.Remove(filepath.Join(root, "msg.5", firstSegment)); err != nil {
		t.Fatal(err)
	}

	b := mustRead(t, s, cycle, 0)

	want := []broker.Gap{{From: 0, To: cursors[1], Reason: channel.ReasonRetention}}
	if got := ids(b); !slices.Equal(got, []string{"b", "c"}) || !reflect.DeepEqual(b.Gaps, want) {
		t.Fatalf("Read(0) after gc = %v gaps %v, want [b c] and %v", got, b.Gaps, want)
	}
}

func TestChannelStore_AForeignOrBrokenRecordIsAMalformedGapWithItsSpan(t *testing.T) {
	t.Parallel()
	s, root := newStore(t, 1<<20)
	segment := filepath.Join(root, "msg.5", firstSegment)
	lg := rawLog(t, root, "msg.5")
	appendRaw := func(r channel.Record) int64 {
		c, err := lg.Append([]channel.Record{r})
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	signal := signalcenter.Event{SchemaVersion: signalcenter.SchemaVersion, Kind: signalcenter.KindLoopWave}
	c0 := appendAll(t, s, cycle, "good-1")[0]
	foreign := appendRaw(channel.Record{Source: "loop", Signal: &signal})
	stored := appendRaw(channel.Record{Source: "msg", Gap: &channel.Gap{Reason: channel.ReasonQueueFull}})
	broken := appendRaw(channel.Record{Source: "msg", Message: json.RawMessage(`"text"`)})
	junk := fileSize(t, segment)
	appendText(t, segment, "not json\n")
	good2 := appendAll(t, s, cycle, "good-2")[0]
	brokenLast := appendRaw(channel.Record{Source: "msg", Message: json.RawMessage(`[1]`)})
	end := fileSize(t, segment)

	b := mustRead(t, s, cycle, 0)

	malformed := func(from, to int64) broker.Gap {
		return broker.Gap{From: from, To: to, Reason: channel.ReasonMalformed}
	}
	wantGaps := []broker.Gap{
		malformed(foreign, stored),
		malformed(stored, broken),
		malformed(broken, junk),
		malformed(junk, good2),
		malformed(brokenLast, end),
	}
	if got := ids(b); !slices.Equal(got, []string{"good-1", "good-2"}) || b.Records[0].Cursor != c0 || b.Records[1].Cursor != good2 || b.Next != end {
		t.Fatalf("Read(0) = %v next %d, want only the two messages and next %d", got, b.Next, end)
	}
	if !reflect.DeepEqual(b.Gaps, wantGaps) {
		t.Fatalf("Read(0) gaps = %v, want %v: each record that is not a message is reported with the span it fills", b.Gaps, wantGaps)
	}
}

func TestChannelStore_DeadLettersGoToMsgDead(t *testing.T) {
	t.Parallel()
	s, root := newStore(t, 1<<20)
	first := broker.Dead{Envelope: record("a").Envelope, Recipient: identity.Principal{Cycle: 5, Phase: "audit"}, Reason: broker.ReasonExpired}
	second := broker.Dead{Envelope: record("b").Envelope, Recipient: identity.Principal{Cycle: 6, Phase: "build"}, Reason: broker.ReasonExpired}
	second.Envelope.ContextID, second.Envelope.From.Cycle = 6, 6

	err5 := s.DeadLetter(context.Background(), 5, first)
	err6 := s.DeadLetter(context.Background(), 6, second)

	if err5 != nil || err6 != nil {
		t.Fatalf("DeadLetter = %v, %v", err5, err6)
	}
	for name, want := range map[string]broker.Dead{"msg.dead.5": first, "msg.dead.6": second} {
		b, err := rawLog(t, root, name).Read(0)
		if err != nil || len(b.Records) != 1 {
			t.Fatalf("%s = %+v, %v, want exactly the one dead letter of its cycle", name, b, err)
		}
		rec := b.Records[0]
		if rec.Source != "msg" || rec.Dispatch != "r/5/scout/p1n1" || rec.Signal != nil || rec.Gap != nil {
			t.Fatalf("%s record = %+v, want a msg message record", name, rec)
		}
		if got, want := keys(t, rec.Message), []string{"envelope", "reason", "recipient"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("%s payload keys = %v, want %v", name, got, want)
		}
		var got broker.Dead
		if err := json.Unmarshal(rec.Message, &got); err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("%s letter = %+v, %v, want %+v", name, got, err, want)
		}
	}
	for _, name := range []string{"msg.5", "msg.6", "msg.dead"} {
		if _, err := os.Stat(filepath.Join(root, name)); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("a dead letter touched %s: %v", name, err)
		}
	}
}

func TestChannelStore_ACanceledContextFailsEachCallAndStoresNothing(t *testing.T) {
	t.Parallel()
	s, root := newStore(t, 1<<20)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, appendErr := s.Append(ctx, cycle, record("a"))
	_, readErr := s.Read(ctx, cycle, 0)
	deadErr := s.DeadLetter(ctx, cycle, broker.Dead{})

	for name, err := range map[string]error{"Append": appendErr, "Read": readErr, "DeadLetter": deadErr} {
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("%s error = %v, want context.Canceled", name, err)
		}
	}
	if _, err := os.Stat(root); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a canceled call touched the store: %v", err)
	}
}

func TestChannelStore_ARecordThatCannotBeEncodedIsRefusedAndWritesNothing(t *testing.T) {
	t.Parallel()
	s, root := newStore(t, 1<<20)
	bad := record("a")
	bad.Envelope.To = []address.Address{{}}

	_, appendErr := s.Append(context.Background(), cycle, bad)
	deadErr := s.DeadLetter(context.Background(), cycle, broker.Dead{Envelope: bad.Envelope})

	if !errors.Is(appendErr, address.ErrSyntax) || !errors.Is(deadErr, address.ErrSyntax) {
		t.Fatalf("errors = %v and %v, want address.ErrSyntax from each", appendErr, deadErr)
	}
	if _, err := os.Stat(root); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a refused record touched the store: %v", err)
	}
}

func TestChannelStore_AMessageLongerThanTheMessageLimitFailsAndIsNotStored(t *testing.T) {
	t.Parallel()
	s, _ := newStore(t, 1<<20)
	long := record("a")
	long.Envelope.Body = strings.Repeat("a", channel.MaxMessageBytes)

	_, err := s.Append(context.Background(), cycle, long)

	if !errors.Is(err, channel.ErrRecordTooLong) {
		t.Fatalf("Append of a %d byte body error = %v, want channel.ErrRecordTooLong", channel.MaxMessageBytes, err)
	}
	if b := mustRead(t, s, cycle, 0); len(b.Records) != 0 {
		t.Fatalf("a refused message was stored: %v", ids(b))
	}
}

func TestChannelStore_ABodyOfTheDefaultCapWithEveryByteEscapedFits(t *testing.T) {
	t.Parallel()
	s, _ := newStore(t, 1<<20)
	full := record("a")
	full.Envelope.Body = strings.Repeat("<", 8192)

	cursor, err := s.Append(context.Background(), cycle, full)

	if err != nil {
		t.Fatalf("Append of an 8192 byte body of escaped bytes error = %v, want it stored: the default body cap must fit one record", err)
	}
	b := mustRead(t, s, cycle, cursor)
	if len(b.Records) != 1 || b.Records[0].Envelope.Body != full.Envelope.Body {
		t.Fatalf("Read = %v, want the full body back", ids(b))
	}
}

func TestNew_RefusesAnIncompleteConfig(t *testing.T) {
	t.Parallel()
	good := testConfig(1)
	cases := map[string]struct {
		root string
		cfg  channel.Config
	}{
		"no root":             {"", good},
		"no segment size":     {"ch", channel.Config{LockDeadline: time.Minute}},
		"no lock deadline":    {"ch", channel.Config{SegmentBytes: 1}},
		"a negative deadline": {"ch", channel.Config{SegmentBytes: 1, LockDeadline: -time.Second}},
	}
	for name, c := range cases {
		if s, err := New(c.root, c.cfg); !errors.Is(err, ErrConfig) || s != nil {
			t.Fatalf("New with %s = %v, %v, want ErrConfig and no store", name, s, err)
		}
	}
	if s, err := New("ch", good); err != nil || s == nil {
		t.Fatalf("New with a full config = %v, %v", s, err)
	}
}
