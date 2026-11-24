// Package ledgerscan streams ledger.jsonl lines newest first in chunks, with a
// raw-byte prefilter and early exit. It only reads.
// See docs/architecture/packages/internal-ledgerscan.md.
package ledgerscan

import (
	"bytes"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

const chunkSize = 64 << 10

type Filter struct{ Needles [][]byte }

func (f Filter) Pass(line []byte) bool {
	if len(f.Needles) == 0 || bytes.IndexByte(line, '\\') >= 0 {
		return true
	}
	for _, n := range f.Needles {
		if !bytes.Contains(line, n) {
			return false
		}
	}
	return true
}

func ContainsValue(v string) []byte {
	if v == "" || strings.ContainsRune(v, utf8.RuneError) {
		return nil
	}
	return []byte(v)
}

func Reverse(path string, f Filter, visit func(line []byte) (stop bool, err error)) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	return scanFile(file, f, visit)
}

func scanFile(file *os.File, f Filter, visit func([]byte) (bool, error)) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	return reverse(file, info.Size(), lineSink{f: f, visit: visit}, chunkSize)
}

type lineSink struct {
	f     Filter
	visit func([]byte) (bool, error)
}

func reverse(r io.ReaderAt, size int64, sink lineSink, chunk int) error {
	var carry []byte
	pos := size
	for pos > 0 {
		n := int64(chunk)
		if n > pos {
			n = pos
		}
		pos -= n
		buf := make([]byte, int(n)+len(carry))
		if _, err := r.ReadAt(buf[:n], pos); err != nil && err != io.EOF {
			return err
		}
		copy(buf[n:], carry)
		stop, rest, err := sink.emitComplete(buf)
		if stop || err != nil {
			return err
		}
		carry = rest
	}
	_, err := sink.emit(carry)
	return err
}

func (s lineSink) emitComplete(buf []byte) (stop bool, rest []byte, err error) {
	end := len(buf)
	for {
		i := bytes.LastIndexByte(buf[:end], '\n')
		if i < 0 {
			return false, buf[:end], nil
		}
		if stop, err = s.emit(buf[i+1 : end]); stop || err != nil {
			return true, nil, err
		}
		end = i
	}
}

func (s lineSink) emit(line []byte) (bool, error) {
	if len(line) == 0 || !s.f.Pass(line) {
		return false, nil
	}
	return s.visit(line)
}
