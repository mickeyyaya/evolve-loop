//go:build darwin

package wake

import (
	"errors"
	"io/fs"
	"os"
	"reflect"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

const fakeKq = 3

type fakeKqueue struct {
	kqErr     error
	regErr    map[int16]error
	procErr   map[uint64]error
	deleteErr map[uint64]error
	openErr   map[string]error
	statfsErr error
	fstype    map[string]string
	terminal  bool
	nextFD    int
	opened    map[string]int
	openModes []int
	closed    []int
	closeErr  error
	changes   []syscall.Kevent_t
	events    []syscall.Kevent_t
	waitErr   error
	timeouts  []*syscall.Timespec
}

func newFakeKqueue() *fakeKqueue {
	return &fakeKqueue{regErr: map[int16]error{}, procErr: map[uint64]error{}, deleteErr: map[uint64]error{}, openErr: map[string]error{}, fstype: map[string]string{}, nextFD: 1000, opened: map[string]int{}}
}

func (f *fakeKqueue) port() kqueuePort {
	return kqueuePort{f.kqueue, f.kevent, f.open, f.statfs, f.close, func(int) bool { return f.terminal }}
}

func (f *fakeKqueue) kqueue() (int, error) {
	if f.kqErr != nil {
		return -1, f.kqErr
	}
	return fakeKq, nil
}

func (f *fakeKqueue) kevent(kq int, changes, events []syscall.Kevent_t, timeout *syscall.Timespec) (int, error) {
	if len(changes) > 0 {
		f.changes = append(f.changes, changes...)
		return 0, f.registerError(changes[0])
	}
	f.timeouts = append(f.timeouts, timeout)
	if f.waitErr != nil {
		return -1, f.waitErr
	}
	return copy(events, f.events), nil
}

func (f *fakeKqueue) registerError(c syscall.Kevent_t) error {
	if c.Filter == syscall.EVFILT_PROC && c.Flags&syscall.EV_DELETE != 0 {
		return f.deleteErr[c.Ident]
	}
	if err := f.procErr[c.Ident]; err != nil && c.Filter == syscall.EVFILT_PROC {
		return err
	}
	return f.regErr[c.Filter]
}

func (f *fakeKqueue) open(path string, mode int, _ uint32) (int, error) {
	f.openModes = append(f.openModes, mode)
	if err := f.openErr[path]; err != nil {
		return -1, err
	}
	f.nextFD++
	f.opened[path] = f.nextFD
	return f.nextFD, nil
}

func (f *fakeKqueue) statfs(path string, buf *syscall.Statfs_t) error {
	name := "apfs"
	if n, ok := f.fstype[path]; ok {
		name = n
	}
	for i := range buf.Fstypename {
		buf.Fstypename[i] = 0
	}
	for i := 0; i < len(name); i++ {
		buf.Fstypename[i] = int8(name[i])
	}
	return f.statfsErr
}

func (f *fakeKqueue) close(fd int) error {
	f.closed = append(f.closed, fd)
	return f.closeErr
}

func (f *fakeKqueue) filters() []int16 {
	var out []int16
	for _, c := range f.changes {
		out = append(out, c.Filter)
	}
	return out
}

func newFakeKernelKqueue(t *testing.T, f *fakeKqueue, output *os.File) *kqueueKernel {
	t.Helper()
	k, err := newKqueue(f.port(), output)
	if err != nil {
		t.Fatalf("newKqueue = %v, want a kernel", err)
	}
	return k
}

func TestWaiter_RefusesANetworkFilesystem(t *testing.T) {
	t.Parallel()
	f := newFakeKqueue()
	f.fstype["/mnt/share/ch/loop"] = "smbfs"
	k := newFakeKernelKqueue(t, f, nil)

	_, err := k.arm(Targets{Dirs: []string{"/mnt/share/ch/loop"}})

	if !errors.Is(err, ErrRefused) || !strings.Contains(err.Error(), "smbfs") || !strings.Contains(err.Error(), "/mnt/share/ch/loop") {
		t.Errorf("arm on smbfs = %v, want %v naming the path and smbfs", err, ErrRefused)
	}
	if len(f.opened) != 0 {
		t.Errorf("opened %v, want nothing opened on a refused filesystem", f.opened)
	}
}

func TestKqueueArm_AcceptsAPFSAndHFS(t *testing.T) {
	t.Parallel()
	f := newFakeKqueue()
	f.fstype["/hfs"] = "hfs"
	k := newFakeKernelKqueue(t, f, nil)

	_, err := k.arm(Targets{Dirs: []string{"/apfs", "/hfs"}})

	if err != nil || len(f.opened) != 2 {
		t.Errorf("arm on apfs and hfs = %v with %v opened, want both armed", err, f.opened)
	}
}

func TestWaiter_AKernelRefusalFailsLoudly(t *testing.T) {
	t.Parallel()
	f := newFakeKqueue()
	k := newFakeKernelKqueue(t, f, nil)
	f.regErr[syscall.EVFILT_VNODE] = syscall.EMFILE

	_, err := k.arm(Targets{Dirs: []string{"/ch/loop"}})

	if !errors.Is(err, ErrRefused) || !errors.Is(err, syscall.EMFILE) {
		t.Errorf("arm = %v, want %v wrapping EMFILE", err, ErrRefused)
	}
	if !slices.Contains(f.closed, f.opened["/ch/loop"]) {
		t.Errorf("closed %v, want the refused descriptor %d closed", f.closed, f.opened["/ch/loop"])
	}
}

func TestNewKqueue_RegistersTheCancelEventAndTheHangup(t *testing.T) {
	t.Parallel()
	r, out, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer out.Close()
	f := newFakeKqueue()

	newFakeKernelKqueue(t, f, out)

	want := []syscall.Kevent_t{
		{Ident: userIdent, Filter: syscall.EVFILT_USER, Flags: syscall.EV_ADD | syscall.EV_CLEAR},
		{Ident: uint64(out.Fd()), Filter: syscall.EVFILT_WRITE, Flags: syscall.EV_ADD | syscall.EV_CLEAR},
	}
	if !reflect.DeepEqual(f.changes, want) {
		t.Errorf("changes = %+v, want %+v", f.changes, want)
	}
}

func TestNewKqueue_EachFailureIsRefusedAndClosesTheQueue(t *testing.T) {
	t.Parallel()
	r, out, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer out.Close()
	cases := []struct {
		name   string
		filter int16
	}{
		{"cancel event", syscall.EVFILT_USER},
		{"hangup", syscall.EVFILT_WRITE},
	}
	for _, c := range cases {
		f := newFakeKqueue()
		f.regErr[c.filter] = syscall.EINVAL

		k, err := newKqueue(f.port(), out)

		if k != nil || !errors.Is(err, ErrRefused) || !errors.Is(err, syscall.EINVAL) || !slices.Equal(f.closed, []int{fakeKq}) {
			t.Errorf("%s: newKqueue = %v, %v, closed %v, want a refusal and the queue closed", c.name, k, err, f.closed)
		}
	}
}

func TestNewKqueue_AQueueFailureIsRefused(t *testing.T) {
	t.Parallel()
	f := newFakeKqueue()
	f.kqErr = syscall.EMFILE

	k, err := newKqueue(f.port(), nil)

	if k != nil || !errors.Is(err, ErrRefused) || !errors.Is(err, syscall.EMFILE) {
		t.Errorf("newKqueue = %v, %v, want a refusal wrapping EMFILE", k, err)
	}
}

func TestNewKqueue_AnUnreadableOutputClosesTheQueue(t *testing.T) {
	t.Parallel()
	out, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	f := newFakeKqueue()

	k, err := newKqueue(f.port(), out)

	if k != nil || err == nil || errors.Is(err, ErrRefused) || !slices.Equal(f.closed, []int{fakeKq}) {
		t.Errorf("newKqueue = %v, %v, closed %v, want the stat error and the queue closed", k, err, f.closed)
	}
}

func TestKqueueArm_RegistersDirectoriesAndFilesWithTheirNotesAndClear(t *testing.T) {
	t.Parallel()
	f := newFakeKqueue()
	k := newFakeKernelKqueue(t, f, nil)
	f.changes = nil

	_, err := k.arm(Targets{Dirs: []string{"/ch/loop"}, Files: []string{"/ch/loop/seg"}})

	want := []syscall.Kevent_t{
		{Ident: uint64(f.opened["/ch/loop"]), Filter: syscall.EVFILT_VNODE, Flags: syscall.EV_ADD | syscall.EV_CLEAR, Fflags: syscall.NOTE_WRITE},
		{Ident: uint64(f.opened["/ch/loop/seg"]), Filter: syscall.EVFILT_VNODE, Flags: syscall.EV_ADD | syscall.EV_CLEAR, Fflags: syscall.NOTE_WRITE | syscall.NOTE_EXTEND | syscall.NOTE_DELETE | syscall.NOTE_RENAME | syscall.NOTE_ATTRIB},
	}
	if err != nil || !reflect.DeepEqual(f.changes, want) {
		t.Errorf("arm = %v with changes %+v, want %+v", err, f.changes, want)
	}
	if mode := syscall.O_EVTONLY | syscall.O_CLOEXEC; !slices.Equal(f.openModes, []int{mode, mode}) {
		t.Errorf("open modes = %v, want O_EVTONLY|O_CLOEXEC for each path", f.openModes)
	}
}

func TestKqueueArm_ARearmClosesTheOldDescriptors(t *testing.T) {
	t.Parallel()
	f := newFakeKqueue()
	k := newFakeKernelKqueue(t, f, nil)
	if _, err := k.arm(Targets{Dirs: []string{"/ch/loop"}, Files: []string{"/ch/loop/seg-0"}}); err != nil {
		t.Fatal(err)
	}
	old := []int{f.opened["/ch/loop"], f.opened["/ch/loop/seg-0"]}

	_, err := k.arm(Targets{Dirs: []string{"/ch/loop"}, Files: []string{"/ch/loop/seg-1"}})

	if err != nil || !slices.Equal(f.closed, old) || !slices.Equal(k.watched, []int{f.opened["/ch/loop"], f.opened["/ch/loop/seg-1"]}) {
		t.Errorf("rearm = %v, closed %v, watched %v, want the old %v closed and the new tail watched", err, f.closed, k.watched, old)
	}
}

func TestKqueueArm_ACloseFailureOfTheOldDescriptorsIsReturned(t *testing.T) {
	t.Parallel()
	f := newFakeKqueue()
	k := newFakeKernelKqueue(t, f, nil)
	if _, err := k.arm(Targets{Dirs: []string{"/ch/loop"}}); err != nil {
		t.Fatal(err)
	}
	f.closeErr = syscall.EIO

	_, err := k.arm(Targets{Dirs: []string{"/ch/loop"}})

	if !errors.Is(err, syscall.EIO) {
		t.Errorf("rearm = %v, want the EIO of the old close", err)
	}
}

func TestKqueueArm_AFailedPathClosesTheNewDescriptorsAndKeepsTheOld(t *testing.T) {
	t.Parallel()
	f := newFakeKqueue()
	k := newFakeKernelKqueue(t, f, nil)
	if _, err := k.arm(Targets{Dirs: []string{"/ch/loop"}}); err != nil {
		t.Fatal(err)
	}
	old := slices.Clone(k.watched)
	f.openErr["/ch/cycle"] = syscall.ENFILE

	_, err := k.arm(Targets{Dirs: []string{"/ch/errors", "/ch/cycle"}})

	if !errors.Is(err, ErrRefused) || !errors.Is(err, syscall.ENFILE) {
		t.Errorf("arm = %v, want a refusal wrapping ENFILE", err)
	}
	if !slices.Equal(f.closed, []int{f.opened["/ch/errors"]}) || !slices.Equal(k.watched, old) {
		t.Errorf("closed %v, watched %v, want only the new descriptor closed and the old watch %v kept", f.closed, k.watched, old)
	}
}

func TestKqueueArm_AStatfsFailureIsAPathError(t *testing.T) {
	t.Parallel()
	f := newFakeKqueue()
	f.statfsErr = syscall.ENOENT
	k := newFakeKernelKqueue(t, f, nil)

	_, err := k.arm(Targets{Files: []string{"/ch/loop/seg"}})

	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) || pathErr.Path != "/ch/loop/seg" || !errors.Is(err, fs.ErrNotExist) || errors.Is(err, ErrRefused) {
		t.Errorf("arm = %v, want a statfs path error for /ch/loop/seg that is not a refusal", err)
	}
}

func TestKqueueArm_ArmsEachPidOnceAndForgetsTheOnesLeftOut(t *testing.T) {
	t.Parallel()
	f := newFakeKqueue()
	k := newFakeKernelKqueue(t, f, nil)
	f.changes = nil

	if _, err := k.arm(Targets{Pids: []int{41, 42}}); err != nil {
		t.Fatal(err)
	}
	gone, err := k.arm(Targets{Pids: []int{42, 43}})

	want := []syscall.Kevent_t{procChange(41), procChange(42), procChange(43), procDelete(41)}
	if err != nil || gone != nil || !reflect.DeepEqual(f.changes, want) {
		t.Errorf("arm = %v, %v with changes %+v, want one exit watch for each new pid", gone, err, f.changes)
	}
	if !reflect.DeepEqual(k.pids, map[int]bool{42: true, 43: true}) {
		t.Errorf("pids = %v, want 42 and 43", k.pids)
	}
}

func procDelete(pid int) syscall.Kevent_t {
	return syscall.Kevent_t{Ident: uint64(pid), Filter: syscall.EVFILT_PROC, Flags: syscall.EV_DELETE}
}

func procChange(pid int) syscall.Kevent_t {
	return syscall.Kevent_t{Ident: uint64(pid), Filter: syscall.EVFILT_PROC, Flags: syscall.EV_ADD | syscall.EV_CLEAR, Fflags: syscall.NOTE_EXIT}
}

func TestKqueueArm_AGonePidIsReportedAndAnotherFailureIsRefused(t *testing.T) {
	t.Parallel()
	f := newFakeKqueue()
	k := newFakeKernelKqueue(t, f, nil)
	f.regErr[syscall.EVFILT_PROC] = syscall.ESRCH

	gone, err := k.arm(Targets{Pids: []int{41}})

	if err != nil || !slices.Equal(gone, []int{41}) || k.pids[41] {
		t.Errorf("arm = %v, %v, pids %v, want 41 gone and not watched", gone, err, k.pids)
	}
	f.regErr[syscall.EVFILT_PROC] = syscall.EPERM
	if _, err := k.arm(Targets{Pids: []int{42}}); !errors.Is(err, ErrRefused) || !errors.Is(err, syscall.EPERM) {
		t.Errorf("arm = %v, want a refusal wrapping EPERM", err)
	}
}

func TestKqueueWait_DecodesEachFilter(t *testing.T) {
	t.Parallel()
	f := newFakeKqueue()
	k := newFakeKernelKqueue(t, f, nil)
	if _, err := k.arm(Targets{Pids: []int{42}}); err != nil {
		t.Fatal(err)
	}
	f.events = []syscall.Kevent_t{
		{Filter: syscall.EVFILT_USER},
		{Filter: syscall.EVFILT_WRITE},
		{Ident: 7, Filter: syscall.EVFILT_PROC},
		{Ident: 42, Filter: syscall.EVFILT_PROC},
		{Filter: syscall.EVFILT_VNODE},
		{Filter: syscall.EVFILT_WRITE, Flags: syscall.EV_EOF},
	}

	got, err := k.wait(1500 * time.Millisecond)

	want := Wake{Changed: true, Exited: []int{42}, Hangup: true}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("wait = %+v, %v, want %+v: a write without EOF, a user event and an unwatched pid are no wake", got, err, want)
	}
	if ts := f.firstTimeout(); ts == nil || ts.Sec != 1 || ts.Nsec != 500_000_000 {
		t.Errorf("timeout = %+v, want 1.5 s", ts)
	}
	if k.pids[42] {
		t.Errorf("pids = %v, want 42 forgotten after its exit", k.pids)
	}
}

func TestKqueueWait_NoTimeoutAndTheKernelError(t *testing.T) {
	t.Parallel()
	f := newFakeKqueue()
	k := newFakeKernelKqueue(t, f, nil)
	f.waitErr = syscall.EINTR

	got, err := k.wait(noTimeout)

	if !errors.Is(err, syscall.EINTR) || !reflect.DeepEqual(got, Wake{}) || len(f.timeouts) != 1 || f.firstTimeout() != nil {
		t.Errorf("wait = %+v, %v with timeouts %v, want EINTR after one call with a nil timeout", got, err, f.timeouts)
	}
}

func TestKqueueCancel_TriggersTheUserEvent(t *testing.T) {
	t.Parallel()
	f := newFakeKqueue()
	k := newFakeKernelKqueue(t, f, nil)
	f.changes = nil
	f.regErr[syscall.EVFILT_USER] = syscall.EBADF

	err := k.cancel()

	want := []syscall.Kevent_t{{Ident: userIdent, Filter: syscall.EVFILT_USER, Fflags: syscall.NOTE_TRIGGER}}
	if !errors.Is(err, syscall.EBADF) || !reflect.DeepEqual(f.changes, want) {
		t.Errorf("cancel = %v with changes %+v, want the trigger %+v and its error", err, f.changes, want)
	}
}

func TestKqueueClose_ClosesEveryDescriptorAndJoinsTheErrors(t *testing.T) {
	t.Parallel()
	f := newFakeKqueue()
	k := newFakeKernelKqueue(t, f, nil)
	if _, err := k.arm(Targets{Dirs: []string{"/ch/loop"}}); err != nil {
		t.Fatal(err)
	}
	f.closeErr = syscall.EIO

	err := k.close()

	if !errors.Is(err, syscall.EIO) || !slices.Equal(f.closed, []int{f.opened["/ch/loop"], fakeKq}) {
		t.Errorf("close = %v, closed %v, want EIO and the watch and the queue closed", err, f.closed)
	}
}

func (f *fakeKqueue) firstTimeout() *syscall.Timespec {
	if len(f.timeouts) == 0 {
		return &syscall.Timespec{Sec: -1}
	}
	return f.timeouts[0]
}

func TestKqueueWait_AWritableOutputWithoutEOFIsNoHangup(t *testing.T) {
	t.Parallel()
	f := newFakeKqueue()
	k := newFakeKernelKqueue(t, f, nil)
	f.events = []syscall.Kevent_t{{Filter: syscall.EVFILT_WRITE, Data: 512}}

	got, err := k.wait(noTimeout)

	if err != nil || !reflect.DeepEqual(got, Wake{}) {
		t.Errorf("wait = %+v, %v, want no wake: a reader that drains the pipe is still there", got, err)
	}
}

func TestKqueueArm_APidArmedBeforeAFailureStaysWatched(t *testing.T) {
	t.Parallel()
	f := newFakeKqueue()
	k := newFakeKernelKqueue(t, f, nil)
	f.procErr[42] = syscall.EPERM

	_, err := k.arm(Targets{Pids: []int{41, 42}})
	f.events = []syscall.Kevent_t{{Ident: 41, Filter: syscall.EVFILT_PROC}}
	got, waitErr := k.wait(noTimeout)

	if !errors.Is(err, ErrRefused) || !errors.Is(err, syscall.EPERM) {
		t.Errorf("arm = %v, want a refusal wrapping EPERM", err)
	}
	if waitErr != nil || !slices.Equal(got.Exited, []int{41}) {
		t.Errorf("wait = %+v, %v, want the exit of 41: its watch was armed before the failure", got, waitErr)
	}
}

func TestKqueueArm_APidLeftOutIsDeletedAndAGoneKnoteIsNoError(t *testing.T) {
	t.Parallel()
	for _, gone := range []error{syscall.ENOENT, syscall.ESRCH} {
		f := newFakeKqueue()
		k := newFakeKernelKqueue(t, f, nil)
		if _, err := k.arm(Targets{Pids: []int{41, 42, 43}}); err != nil {
			t.Fatal(err)
		}
		f.deleteErr[43] = gone
		f.changes = nil

		_, err := k.arm(Targets{Pids: []int{42}})

		deleted := []uint64{}
		for _, c := range f.changes {
			deleted = append(deleted, c.Ident)
		}
		slices.Sort(deleted)
		if err != nil || !slices.Equal(deleted, []uint64{41, 43}) || !reflect.DeepEqual(k.pids, map[int]bool{42: true}) {
			t.Errorf("arm with %v = %v, deleted %v, pids %v, want 41 and 43 deleted, 42 kept and no error", gone, err, deleted, k.pids)
		}
	}
}

func TestKqueueArm_AFailedDeleteIsReturned(t *testing.T) {
	t.Parallel()
	f := newFakeKqueue()
	k := newFakeKernelKqueue(t, f, nil)
	if _, err := k.arm(Targets{Pids: []int{41}}); err != nil {
		t.Fatal(err)
	}
	f.deleteErr[41] = syscall.EBADF

	_, err := k.arm(Targets{})

	if !errors.Is(err, syscall.EBADF) || len(k.pids) != 0 {
		t.Errorf("arm = %v, pids %v, want the EBADF of the delete and 41 forgotten", err, k.pids)
	}
}
