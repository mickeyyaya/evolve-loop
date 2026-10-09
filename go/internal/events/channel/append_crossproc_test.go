//go:build integration

package channel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	crossProcRootEnv = "EVOLVE_CHANNEL_STRESS_ROOT"
	crossProcN       = 4
	crossProcBatches = 120
	crossProcSegment = 8 << 10
)

func TestHelperChannelAppender(t *testing.T) {
	root := os.Getenv(crossProcRootEnv)
	if root == "" {
		t.Skip("the body of a child process of TestAppend_ConcurrentProcessesWriteUniqueWholeLines")
	}
	l, err := New(root, testChannel, testConfig(crossProcSegment))
	if err != nil {
		t.Fatal(err)
	}
	for i := uint64(0); i < crossProcBatches; i++ {
		a, b := signalRecord(2*i, "first"), signalRecord(2*i+1, "second")
		a.Signal.PID, b.Signal.PID = os.Getpid(), os.Getpid()
		if _, err := l.Append([]Record{a, b}); err != nil {
			t.Fatalf("child append %d: %v", i, err)
		}
	}
}

func TestAppend_ConcurrentProcessesWriteUniqueWholeLines(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ch")
	var children []func() error
	for i := 0; i < crossProcN; i++ {
		cmd := sysexec.Command(context.Background(), os.Args[0], "-test.run=^TestHelperChannelAppender$", "-test.count=1")
		cmd.Env = append(os.Environ(), crossProcRootEnv+"="+root)
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			t.Fatalf("start child %d: %v", i, err)
		}
		children = append(children, cmd.Wait)
	}
	for i, wait := range children {
		if err := wait(); err != nil {
			t.Fatalf("child %d: %v", i, err)
		}
	}
	l, err := New(root, testChannel, testConfig(crossProcSegment))
	if err != nil {
		t.Fatal(err)
	}

	b := mustRead(t, l, 0)

	assertWholeUniqueBatches(t, b.Records)
	if len(segmentBases(t, root)) < 3 {
		t.Fatalf("segment bases = %v, want the race to cross rotations", segmentBases(t, root))
	}
}

func assertWholeUniqueBatches(t *testing.T, records []Record) {
	t.Helper()
	if len(records) != 2*crossProcN*crossProcBatches {
		t.Fatalf("records = %d, want %d", len(records), 2*crossProcN*crossProcBatches)
	}
	seen := map[string]bool{}
	pids := map[int]bool{}
	for i, r := range records {
		if r.Signal == nil {
			t.Fatalf("record %d = %q, want only whole signal lines", i, describe(records[i:i+1]))
		}
		key := fmt.Sprintf("%d.%d", r.Signal.PID, r.Signal.Seq)
		if seen[key] {
			t.Fatalf("event %s is in the channel twice", key)
		}
		seen[key] = true
		pids[r.Signal.PID] = true
		if i > 0 && r.Cursor <= records[i-1].Cursor {
			t.Fatalf("cursor %d after %d: cursors must increase", r.Cursor, records[i-1].Cursor)
		}
		if r.Signal.Seq%2 == 1 && (records[i-1].Signal.PID != r.Signal.PID || records[i-1].Signal.Seq != r.Signal.Seq-1) {
			t.Fatalf("event %s does not follow its batch mate: one batch is one write", key)
		}
	}
	if len(pids) != crossProcN {
		t.Fatalf("writers = %d, want %d", len(pids), crossProcN)
	}
}
