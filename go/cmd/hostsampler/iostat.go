package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	closeGrace      = 200 * time.Millisecond
	cpuLineMaxAge   = 2 * time.Second
	iostatMinFields = 6
	iostatIdleFrom  = 4
)

type cpuFeed struct {
	mu  sync.Mutex
	pct float64
	at  time.Time
	has bool
	now func() time.Time
}

func parseIostatCPU(line string) (float64, bool) {
	fields := strings.Fields(line)
	if len(fields) < iostatMinFields {
		return 0, false
	}
	idle, err := strconv.ParseFloat(fields[len(fields)-iostatIdleFrom], 64)
	if err != nil {
		return 0, false
	}
	return min(max(percent-idle, 0), percent), true
}

func startCPUFeed(open func(context.Context, string, ...string) (io.ReadCloser, error), now func() time.Time, diag io.Writer) *cpuFeed {
	feed := &cpuFeed{now: now}
	stream, err := open(context.Background(), "iostat", "-w", "1")
	if err != nil {
		fmt.Fprintf(diag, "hostsampler: iostat did not start: %v\n", err)
		return feed
	}
	go feed.consume(stream, diag)
	return feed
}

func (f *cpuFeed) consume(stream io.ReadCloser, diag io.Writer) {
	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		if pct, ok := parseIostatCPU(scanner.Text()); ok {
			f.mu.Lock()
			f.pct, f.at, f.has = pct, f.now(), true
			f.mu.Unlock()
		}
	}
	fmt.Fprintf(diag, "hostsampler: iostat stream ended: scan=%v close=%v\n", scanner.Err(), stream.Close())
}

func (f *cpuFeed) latest(now time.Time) (float64, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.pct, f.has && now.Sub(f.at) <= cpuLineMaxAge
}

type childStdout struct {
	io.ReadCloser
	wait   func() error
	cancel context.CancelFunc
}

func (s childStdout) Close() error {
	closeErr := s.ReadCloser.Close()
	exited := make(chan error, 1)
	go func() { exited <- s.wait() }()
	select {
	case waitErr := <-exited:
		s.cancel()
		return errors.Join(closeErr, waitErr)
	case <-time.After(closeGrace):
		s.cancel()
		return errors.Join(closeErr, <-exited)
	}
}

func execStream(ctx context.Context, name string, args ...string) (io.ReadCloser, error) {
	ctx, cancel := context.WithCancel(ctx)
	cmd := sysexec.Command(ctx, name, args...)
	out, err := cmd.StdoutPipe()
	if err == nil {
		err = cmd.Start()
	}
	if err != nil {
		cancel()
		return nil, err
	}
	return childStdout{ReadCloser: out, wait: cmd.Wait, cancel: cancel}, nil
}
