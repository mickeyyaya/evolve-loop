package cyclespend

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"time"
)

type Sampler struct {
	load  func() float64
	now   func() time.Time
	spans []GateSpan
}

func NewSampler(load func() float64, now func() time.Time) *Sampler {
	return &Sampler{load: load, now: now}
}

func (s *Sampler) Measure(gate string, run func() error) error {
	start := s.now()
	err := run()
	end := s.now()
	s.spans = append(s.spans, GateSpan{
		Gate:  gate,
		Start: start,
		End:   end,
		Load:  s.load(),
	})
	return err
}

func (s *Sampler) Spans() []GateSpan {
	return slices.Clone(s.spans)
}

func (s *Sampler) Flush(workspace string) error {
	if len(s.spans) == 0 {
		return nil
	}
	var buf bytes.Buffer
	for _, sp := range s.spans {
		b, err := json.Marshal(sp)
		if err != nil {
			return err
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	f, err := os.OpenFile(filepath.Join(workspace, GateSpansFile), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	_, writeErr := f.Write(buf.Bytes())
	closeErr := f.Close()
	err = errors.Join(writeErr, closeErr)
	if err == nil {
		s.spans = nil
	}
	return err
}
