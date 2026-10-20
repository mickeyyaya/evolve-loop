// Package shipwindow is the lease that serializes the audit-binding-to-ship-push window across concurrent lanes.
// See docs/architecture/packages/internal-shipwindow.md.
package shipwindow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

const FileName = "ship-window.lock"

const DefaultTTL = 5 * time.Minute

const (
	queueDirName = "ship-window.queue"
	defaultPoll  = 500 * time.Millisecond
)

var ticketSeq uint64

type Options struct {
	TTL      time.Duration
	Now      func() time.Time
	Alive    func(pid int) bool
	Poll     time.Duration
	OwnerPID int
}

func (o Options) normalized() Options {
	if o.TTL <= 0 {
		o.TTL = DefaultTTL
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Alive == nil {
		o.Alive = pidAlive
	}
	if o.Poll <= 0 {
		o.Poll = defaultPoll
	}
	if o.OwnerPID == 0 {
		o.OwnerPID = os.Getpid()
	}
	return o
}

type record struct {
	OwnerPID    int    `json:"owner_pid"`
	HeartbeatAt string `json:"heartbeat_at"`
	Token       string `json:"token"`
}

type Lease struct {
	path  string
	token string
}

func PathIn(evolveDir string) string {
	return filepath.Join(evolveDir, FileName)
}

func Acquire(ctx context.Context, evolveDir string, opts Options) (*Lease, error) {
	o := opts.normalized()
	queueDir := filepath.Join(evolveDir, queueDirName)
	if err := os.MkdirAll(queueDir, 0o755); err != nil {
		return nil, fmt.Errorf("shipwindow: queue dir: %w", err)
	}
	ticket := fmt.Sprintf("%020d-%09d-%d", o.Now().UnixNano(), atomic.AddUint64(&ticketSeq, 1), o.OwnerPID)
	ticketPath := filepath.Join(queueDir, ticket)
	if err := os.WriteFile(ticketPath, []byte(strconv.Itoa(o.OwnerPID)+"\n"), 0o644); err != nil {
		return nil, fmt.Errorf("shipwindow: enqueue: %w", err)
	}
	for {
		l, err := tryAcquire(evolveDir, queueDir, ticket, o)
		if err != nil || l != nil {
			_ = os.Remove(ticketPath)
			return l, err
		}
		select {
		case <-ctx.Done():
			_ = os.Remove(ticketPath)
			return nil, ctx.Err()
		case <-time.After(o.Poll):
		}
	}
}

func tryAcquire(evolveDir, queueDir, ticket string, o Options) (*Lease, error) {
	head, err := queueHead(queueDir, ticket, o.Alive)
	if err != nil {
		return nil, err
	}
	if head != ticket {
		return nil, nil
	}
	lockPath := PathIn(evolveDir)
	rec, ok, err := readRecord(lockPath)
	if err != nil && !ok {
		return nil, fmt.Errorf("shipwindow: read lease: %w", err)
	}
	if err != nil {
		rec = record{}
	}
	if ok {
		if runlease.OwnerLive(runlease.Lease{OwnerPID: rec.OwnerPID, HeartbeatAt: rec.HeartbeatAt}, o.Now(), o.TTL, o.Alive) {
			return nil, nil
		}
		breakPath := lockPath + ".breaking." + ticket
		if rerr := os.Rename(lockPath, breakPath); rerr == nil {
			_ = os.Remove(breakPath)
		} else if !errors.Is(rerr, os.ErrNotExist) {
			return nil, fmt.Errorf("shipwindow: break stale lease: %w", rerr)
		}
	}
	b, err := json.Marshal(record{
		OwnerPID:    o.OwnerPID,
		HeartbeatAt: o.Now().UTC().Format(time.RFC3339Nano),
		Token:       ticket,
	})
	if err != nil {
		return nil, fmt.Errorf("shipwindow: marshal: %w", err)
	}
	tmp := lockPath + ".tmp." + ticket
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return nil, fmt.Errorf("shipwindow: stage lease: %w", err)
	}
	linkErr := os.Link(tmp, lockPath)
	_ = os.Remove(tmp)
	if linkErr != nil {
		if errors.Is(linkErr, os.ErrExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("shipwindow: create lease: %w", linkErr)
	}
	return &Lease{path: lockPath, token: ticket}, nil
}

func queueHead(queueDir, own string, alive func(int) bool) (string, error) {
	entries, err := os.ReadDir(queueDir)
	if err != nil {
		return "", fmt.Errorf("shipwindow: read queue: %w", err)
	}
	for _, e := range entries {
		name := e.Name()
		if name == own {
			return own, nil
		}
		if pid := ticketPID(name); pid > 0 && !alive(pid) {
			_ = os.Remove(filepath.Join(queueDir, name))
			continue
		}
		return name, nil
	}
	return "", fmt.Errorf("shipwindow: own ticket %s missing from queue", own)
}

func ticketPID(name string) int {
	i := strings.LastIndexByte(name, '-')
	if i < 0 {
		return 0
	}
	pid, err := strconv.Atoi(name[i+1:])
	if err != nil {
		return 0
	}
	return pid
}

func readRecord(path string) (record, bool, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return record{}, false, nil
	}
	if err != nil {
		return record{}, false, err
	}
	var r record
	if jerr := json.Unmarshal(raw, &r); jerr != nil {
		return record{}, true, fmt.Errorf("shipwindow: parse %s: %w", path, jerr)
	}
	return r, true, nil
}

func (l *Lease) Release() error {
	rec, ok, err := readRecord(l.path)
	if err != nil && !ok {
		return fmt.Errorf("shipwindow: release read: %w", err)
	}
	if err != nil || !ok || rec.Token != l.token {
		return nil
	}
	if rerr := os.Remove(l.path); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
		return fmt.Errorf("shipwindow: release: %w", rerr)
	}
	return nil
}

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	serr := p.Signal(syscall.Signal(0))
	return serr == nil || errors.Is(serr, syscall.EPERM)
}
