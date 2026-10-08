//go:build integration

package main

import (
	"os"
	"testing"
	"time"
)

func TestProcessStartTime_ReadsTheRunningProcess(t *testing.T) {
	started, ok := processStartTime(os.Getpid())

	now := time.Now()
	if !ok || started.After(now) || now.Sub(started) > time.Hour {
		t.Errorf("processStartTime(self) = %v, %t; want a start in the last hour, not after %v", started, ok, now)
	}
	if _, ok := processStartTime(-1); ok {
		t.Error("processStartTime(-1) reported a start time for no process")
	}
}
