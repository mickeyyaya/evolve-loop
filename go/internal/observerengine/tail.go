package observerengine

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
)

const (
	opStat = "stat"
	opOpen = "open"
	opSeek = "seek"
	opScan = "scan"
)

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
		e.lastByteOff = 0
	}
	f, err := os.Open(path)
	if err != nil {
		e.tailFault(opOpen, err)
		return nil, e.lastByteOff
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Seek(e.lastByteOff, 0); err != nil {
		e.tailFault(opSeek, err)
		return nil, e.lastByteOff
	}
	var lines []string
	scanner := bufio.NewScanner(f)
	const maxLineBytes = 10 * 1024 * 1024
	scanner.Buffer(make([]byte, 1024*1024), maxLineBytes)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		e.tailFault(opScan, err)
	}
	return lines, info.Size()
}

func (e *Engine) tailFault(op string, err error) {
	e.faultOnce(originTick, CodeStdoutTailFailed, op, err.Error(), map[string]string{
		"step": "tail", "op": op, "path": e.s.Paths.Stdout,
	})
}
