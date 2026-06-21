package main

// cmd_fleet_logprefix.go — per-cycle log attribution (Decorator). Concurrent
// fleet/campaign cycles share one stdout/stderr; without this their output
// byte-interleaves into an unreadable stream. prefixLineWriter line-buffers each
// cycle's output, tags every complete line with the cycle's scope, and serializes
// writes through one shared mutex so lines from different cycles never tear.

import (
	"bytes"
	"io"
	"sync"
)

type prefixLineWriter struct {
	w      io.Writer
	prefix string
	mu     *sync.Mutex // shared across all cycles writing to the same sink
	buf    []byte      // holds a partial (not-yet-newline-terminated) line
}

// Write buffers input and emits each complete line prefixed, under the shared
// mutex. A partial trailing line stays buffered until the next newline or Flush.
func (p *prefixLineWriter) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.buf = append(p.buf, b...)
	for {
		i := bytes.IndexByte(p.buf, '\n')
		if i < 0 {
			break
		}
		if err := p.emitLocked(p.buf[:i+1]); err != nil {
			return 0, err
		}
		p.buf = p.buf[i+1:]
	}
	return len(b), nil
}

// Flush emits any buffered partial line (adding a trailing newline) so nothing is
// lost when a cycle exits without a final newline.
func (p *prefixLineWriter) Flush() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.buf) == 0 {
		return
	}
	_ = p.emitLocked(append(p.buf, '\n'))
	p.buf = nil
}

// emitLocked writes prefix+line; caller holds p.mu.
func (p *prefixLineWriter) emitLocked(line []byte) error {
	if _, err := io.WriteString(p.w, p.prefix); err != nil {
		return err
	}
	_, err := p.w.Write(line)
	return err
}
