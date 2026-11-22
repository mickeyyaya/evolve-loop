package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/hostsentinel"
)

const midWaveErrorTail = 400

type midWaveOutcome struct {
	Skipped string
	Err     string
	Freed   int64
	After   hostsentinel.Sample
}

type midWaveRun struct {
	root     string
	budget   time.Duration
	lease    func(root string, wait time.Duration, stderr io.Writer) (func(), int)
	newRun   func(ctx context.Context, stdout, stderr io.Writer) gcRun
	free     func(path string) (uint64, error)
	cacheDir func(ctx context.Context) (string, error)
	measure  func(ctx context.Context, dir string) (int64, error)
}

func (m midWaveRun) run(ctx context.Context) midWaveOutcome {
	release, rc := m.lease(m.root, 0, io.Discard)
	if rc != cliExitOK {
		return midWaveOutcome{Skipped: "gc_lease_held"}
	}
	defer release()
	ctx, cancel := context.WithTimeout(ctx, m.budget)
	defer cancel()
	before, err := m.free(m.root)
	if err != nil {
		return midWaveOutcome{Err: fmt.Sprintf("free disk before the run: %v", err)}
	}
	var stdout, stderr bytes.Buffer
	rc = m.newRun(ctx, &stdout, &stderr).cacheScope(m.root)
	if rc != 0 || ctx.Err() != nil {
		return midWaveOutcome{Err: failureText(ctx, &stderr)}
	}
	return m.afterRun(ctx, before)
}

func failureText(ctx context.Context, stderr *bytes.Buffer) string {
	text := strings.TrimSpace(stderr.String())
	if len(text) > midWaveErrorTail {
		text = text[len(text)-midWaveErrorTail:]
	}
	if err := ctx.Err(); err != nil {
		text = fmt.Sprintf("the run stopped (%v): %s", err, text)
	}
	return "the cache scope failed: " + text
}

func (m midWaveRun) afterRun(ctx context.Context, before uint64) midWaveOutcome {
	after, err := m.free(m.root)
	if err != nil {
		return midWaveOutcome{Err: fmt.Sprintf("free disk after the run: %v", err)}
	}
	dir, err := m.cacheDir(ctx)
	if err != nil {
		return midWaveOutcome{Err: fmt.Sprintf("go cache dir after the run: %v", err)}
	}
	size, err := m.measure(ctx, dir)
	if err != nil {
		return midWaveOutcome{Err: fmt.Sprintf("go cache size after the run: %v", err)}
	}
	return midWaveOutcome{
		Freed: int64(after) - int64(before),
		After: hostsentinel.Sample{FreeBytes: after, Path: m.root, CacheBytes: size, CacheRead: true},
	}
}
