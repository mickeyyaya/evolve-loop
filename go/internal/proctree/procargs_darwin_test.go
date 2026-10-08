//go:build darwin

package proctree

import (
	"errors"
	"os"
	"reflect"
	"slices"
	"syscall"
	"testing"
	"unsafe"
)

type fakeSysctl struct {
	sizeErr, readErr error
	raw              []byte
	pids             []int32
}

func (f *fakeSysctl) call(mib *[procArgsMibLen]int32, out *byte, size *uintptr) error {
	f.pids = append(f.pids, mib[2])
	if out == nil {
		*size = uintptr(len(f.raw))
		return f.sizeErr
	}
	if f.readErr != nil {
		return f.readErr
	}
	*size = uintptr(copy(unsafe.Slice(out, *size), f.raw))
	return nil
}

func TestReadProcArgsVia_ParsesTheBufferOfTheAskedPid(t *testing.T) {
	t.Parallel()
	f := &fakeSysctl{raw: []byte("\x02\x00\x00\x00/bin/x\x00\x00/bin/x\x00-v\x00EVOLVE_DISPATCH_ID=r/1/a/p9n1\x00")}

	args, env, err := readProcArgsVia(f.call, 4242)

	if err != nil || !reflect.DeepEqual(args, []string{"/bin/x", "-v"}) || env["EVOLVE_DISPATCH_ID"] != "r/1/a/p9n1" {
		t.Errorf("readProcArgsVia = %q, %v, %v, want the two arguments and the tag", args, env, err)
	}
	if !reflect.DeepEqual(f.pids, []int32{4242, 4242}) {
		t.Errorf("sysctl pids = %v, want the size query and the read both for pid 4242", f.pids)
	}
}

func TestReadProcArgsVia_ASizeQueryFailureIsReturned(t *testing.T) {
	t.Parallel()
	f := &fakeSysctl{sizeErr: syscall.ESRCH, raw: []byte("x")}

	args, _, err := readProcArgsVia(f.call, 4242)

	if !errors.Is(err, syscall.ESRCH) || args != nil || len(f.pids) != 1 {
		t.Errorf("readProcArgsVia = %q, %v after %d calls, want ESRCH after the size query alone", args, err, len(f.pids))
	}
}

func TestReadProcArgsVia_AnEmptyBufferIsTruncated(t *testing.T) {
	t.Parallel()
	f := &fakeSysctl{}

	args, _, err := readProcArgsVia(f.call, 4242)

	if !errors.Is(err, errProcArgsTruncated) || args != nil || len(f.pids) != 1 {
		t.Errorf("readProcArgsVia = %q, %v after %d calls, want %v and no read", args, err, len(f.pids), errProcArgsTruncated)
	}
}

func TestReadProcArgsVia_AReadFailureIsReturned(t *testing.T) {
	t.Parallel()
	f := &fakeSysctl{readErr: syscall.EPERM, raw: []byte("\x01\x00\x00\x00/bin/x\x00\x00/bin/x\x00")}

	args, _, err := readProcArgsVia(f.call, 4242)

	if !errors.Is(err, syscall.EPERM) || args != nil {
		t.Errorf("readProcArgsVia = %q, %v, want EPERM and no arguments", args, err)
	}
}

func TestReadProcArgs_ReadsTheArgumentsOfThisTestProcess(t *testing.T) {
	t.Parallel()
	args, _, err := readProcArgs(os.Getpid())

	if err != nil || !slices.Equal(args, os.Args) {
		t.Errorf("readProcArgs(self) = %q, %v, want %q", args, err, os.Args)
	}
}

func TestReadProcArgs_AnInvalidPidIsAKernelError(t *testing.T) {
	t.Parallel()
	args, env, err := readProcArgs(-1)

	var errno syscall.Errno
	if !errors.As(err, &errno) || args != nil || env != nil {
		t.Errorf("readProcArgs(-1) = %q, %v, %v, want a kernel errno and nothing read", args, env, err)
	}
}
