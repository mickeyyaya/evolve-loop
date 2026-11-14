package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
)

func TestDiskSignals_NameTheBreachedLevelAndStateTheFloor(t *testing.T) {
	cases := []struct {
		name       string
		free       uint64
		err        error
		wantStderr string
		notStderr  string
	}{
		{"under the floor", 1e9, nil, "gc.disk_floor_gb", "gc.disk_warn_gb"},
		{"under the warn level", 15e9, nil, "gc.disk_warn_gb", "gc.disk_floor_gb"},
		{"healthy", 50e9, nil, "", "WARN"},
		{"unreadable", 0, errors.New("statfs failed"), "unreadable: statfs failed", "gc.disk_floor_gb"},
	}
	for _, c := range cases {
		prev := gcDiskFreeBytes
		gcDiskFreeBytes = func(string) (uint64, error) { return c.free, c.err }
		var stdout, stderr bytes.Buffer
		newGCRun(context.Background(), true, &stdout, &stderr).diskSignals(t.TempDir(), gc.Policy{})
		gcDiskFreeBytes = prev

		if !strings.Contains(stdout.String(), "gc.disk_floor_gb is 10.00 GB") {
			t.Errorf("%s: stdout does not state the default floor: %q", c.name, stdout.String())
		}
		if c.wantStderr != "" && !strings.Contains(stderr.String(), c.wantStderr) {
			t.Errorf("%s: stderr %q lacks %q", c.name, stderr.String(), c.wantStderr)
		}
		if strings.Contains(stderr.String(), c.notStderr) {
			t.Errorf("%s: stderr %q holds %q", c.name, stderr.String(), c.notStderr)
		}
	}
}
