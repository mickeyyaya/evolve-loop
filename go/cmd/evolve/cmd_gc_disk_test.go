package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestDiskSignals_NameTheBreachedLevelAndStateTheFloor(t *testing.T) {
	cases := []struct {
		name       string
		free       uint64
		err        error
		wantStderr string
		notStderr  string
	}{
		{"under the floor", 1e9, nil, "disk.floor_gb", "disk.warn_gb"},
		{"under the warn level", 15e9, nil, "disk.warn_gb", "disk.floor_gb"},
		{"healthy", 50e9, nil, "", "WARN"},
		{"unreadable", 0, errors.New("statfs failed"), "unreadable: statfs failed", "disk.floor_gb"},
	}
	for _, c := range cases {
		prev := gcDiskFreeBytes
		gcDiskFreeBytes = func(string) (uint64, error) { return c.free, c.err }
		var stdout, stderr bytes.Buffer
		newGCRun(context.Background(), true, &stdout, &stderr).diskSignals(t.TempDir(), policy.Policy{})
		gcDiskFreeBytes = prev

		if !strings.Contains(stdout.String(), "disk.floor_gb is 10.00 GB") {
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
