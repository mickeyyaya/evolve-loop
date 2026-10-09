package signalcenter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"time"
)

type StreamChunk struct {
	Events  []Event
	Next    int64
	Skipped int
}

type LineChunk struct {
	Lines [][]byte
	Start int64
	Next  int64
}

func ReadLines(path string, from int64) (LineChunk, error) {
	chunk, err := splitLines(path, from)
	if err != nil {
		return LineChunk{Start: from, Next: from}, fmt.Errorf("signalcenter: read lines %s: %w", path, err)
	}
	return chunk, nil
}

func ReadStream(path string, from int64) (StreamChunk, error) {
	lines, err := splitLines(path, from)
	if err != nil {
		return StreamChunk{Next: from}, fmt.Errorf("signalcenter: read stream %s: %w", path, err)
	}
	chunk := StreamChunk{Next: lines.Next}
	for _, line := range lines.Lines {
		chunk.add(line)
	}
	return chunk, nil
}

func splitLines(path string, from int64) (LineChunk, error) {
	data, start, err := readStreamFrom(path, from)
	if err != nil {
		return LineChunk{}, err
	}
	end := bytes.LastIndexByte(data, '\n') + 1
	chunk := LineChunk{Start: start, Next: start + int64(end)}
	if end > 0 {
		chunk.Lines = bytes.Split(data[:end-1], []byte{'\n'})
	}
	return chunk, nil
}

func readStreamFrom(path string, from int64) ([]byte, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = f.Close() }()
	size, err := f.Seek(0, io.SeekEnd)
	start := from
	if start < 0 || start > size {
		start = 0
	}
	if err == nil {
		_, err = f.Seek(start, io.SeekStart)
	}
	var data []byte
	if err == nil {
		data, err = io.ReadAll(f)
	}
	return data, start, err
}

func (c *StreamChunk) add(line []byte) {
	if len(bytes.TrimSpace(line)) == 0 {
		return
	}
	var e Event
	if json.Unmarshal(line, &e) != nil {
		c.Skipped++
		return
	}
	if _, err := time.Parse(time.RFC3339Nano, e.TS); err != nil {
		c.Skipped++
		return
	}
	c.Events = append(c.Events, e)
}

type stampedEvent struct {
	at    time.Time
	valid bool
	event Event
}

func (s stampedEvent) before(o stampedEvent) bool {
	if s.valid != o.valid {
		return o.valid
	}
	return s.at.Before(o.at)
}

func MergeByTS(streams ...[]Event) []Event {
	var stamped []stampedEvent
	for _, stream := range streams {
		for _, e := range stream {
			at, err := time.Parse(time.RFC3339Nano, e.TS)
			stamped = append(stamped, stampedEvent{at: at, valid: err == nil, event: e})
		}
	}
	sort.SliceStable(stamped, func(i, j int) bool { return stamped[i].before(stamped[j]) })
	merged := make([]Event, len(stamped))
	for i, s := range stamped {
		merged[i] = s.event
	}
	return merged
}
