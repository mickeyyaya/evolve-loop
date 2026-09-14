package observerengine

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
)

// The ops a tail fault names.
const (
	opStat = "stat"
	opOpen = "open"
	opSeek = "seek"
	opScan = "scan"
)

// tail reads the bytes appended to the stdout log since the last offset and
// returns them as lines plus the file's size. An absent log is silent (tmux
// drivers dump it at exit); every other fault is reported once per op and
// keeps the prior offset. Rotation (the file shrank below the offset)
// restarts from 0. The scanner's 10 MiB line bound is checked: a longer line
// stops the read, and the size is still returned (the preserved offset rule —
// the unterminated-line loss at that point is follow-up F4).
func (e *Engine) tail() ([]string, int64) {
	path := e.s.Paths.Stdout
	info, err := os.Stat(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			e.tailFault(opStat, err)
		}
		return nil, e.lastByteOff
	}
	if info.Size() < e.lastByteOff {
		e.lastByteOff = 0 // rotation; restart from 0
	}
	f, err := os.Open(path)
	if err != nil {
		e.tailFault(opOpen, err)
		return nil, e.lastByteOff
	}
	defer func() { _ = f.Close() }() // read-only handle; a close error carries no signal
	if _, err := f.Seek(e.lastByteOff, 0); err != nil {
		e.tailFault(opSeek, err)
		return nil, e.lastByteOff
	}
	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		e.tailFault(opScan, err)
	}
	return lines, info.Size()
}

// tailFault reports one stdout-tail fault per op for the observer's life.
func (e *Engine) tailFault(op string, err error) {
	e.faultOnce(originTick, CodeStdoutTailFailed, op, err.Error(), map[string]string{
		"step": "tail", "op": op, "path": e.s.Paths.Stdout,
	})
}
