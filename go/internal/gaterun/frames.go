package gaterun

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
)

const (
	KindInput    = "input"
	KindRequest  = "request"
	KindResponse = "response"
	KindVerdict  = "verdict"
	KindError    = "error"
)

type Frame struct {
	Kind    string                `json:"kind"`
	ID      int                   `json:"id,omitempty"`
	Method  string                `json:"method,omitempty"`
	Params  json.RawMessage       `json:"params,omitempty"`
	Result  json.RawMessage       `json:"result,omitempty"`
	Input   *gatecontract.Input   `json:"input,omitempty"`
	Verdict *gatecontract.Verdict `json:"verdict,omitempty"`
	Message string                `json:"message,omitempty"`
}

func WriteFrame(w io.Writer, frame Frame) error {
	raw, err := json.Marshal(frame)
	if err != nil {
		return fmt.Errorf("encode a %s frame: %w", frame.Kind, err)
	}
	if _, err := w.Write(append(raw, '\n')); err != nil {
		return fmt.Errorf("write a %s frame: %w", frame.Kind, err)
	}
	return nil
}

const MaxFrameBytes = 4 << 20

func readLine(r *bufio.Reader) ([]byte, error) {
	var line []byte
	for {
		chunk, err := r.ReadSlice('\n')
		line = append(line, chunk...)
		if len(line) > MaxFrameBytes {
			return nil, fmt.Errorf("a frame is over the limit of %d bytes", MaxFrameBytes)
		}
		if !errors.Is(err, bufio.ErrBufferFull) {
			return line, err
		}
	}
}

func ReadFrame(r *bufio.Reader) (Frame, error) {
	line, err := readLine(r)
	if len(line) == 0 && errors.Is(err, io.EOF) {
		return Frame{}, io.EOF
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return Frame{}, fmt.Errorf("read a frame: %w", err)
	}
	var frame Frame
	if err := json.Unmarshal(line, &frame); err != nil {
		return Frame{}, fmt.Errorf("malformed frame %q: %w", line, err)
	}
	if problem := frameProblem(frame); problem != "" {
		return Frame{}, fmt.Errorf("malformed frame %q: %s", line, problem)
	}
	return frame, nil
}

func frameProblem(frame Frame) string {
	switch {
	case frame.Kind == KindInput && frame.Input == nil:
		return "an input frame holds no input"
	case frame.Kind == KindVerdict && frame.Verdict == nil:
		return "a verdict frame holds no verdict"
	case frame.Kind == KindRequest && (frame.Method == "" || frame.ID == 0):
		return "a request frame holds no method or no id"
	case frame.Kind == KindResponse && frame.ID == 0:
		return "a response frame holds no id"
	case frame.Kind == KindError && frame.Message == "":
		return "an error frame holds no message"
	case frame.Kind != KindInput && frame.Kind != KindVerdict && frame.Kind != KindRequest && frame.Kind != KindResponse && frame.Kind != KindError:
		return fmt.Sprintf("unknown kind %q", frame.Kind)
	}
	return ""
}
