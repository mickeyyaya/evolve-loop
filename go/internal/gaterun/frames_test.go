package gaterun

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
)

func readerOf(s string) *bufio.Reader { return bufio.NewReader(strings.NewReader(s)) }

func TestWriteFrameAndReadFrame_RoundTripEachKindOnOneLine(t *testing.T) {
	t.Parallel()
	frames := []Frame{
		{Kind: KindInput, Input: &gatecontract.Input{Root: "r", Base: "b"}},
		{Kind: KindRequest, ID: 3, Method: MethodTrackedTree, Params: []byte(`{"root":"r"}`)},
		{Kind: KindResponse, ID: 3, Result: []byte(`["a"]`)},
		{Kind: KindVerdict, Verdict: &gatecontract.Verdict{Reds: []string{"x"}, Notes: []string{"n"}}},
		{Kind: KindError, Message: "boom"},
	}
	var buf bytes.Buffer
	for _, frame := range frames {
		if err := WriteFrame(&buf, frame); err != nil {
			t.Fatal(err)
		}
	}
	if got := strings.Count(buf.String(), "\n"); got != len(frames) {
		t.Fatalf("lines = %d, want %d", got, len(frames))
	}
	reader := bufio.NewReader(&buf)
	for _, want := range frames {
		got, err := ReadFrame(reader)
		if err != nil || got.Kind != want.Kind || got.ID != want.ID || got.Method != want.Method || got.Message != want.Message {
			t.Errorf("ReadFrame = %+v, %v, want %+v", got, err, want)
		}
	}
	if _, err := ReadFrame(reader); !errors.Is(err, io.EOF) {
		t.Errorf("ReadFrame at the end = %v, want io.EOF", err)
	}
}

func TestReadFrame_AMalformedFrameIsAnError(t *testing.T) {
	t.Parallel()
	for name, line := range map[string]string{
		"not json":         "garbage\n",
		"unknown kind":     `{"kind":"other"}` + "\n",
		"no kind":          `{}` + "\n",
		"input no input":   `{"kind":"input"}` + "\n",
		"verdict no body":  `{"kind":"verdict"}` + "\n",
		"request no name":  `{"kind":"request","id":1}` + "\n",
		"request no id":    `{"kind":"request","method":"m"}` + "\n",
		"response no id":   `{"kind":"response"}` + "\n",
		"error no message": `{"kind":"error"}` + "\n",
	} {
		if frame, err := ReadFrame(readerOf(line)); err == nil || errors.Is(err, io.EOF) {
			t.Errorf("%s: ReadFrame = %+v, %v, want a malformed-frame error", name, frame, err)
		}
	}
}

func TestReadFrame_ALastFrameWithNoNewlineIsRead(t *testing.T) {
	t.Parallel()
	frame, err := ReadFrame(readerOf(`{"kind":"error","message":"m"}`))
	if err != nil || frame.Message != "m" {
		t.Errorf("ReadFrame = %+v, %v", frame, err)
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }

func TestWriteFrame_AFailedWriteIsAnError(t *testing.T) {
	t.Parallel()
	if err := WriteFrame(failWriter{}, Frame{Kind: KindError, Message: "m"}); err == nil {
		t.Error("WriteFrame to a closed writer = nil, want an error")
	}
}

type failReader struct{}

func (failReader) Read([]byte) (int, error) { return 0, errors.New("broken pipe") }

func TestReadFrame_AFailedReadIsAnErrorThatIsNotEOF(t *testing.T) {
	t.Parallel()
	if _, err := ReadFrame(bufio.NewReader(failReader{})); err == nil || errors.Is(err, io.EOF) {
		t.Errorf("ReadFrame = %v, want a read error", err)
	}
}

func TestWriteFrame_AFrameThatCannotBeEncodedIsAnError(t *testing.T) {
	t.Parallel()
	if err := WriteFrame(io.Discard, Frame{Kind: KindRequest, Params: []byte("{bad")}); err == nil {
		t.Error("WriteFrame with invalid params = nil, want an error")
	}
}

func TestReadFrame_ALineOverTheLimitIsAnErrorAndALineAtTheLimitIsRead(t *testing.T) {
	t.Parallel()
	over := `{"kind":"error","message":"` + strings.Repeat("a", MaxFrameBytes) + `"}` + "\n"
	if _, err := ReadFrame(readerOf(over)); err == nil || errors.Is(err, io.EOF) || !strings.Contains(err.Error(), "limit") {
		t.Errorf("ReadFrame over the limit = %v, want a limit error", err)
	}
	head, tail := `{"kind":"error","message":"`, `"}`+"\n"
	fits := head + strings.Repeat("a", MaxFrameBytes-len(head)-len(tail)) + tail
	if len(fits) != MaxFrameBytes {
		t.Fatalf("len = %d", len(fits))
	}
	if frame, err := ReadFrame(readerOf(fits)); err != nil || frame.Kind != KindError {
		t.Errorf("ReadFrame at the limit = %v", err)
	}
}
