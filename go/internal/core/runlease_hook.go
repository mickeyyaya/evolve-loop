package core

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func leaseRefreshInterval() time.Duration { return runlease.DefaultTTL / 2 }

func startRunLease(workspace, runID string, nowFn func() time.Time, interval time.Duration) (stop func()) {
	if workspace == "" {
		return func() {}
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN runlease mkdir %s: %v\n", workspace, err)
	}
	lease := runlease.Lease{RunID: runID, OwnerPID: os.Getpid()}
	if err := runlease.Write(workspace, lease, nowFn()); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN runlease initial write %s: %v\n", workspace, err)
	}
	ticker := time.NewTicker(interval)
	done := make(chan struct{})
	go runLeaseHeartbeat(workspace, lease, ticker.C, nowFn, done)
	var once sync.Once
	return func() {
		once.Do(func() {
			close(done)
			ticker.Stop()
		})
	}
}

func runLeaseHeartbeat(workspace string, lease runlease.Lease, tick <-chan time.Time, nowFn func() time.Time, done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case <-tick:
			if err := runlease.Write(workspace, lease, nowFn()); err != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN runlease refresh %s: %v\n", workspace, err)
			}
		}
	}
}
