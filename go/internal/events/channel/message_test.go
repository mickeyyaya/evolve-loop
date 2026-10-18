package channel

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

const messagePayload = `{"envelope":{"id":"01J"},"recipients":[]}`

func messageRecord(payload string) Record {
	return Record{Source: "msg", Dispatch: "r/5/scout/p1n1", Message: json.RawMessage(payload)}
}

func payloadOf(r Record) string {
	switch {
	case r.Message != nil:
		return "message"
	case r.Gap != nil:
		return "gap"
	}
	return "signal"
}

func TestRecord_HoldsExactlyOneOfSignalGapAndMessage(t *testing.T) {
	t.Parallel()
	signal := signalRecord(1, "s").Signal
	gap := &Gap{Reason: ReasonQueueFull}
	message := json.RawMessage(messagePayload)
	cases := []struct {
		name string
		rec  Record
		ok   bool
	}{
		{"a signal", Record{Source: "loop", Signal: signal}, true},
		{"a gap", Record{Source: "loop", Gap: gap}, true},
		{"a message", Record{Source: "msg", Message: message}, true},
		{"none of the three", Record{Source: "msg"}, false},
		{"a signal and a gap", Record{Source: "loop", Signal: signal, Gap: gap}, false},
		{"a signal and a message", Record{Source: "msg", Signal: signal, Message: message}, false},
		{"a gap and a message", Record{Source: "msg", Gap: gap, Message: message}, false},
		{"all three", Record{Source: "msg", Signal: signal, Gap: gap, Message: message}, false},
		{"a null message", Record{Source: "msg", Message: json.RawMessage("null")}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			l, root := newTestLog(t, 1<<20)
			_, appendErr := l.Append([]Record{c.rec})
			if c.ok != (appendErr == nil) || (!c.ok && !errors.Is(appendErr, ErrInvalidRecord)) {
				t.Fatalf("Append of %s = %v, want ok=%v and ErrInvalidRecord when refused", c.name, appendErr, c.ok)
			}
			if !c.ok {
				if _, err := os.Stat(filepath.Join(root, testChannel)); !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("a refused record touched the channel: %v", err)
				}
			}
			line := recordLine(t, c.rec)
			writeSegment(t, root, 0, line)
			b := mustRead(t, l, 0)
			if c.ok && (len(b.Records) != 1 || b.Records[0].Cursor != 0 || b.Next != int64(len(line))) {
				t.Fatalf("Read of %s = %+v, want the one record", c.name, b)
			}
			if !c.ok && (len(b.Records) != 1 || b.Records[0].Source != SourceWatch || b.Records[0].Gap.Reason != ReasonMalformed) {
				t.Fatalf("Read of a line with %s = %+v, want one malformed gap", c.name, b)
			}
		})
	}
}

func TestRecord_AMessageKeepsItsPayloadBytesAcrossAppendAndRead(t *testing.T) {
	t.Parallel()
	l, root := newTestLog(t, 1<<20)

	cursor := mustAppend(t, l, messageRecord(messagePayload))
	b := mustRead(t, l, cursor)

	want := `{"source":"msg","dispatch":"r/5/scout/p1n1","message":` + messagePayload + "}\n"
	raw, err := os.ReadFile(segmentPath(root, 0))
	if err != nil || string(raw) != want {
		t.Fatalf("segment = %q, %v, want %q", raw, err, want)
	}
	got := b.Records[0]
	if got.Source != "msg" || got.Dispatch != "r/5/scout/p1n1" || string(got.Message) != messagePayload || got.Signal != nil || got.Gap != nil {
		t.Fatalf("Read = %+v, want the message record as written", got)
	}
}

func TestAppend_AMessageRecordHasItsOwnLineLimit(t *testing.T) {
	t.Parallel()
	l, root := newTestLog(t, 1<<20)
	base := recordLine(t, messageRecord(`{"body":""}`))
	pad := MaxMessageBytes - (len(base) - 1)
	atLimit := messageRecord(`{"body":"` + strings.Repeat("a", pad) + `"}`)
	overLimit := messageRecord(`{"body":"` + strings.Repeat("a", pad+1) + `"}`)

	_, overErr := l.Append([]Record{overLimit})
	if _, err := os.Stat(filepath.Join(root, testChannel)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a refused batch touched the channel: %v", err)
	}
	cursor, atErr := l.Append([]Record{atLimit})

	if !errors.Is(overErr, ErrRecordTooLong) || !strings.Contains(overErr.Error(), "channel loop") {
		t.Fatalf("Append of a message line of %d bytes = %v, want ErrRecordTooLong naming the channel", MaxMessageBytes+1, overErr)
	}
	if atErr != nil || cursor != 0 || len(recordLine(t, atLimit)) != MaxMessageBytes+1 {
		t.Fatalf("Append of a message line of exactly %d bytes = %v, want it written", MaxMessageBytes, atErr)
	}
	if MaxMessageBytes != 65536 || MaxMessageBytes <= MaxRecordBytes {
		t.Fatalf("MaxMessageBytes = %d, want 65536: more than a signal line, and room for the default 8192-byte body with every byte escaped to six", MaxMessageBytes)
	}
}

func TestAppend_ASignalRecordKeepsTheSignalLineLimit(t *testing.T) {
	t.Parallel()
	l, _ := newTestLog(t, 1<<20)
	base := recordLine(t, signalRecord(1, ""))
	over := signalRecord(1, strings.Repeat("a", MaxRecordBytes-(len(base)-1)+1))

	_, err := l.Append([]Record{over})

	if !errors.Is(err, ErrRecordTooLong) {
		t.Fatalf("Append of a signal line of %d bytes = %v, want ErrRecordTooLong: the message limit is not for signals", MaxRecordBytes+1, err)
	}
}

func TestRead_AMessageRecordIsInTheCursorOrderWithItsNeighbours(t *testing.T) {
	t.Parallel()
	l, _ := newTestLog(t, 1<<20)
	first := mustAppend(t, l, Record{Source: "msg", Gap: &Gap{Reason: ReasonWriteError}})
	second := mustAppend(t, l, messageRecord(messagePayload))

	b := mustRead(t, l, first)

	kinds := make([]string, len(b.Records))
	for i, r := range b.Records {
		kinds[i] = payloadOf(r)
	}
	if want := []string{"gap", "message"}; !reflect.DeepEqual(kinds, want) || b.Records[1].Cursor != second {
		t.Fatalf("Read = %q with the message at %d, want %q with the message at %d", kinds, b.Records[1].Cursor, want, second)
	}
}
