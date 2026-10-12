package main

import (
	"bytes"
	"io"
	"sync"
)

type prefixLineWriter struct {
	w      io.Writer
	prefix string
	mu     *sync.Mutex
	buf    []byte
}

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

func (p *prefixLineWriter) Flush() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.buf) == 0 {
		return
	}
	_ = p.emitLocked(append(p.buf, '\n'))
	p.buf = nil
}

func (p *prefixLineWriter) emitLocked(line []byte) error {
	if _, err := io.WriteString(p.w, p.prefix); err != nil {
		return err
	}
	_, err := p.w.Write(line)
	return err
}
