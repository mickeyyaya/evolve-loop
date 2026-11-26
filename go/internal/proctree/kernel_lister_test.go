package proctree

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

const linuxCommBytes = 15

func TestExecLister_RunsNoProgramAndListsThisProcess(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	table, err := ExecLister()(context.Background())

	if err != nil {
		t.Fatalf("ExecLister with an empty PATH = %v, want the table from the kernel: no program may run", err)
	}
	self, found := rowOf(table, os.Getpid())
	if !found {
		t.Fatalf("the table of %d rows lacks this process %d", len(table), os.Getpid())
	}
	if self.Ppid != os.Getppid() || self.Pgid != syscall.Getpgrp() || self.Comm == "" {
		t.Errorf("row = %+v, want ppid %d, pgid %d and a command name", self, os.Getppid(), syscall.Getpgrp())
	}
}

func rowOf(table []Process, pid int) (Process, bool) {
	for _, p := range table {
		if p.Pid == pid {
			return p, true
		}
	}
	return Process{}, false
}

func TestCommOf_ReadsOnePidWithoutExec(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	want := filepath.Base(os.Args[0])
	if len(want) > linuxCommBytes {
		want = want[:linuxCommBytes]
	}

	got, err := CommOf(os.Getpid())

	if err != nil || filepath.Base(got) != want {
		t.Errorf("CommOf(%d) = %q, %v, want the name %q of the test binary", os.Getpid(), got, err, want)
	}
}

func TestCommOf_AGonePidIsESRCH(t *testing.T) {
	t.Parallel()
	const beyondPidMax = 1 << 30

	got, err := CommOf(beyondPidMax)

	if err != syscall.ESRCH || got != "" {
		t.Errorf("CommOf(%d) = %q, %v, want ESRCH: a pid that cannot exist has no name", beyondPidMax, got, err)
	}
}

func TestExecLister_RowsHaveTheStartSecondOfStartTime(t *testing.T) {
	t.Parallel()
	table, err := ExecLister()(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	self, found := rowOf(table, os.Getpid())
	if !found {
		t.Fatalf("the table lacks this process")
	}

	want, err := StartTime(os.Getpid())

	if err != nil || !self.Started.Equal(want.Truncate(time.Second)) {
		t.Errorf("Started = %v, want %v (StartTime %v cut to the second): a persisted tree must match a new listing", self.Started, want.Truncate(time.Second), want)
	}
}
