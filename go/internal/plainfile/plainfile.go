package plainfile

import (
	"errors"
	"io"
	"os"
	"syscall"

	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
)

var ErrNotPlain = errors.New("the path is not a regular file, so the pipeline does not read it")

var ErrTooLarge = errors.New("the file is larger than the size limit, so the pipeline does not read it")

func Open(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0)
}

func Read(path string, limit int64) ([]byte, error) {
	return ReadWith(path, limit, Open)
}

func ReadWith(path string, limit int64, open func(string) (*os.File, error)) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || inputindex.HardLinked(before) {
		return nil, ErrNotPlain
	}
	f, err := open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	after, err := f.Stat()
	if err != nil || !os.SameFile(before, after) {
		return nil, ErrNotPlain
	}
	if after.Size() > limit {
		return nil, ErrTooLarge
	}
	return io.ReadAll(io.LimitReader(f, limit))
}
