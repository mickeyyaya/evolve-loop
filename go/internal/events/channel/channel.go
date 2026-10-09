package channel

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync/atomic"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
)

type Config struct {
	SegmentBytes int64
	LockDeadline time.Duration
}

type segmentFile interface {
	io.Writer
	io.ReaderAt
	Stat() (os.FileInfo, error)
	Close() error
}

type Log struct {
	name     string
	dir      string
	lockPath string
	cfg      Config
	open     func(path string, flag int, perm os.FileMode) (segmentFile, error)
	lock     func(path string, wait time.Duration, onSettled func()) (func(), error)
	marshal  func(v any) ([]byte, error)
	pending  atomic.Bool
}

var channelName = regexp.MustCompile(`^[a-z0-9_-]+(\.[a-z0-9_-]+)*$`)

func New(root, name string, cfg Config) (*Log, error) {
	if !channelName.MatchString(name) {
		return nil, fmt.Errorf("channel: name %q is not dot-separated tokens of [a-z0-9_-]", name)
	}
	return &Log{
		name:     name,
		dir:      filepath.Join(root, name),
		lockPath: filepath.Join(root, name+".lock"),
		cfg:      cfg,
		open:     openSegment,
		lock:     flock.LockWithin,
		marshal:  json.Marshal,
	}, nil
}

func (l *Log) Dir() string { return l.dir }

func openSegment(path string, flag int, perm os.FileMode) (segmentFile, error) {
	f, err := os.OpenFile(path, flag, perm)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (l *Log) errorf(format string, args ...any) error {
	return fmt.Errorf("channel "+l.name+": "+format, args...)
}
