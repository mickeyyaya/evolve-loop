package channel

import (
	"encoding/json"
	"errors"
	"io/fs"
	"math"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type reading struct {
	cur     int64
	budget  int64
	used    int64
	full    bool
	records []Record
}

func (l *Log) Read(from int64) (Batch, error) {
	return l.ReadN(from, math.MaxInt64)
}

func (l *Log) ReadN(from, maxBytes int64) (Batch, error) {
	segs, err := l.Segments()
	if err != nil {
		return Batch{Next: from}, err
	}
	return l.readFrom(segs, from, maxBytes)
}

func (l *Log) readFrom(segs []Segment, from, maxBytes int64) (Batch, error) {
	r := &reading{cur: from, budget: maxBytes}
	if len(segs) == 0 {
		r.moveTo(ReasonReset, 0)
		return r.batch(), nil
	}
	if from < segs[0].Base {
		r.moveTo(ReasonRetention, segs[0].Base)
	}
	for i := segmentAt(segs, r.cur); i < len(segs) && !r.full; i++ {
		if err := l.readSegment(r, segs[i], segs[i+1:]); err != nil {
			return Batch{Next: from}, err
		}
	}
	return r.batch(), nil
}

func (l *Log) readSegment(r *reading, s Segment, later []Segment) error {
	off := r.cur - s.Base
	chunk, err := signalcenter.ReadLines(s.Path, off)
	if errors.Is(err, fs.ErrNotExist) && len(later) > 0 {
		r.moveTo(ReasonRetention, later[0].Base)
		return nil
	}
	if err != nil {
		return l.errorf("read segment: %w", err)
	}
	if chunk.Start != off {
		r.moveTo(ReasonReset, endOf(s, chunk, later))
		return nil
	}
	for _, line := range chunk.Lines {
		if !r.take(line) {
			return nil
		}
	}
	if len(later) > 0 {
		r.moveTo(ReasonReset, later[0].Base)
	}
	return nil
}

func endOf(s Segment, chunk signalcenter.LineChunk, later []Segment) int64 {
	if len(later) > 0 {
		return later[0].Base
	}
	return s.Base + chunk.Next
}

func (r *reading) moveTo(reason string, to int64) {
	if r.cur == to {
		return
	}
	r.records = append(r.records, Record{Source: SourceWatch, Gap: &Gap{Reason: reason, From: r.cur, To: to}})
	r.cur = to
}

func (r *reading) take(line []byte) bool {
	size := int64(len(line)) + 1
	if r.used > 0 && r.used+size > r.budget {
		r.full = true
		return false
	}
	r.used += size
	at := r.cur
	rec, ok := parseRecord(line)
	if !ok {
		r.moveTo(ReasonMalformed, at+size)
		return true
	}
	rec.Cursor = at
	r.records = append(r.records, rec)
	r.cur = at + size
	return true
}

func (r *reading) batch() Batch {
	return Batch{Records: r.records, Next: r.cur}
}

func parseRecord(line []byte) (Record, bool) {
	var rec Record
	if json.Unmarshal(line, &rec) != nil || !rec.hasOnePayload() {
		return Record{}, false
	}
	return rec, true
}

func (l *Log) Last() (int64, error) {
	segs, err := l.Segments()
	if err != nil {
		return 0, err
	}
	end := int64(0)
	for i := len(segs) - 1; i >= 0 && i >= len(segs)-2; i-- {
		chunk, err := l.readAll(segs[i])
		if err != nil {
			return 0, err
		}
		end = max(end, segs[i].Base+chunk.Next)
		if n := len(chunk.Lines); n > 0 {
			return segs[i].Base + chunk.Next - int64(len(chunk.Lines[n-1])+1), nil
		}
	}
	return end, nil
}

func (l *Log) End() (int64, error) {
	segs, err := l.Segments()
	if err != nil || len(segs) == 0 {
		return 0, err
	}
	tail := segs[len(segs)-1]
	chunk, err := l.readAll(tail)
	return tail.Base + chunk.Next, err
}

func (l *Log) readAll(s Segment) (signalcenter.LineChunk, error) {
	chunk, err := signalcenter.ReadLines(s.Path, 0)
	if err != nil {
		return chunk, l.errorf("read segment: %w", err)
	}
	return chunk, nil
}
