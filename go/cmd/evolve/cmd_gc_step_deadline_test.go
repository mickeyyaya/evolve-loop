package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func spentGCReaperContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	t.Cleanup(cancel)
	<-ctx.Done()
	return ctx
}

func TestGCProject_AStepAfterASpentReaperDeadlineStillRuns(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	gcFakeGoCache(t)
	gcWritePolicy(t, projectRoot, `{"go_cache_ttl_hours":24}`)
	withGCProcessHost(t, &fakeGCProcessHost{})
	var stdout, stderr bytes.Buffer

	newGCRun(spentGCReaperContext(t), true, &stdout, &stderr).project(projectRoot)

	if strings.Contains(stderr.String(), "go build cache trim skipped") {
		t.Fatalf("the go cache step inherited the spent reaper deadline:\n%s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "1 go build cache file(s) would be trimmed") {
		t.Errorf("the go cache step did not run:\nstdout=%s\nstderr=%s", stdout.String(), stderr.String())
	}
}

func TestGCStep_GivesTheStepItsOwnLiveDeadline(t *testing.T) {
	t.Parallel()
	r := gcRun{ctx: spentGCReaperContext(t)}
	var live bool
	var left time.Duration

	r.step(func(s gcRun) bool {
		deadline, ok := s.ctx.Deadline()
		live, left = s.ctx.Err() == nil && ok, time.Until(deadline)
		return false
	})

	if !live || left <= 0 || left > orphanGCTimeout {
		t.Errorf("step context live=%v, time left=%v; want a live context bounded by %v", live, left, orphanGCTimeout)
	}
}

func TestGCStep_ReturnsTheStepResultAndEndsItsContext(t *testing.T) {
	t.Parallel()
	r := gcRun{ctx: context.Background()}
	var seen context.Context

	failed := r.step(func(s gcRun) bool { seen = s.ctx; return true })

	if !failed {
		t.Error("step dropped the failure of the step")
	}
	if seen.Err() == nil {
		t.Error("the step context is still live after the step returned")
	}
}
