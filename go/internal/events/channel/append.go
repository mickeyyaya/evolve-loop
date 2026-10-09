package channel

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
)

var (
	ErrLockPending = fmt.Errorf("channel: a lock call of this process still waits: %w", flock.ErrLockDeadline)
	ErrRotate      = errors.New("channel: the batch is on disk, but the next segment was not made")
)

func (l *Log) Append(records []Record) (int64, error) {
	batch, err := l.encode(records)
	if err != nil {
		return 0, err
	}
	if !l.pending.CompareAndSwap(false, true) {
		return 0, l.errorf("lock: %w", ErrLockPending)
	}
	release, err := l.lock(l.lockPath, l.cfg.LockDeadline, func() { l.pending.Store(false) })
	if err != nil {
		return 0, l.errorf("lock: %w", err)
	}
	defer release()
	return l.appendLocked(batch)
}

func (l *Log) encode(records []Record) ([]byte, error) {
	var buf bytes.Buffer
	for _, r := range records {
		if !r.hasOnePayload() || r.Source == SourceWatch {
			return nil, l.errorf("encode: %w", ErrInvalidRecord)
		}
		line, err := l.marshal(r)
		if err != nil {
			return nil, l.errorf("encode: %w", err)
		}
		if len(line) > MaxRecordBytes {
			return nil, l.errorf("%d bytes: %w", len(line), ErrRecordTooLong)
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

func (l *Log) appendLocked(batch []byte) (int64, error) {
	if err := os.MkdirAll(l.dir, 0o755); err != nil {
		return 0, l.errorf("make directory: %w", err)
	}
	segs, err := l.Segments()
	if err != nil {
		return 0, err
	}
	tail := l.tail(segs)
	f, err := l.open(tail.Path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return 0, l.errorf("open tail: %w", err)
	}
	defer func() { _ = f.Close() }()
	size, repair, err := l.tailState(tail.Path, f)
	if err != nil {
		return 0, err
	}
	if _, err := f.Write(append(repair, batch...)); err != nil {
		return 0, l.errorf("write batch: %w", err)
	}
	size += int64(len(repair))
	return tail.Base + size, l.rotate(tail.Base, size+int64(len(batch)))
}

func (l *Log) tailState(path string, f segmentFile) (int64, []byte, error) {
	info, err := f.Stat()
	if err != nil {
		return 0, nil, l.errorf("stat tail: %w", err)
	}
	size := info.Size()
	if size == 0 {
		return 0, nil, nil
	}
	last, err := l.lastByte(path, size)
	if err != nil || last == '\n' {
		return size, nil, err
	}
	return size, []byte{'\n'}, nil
}

func (l *Log) lastByte(path string, size int64) (byte, error) {
	r, err := l.open(path, os.O_RDONLY, 0)
	if err != nil {
		return 0, l.errorf("open tail to read: %w", err)
	}
	defer func() { _ = r.Close() }()
	b := make([]byte, 1)
	if _, err := r.ReadAt(b, size-1); err != nil {
		return 0, l.errorf("read last byte: %w", err)
	}
	return b[0], nil
}

func (l *Log) rotate(base, size int64) error {
	if size <= l.cfg.SegmentBytes {
		return nil
	}
	next, err := l.open(filepath.Join(l.dir, segmentName(base+size)), os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return l.errorf("%w: %w", ErrRotate, err)
	}
	return next.Close()
}
