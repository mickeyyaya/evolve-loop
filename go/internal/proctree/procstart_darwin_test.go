//go:build darwin

package proctree

import (
	"encoding/binary"
	"errors"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

type fakeKinfo struct {
	err  error
	raw  []byte
	mibs [][]int32
}

func (f *fakeKinfo) call(mib []int32, out *byte, size *uintptr) error {
	f.mibs = append(f.mibs, slices.Clone(mib))
	if f.err != nil {
		return f.err
	}
	*size = uintptr(copy(unsafe.Slice(out, *size), f.raw))
	return nil
}

func kinfoWithStart(sec int64, usec int32) []byte {
	raw := make([]byte, kinfoProcBytes)
	binary.LittleEndian.PutUint64(raw[0:8], uint64(sec))
	binary.LittleEndian.PutUint32(raw[8:12], uint32(usec))
	return raw
}

func TestStartOfVia_ReadsPStarttimeOfTheAskedPid(t *testing.T) {
	t.Parallel()
	f := &fakeKinfo{raw: kinfoWithStart(1760011223, 4567)}

	got, err := startOfVia(f.call, 4242)

	if err != nil || got != "1760011223.004567" {
		t.Errorf("startOfVia = %q, %v, want %q", got, err, "1760011223.004567")
	}
	want := [][]int32{{ctlKern, kernProc, kernProcPID, 4242}}
	if len(f.mibs) != 1 || !slices.Equal(f.mibs[0], want[0]) {
		t.Errorf("sysctl mibs = %v, want %v: one KERN_PROC_PID read for pid 4242", f.mibs, want)
	}
}

func TestStartOfVia_AnEmptyAnswerIsESRCH(t *testing.T) {
	t.Parallel()
	f := &fakeKinfo{}

	got, err := startOfVia(f.call, 4242)

	if !errors.Is(err, syscall.ESRCH) || got != "" {
		t.Errorf("startOfVia = %q, %v, want ESRCH: the kernel answers a gone pid with no kinfo_proc", got, err)
	}
}

func TestStartOfVia_AShortAnswerIsESRCH(t *testing.T) {
	t.Parallel()
	f := &fakeKinfo{raw: kinfoWithStart(1760011223, 1)[:kinfoProcBytes-1]}

	got, err := startOfVia(f.call, 4242)

	if !errors.Is(err, syscall.ESRCH) || got != "" {
		t.Errorf("startOfVia = %q, %v, want ESRCH for an answer shorter than kinfo_proc", got, err)
	}
}

func TestStartOfVia_AKernelErrorIsReturned(t *testing.T) {
	t.Parallel()
	f := &fakeKinfo{err: syscall.EPERM, raw: kinfoWithStart(1, 1)}

	got, err := startOfVia(f.call, 4242)

	if !errors.Is(err, syscall.EPERM) || got != "" {
		t.Errorf("startOfVia = %q, %v, want EPERM and no start", got, err)
	}
}

func TestStartOf_ThisProcessReadsOneStableRecentValue(t *testing.T) {
	t.Parallel()
	first, err := StartOf(os.Getpid())
	if err != nil {
		t.Fatalf("StartOf(self) error = %v", err)
	}
	second, err := StartOf(os.Getpid())

	if err != nil || second != first {
		t.Errorf("StartOf(self) = %q then %q, %v, want one stable value", first, second, err)
	}
	sec, _, _ := strings.Cut(first, ".")
	n, perr := strconv.ParseInt(sec, 10, 64)
	started := time.Unix(n, 0)
	if perr != nil || started.After(time.Now()) || time.Since(started) > time.Hour {
		t.Errorf("StartOf(self) = %q, want the start second of this test process (within the last hour)", first)
	}
}

func TestStartOf_AnUnusedPidIsESRCH(t *testing.T) {
	t.Parallel()
	got, err := StartOf(math.MaxInt32)

	if !errors.Is(err, syscall.ESRCH) || got != "" {
		t.Errorf("StartOf(MaxInt32) = %q, %v, want ESRCH", got, err)
	}
}
