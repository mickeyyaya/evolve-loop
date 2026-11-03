//go:build integration

package main

import (
	"os"
	"testing"
	"time"
)

func TestProcessStartTime_ReadsTheRunningProcess(t *testing.T) {
	t.Parallel()
	started, ok := processStartTime(os.Getpid())

	now := time.Now()
	if !ok || started.After(now) || now.Sub(started) > time.Hour {
		t.Errorf("processStartTime(self) = %v, %t; want a start in the last hour, not after %v", started, ok, now)
	}
	if _, ok := processStartTime(-1); ok {
		t.Error("processStartTime(-1) reported a start time for no process")
	}
}

func TestProcessStartTime_NeedsNoPsOnThePath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	started, ok := processStartTime(os.Getpid())

	now := time.Now()
	if !ok || started.After(now) || now.Sub(started) > time.Hour {
		t.Errorf("processStartTime(self) with an empty PATH = %v, %t; want a start in the last hour: the kernel read needs no exec", started, ok)
	}
}
