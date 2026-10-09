//go:build linux

package wake

import (
	"errors"
	"io/fs"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	fakeEpoll   = 3
	fakeCancelR = 4
	fakeCancelW = 5
	ext4Magic   = 0xef53
	nfsMagic    = 0x6969
)

type epollAdd struct {
	fd     int
	events uint32
}

type fakeInotify struct {
	failAt    map[string]error
	fstype    map[string]int64
	statfsErr error
	terminal  bool
	nextFD    int
	instances []int
	watches   map[int][]string
	masks     []uint32
	added     []epollAdd
	pidfds    map[int]int
	closed    []int
	closeErr  error
	events    []syscall.EpollEvent
	waitErr   error
	msecs     []int
	reads     map[int][]error
	written   []int
	writeErr  error
}

func newFakeInotify() *fakeInotify {
	return &fakeInotify{failAt: map[string]error{}, fstype: map[string]int64{}, nextFD: 1000, watches: map[int][]string{}, pidfds: map[int]int{}, reads: map[int][]error{}}
}

func (f *fakeInotify) port() inotifyPort {
	return inotifyPort{
		f.inotifyInit, f.addWatch, f.epollCreate, f.epollCtl, f.epollWait, f.pipe, f.pidfdOpen,
		f.read, f.write, f.statfs, f.close, func(int) bool { return f.terminal },
	}
}

func (f *fakeInotify) newFD() int {
	f.nextFD++
	return f.nextFD
}

func (f *fakeInotify) inotifyInit(flags int) (int, error) {
	if err := f.failAt["inotify"]; err != nil {
		return -1, err
	}
	fd := f.newFD()
	f.instances = append(f.instances, fd)
	return fd, nil
}

func (f *fakeInotify) addWatch(fd int, path string, mask uint32) (int, error) {
	f.masks = append(f.masks, mask)
	if err := f.failAt["watch "+path]; err != nil {
		return -1, err
	}
	f.watches[fd] = append(f.watches[fd], path)
	return len(f.watches[fd]), nil
}

func (f *fakeInotify) epollCreate(int) (int, error) {
	if err := f.failAt["epoll"]; err != nil {
		return -1, err
	}
	return fakeEpoll, nil
}

func (f *fakeInotify) epollCtl(epfd, op, fd int, event *syscall.EpollEvent) error {
	if op != syscall.EPOLL_CTL_ADD || epfd != fakeEpoll || int(event.Fd) != fd {
		return syscall.EINVAL
	}
	f.added = append(f.added, epollAdd{fd, event.Events})
	return f.failAt["add "+strconv.Itoa(fd)]
}

func (f *fakeInotify) epollWait(epfd int, events []syscall.EpollEvent, msec int) (int, error) {
	f.msecs = append(f.msecs, msec)
	if f.waitErr != nil {
		return -1, f.waitErr
	}
	return copy(events, f.events), nil
}

func (f *fakeInotify) pipe(fds []int, flags int) error {
	if err := f.failAt["pipe"]; err != nil {
		return err
	}
	fds[0], fds[1] = fakeCancelR, fakeCancelW
	return nil
}

func (f *fakeInotify) pidfdOpen(pid int) (int, error) {
	if err := f.failAt["pidfd"]; err != nil {
		return -1, err
	}
	fd := f.newFD()
	f.pidfds[pid] = fd
	return fd, nil
}

func (f *fakeInotify) read(fd int, buf []byte) (int, error) {
	queue := f.reads[fd]
	if len(queue) == 0 {
		return -1, syscall.EAGAIN
	}
	f.reads[fd] = queue[1:]
	return len(buf), queue[0]
}

func (f *fakeInotify) write(fd int, buf []byte) (int, error) {
	f.written = append(f.written, fd)
	return len(buf), f.writeErr
}

func (f *fakeInotify) statfs(path string, buf *syscall.Statfs_t) error {
	buf.Type = ext4Magic
	if magic, ok := f.fstype[path]; ok {
		buf.Type = magic
	}
	return f.statfsErr
}

func (f *fakeInotify) close(fd int) error {
	f.closed = append(f.closed, fd)
	return f.closeErr
}

func newFakeKernelInotify(t *testing.T, f *fakeInotify, output *os.File) *inotifyKernel {
	t.Helper()
	k, err := newInotify(f.port(), output)
	if err != nil {
		t.Fatalf("newInotify = %v, want a kernel", err)
	}
	return k
}

func TestWaiter_RefusesANetworkFilesystem(t *testing.T) {
	t.Parallel()
	f := newFakeInotify()
	f.fstype["/mnt/nfs/ch/loop"] = nfsMagic
	k := newFakeKernelInotify(t, f, nil)

	_, err := k.arm(Targets{Dirs: []string{"/mnt/nfs/ch/loop"}})

	if !errors.Is(err, ErrRefused) || !strings.Contains(err.Error(), "0x6969") || !strings.Contains(err.Error(), "/mnt/nfs/ch/loop") {
		t.Errorf("arm on nfs = %v, want %v naming the path and the magic 0x6969", err, ErrRefused)
	}
	if len(f.instances) != 0 {
		t.Errorf("instances %v, want no inotify instance for a refused filesystem", f.instances)
	}
}

func TestInotifyArm_AcceptsEveryLocalFilesystemOfTheTable(t *testing.T) {
	t.Parallel()
	f := newFakeInotify()
	var dirs []string
	for magic, name := range localFilesystems {
		f.fstype["/"+name] = magic
		dirs = append(dirs, "/"+name)
	}
	k := newFakeKernelInotify(t, f, nil)

	_, err := k.arm(Targets{Dirs: dirs})

	if err != nil || len(f.watches[k.inotify]) != 5 {
		t.Errorf("arm on %v = %v with watches %v, want the five local filesystems armed", dirs, err, f.watches)
	}
}

func TestWaiter_AKernelRefusalFailsLoudly(t *testing.T) {
	t.Parallel()
	f := newFakeInotify()
	k := newFakeKernelInotify(t, f, nil)
	f.failAt["watch /ch/loop"] = syscall.ENOSPC

	_, err := k.arm(Targets{Dirs: []string{"/ch/loop"}})

	if !errors.Is(err, ErrRefused) || !errors.Is(err, syscall.ENOSPC) {
		t.Errorf("arm = %v, want %v wrapping ENOSPC (the inotify watch limit)", err, ErrRefused)
	}
	if !slices.Equal(f.closed, f.instances) || k.inotify != -1 {
		t.Errorf("closed %v, inotify %d, want the new instance %v closed and none kept", f.closed, k.inotify, f.instances)
	}
}

func TestNewInotify_AddsTheCancelPipeAndTheHangup(t *testing.T) {
	t.Parallel()
	r, out, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer out.Close()
	f := newFakeInotify()

	k := newFakeKernelInotify(t, f, out)

	want := []epollAdd{{fakeCancelR, syscall.EPOLLIN}, {int(out.Fd()), syscall.EPOLLHUP | syscall.EPOLLERR}}
	if !reflect.DeepEqual(f.added, want) || k.output != int(out.Fd()) {
		t.Errorf("epoll adds = %+v, output %d, want %+v", f.added, k.output, want)
	}
}

func TestNewInotify_EachFailureIsRefusedAndClosesWhatItOpened(t *testing.T) {
	t.Parallel()
	r, out, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer out.Close()
	cases := []struct {
		name   string
		at     string
		closed []int
	}{
		{"epoll set", "epoll", nil},
		{"cancel pipe", "pipe", []int{fakeEpoll}},
		{"cancel pipe add", "add " + strconv.Itoa(fakeCancelR), []int{fakeCancelR, fakeCancelW, fakeEpoll}},
		{"hangup add", "add " + strconv.Itoa(int(out.Fd())), []int{fakeCancelR, fakeCancelW, fakeEpoll}},
	}
	for _, c := range cases {
		f := newFakeInotify()
		f.failAt[c.at] = syscall.EINVAL

		k, err := newInotify(f.port(), out)

		if k != nil || !errors.Is(err, ErrRefused) || !errors.Is(err, syscall.EINVAL) || !slices.Equal(f.closed, c.closed) {
			t.Errorf("%s: newInotify = %v, %v, closed %v, want a refusal and %v closed", c.name, k, err, f.closed, c.closed)
		}
	}
}

func TestNewInotify_AnUnreadableOutputClosesTheSet(t *testing.T) {
	t.Parallel()
	out, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	f := newFakeInotify()

	k, err := newInotify(f.port(), out)

	if k != nil || err == nil || errors.Is(err, ErrRefused) || !slices.Equal(f.closed, []int{fakeCancelR, fakeCancelW, fakeEpoll}) {
		t.Errorf("newInotify = %v, %v, closed %v, want the stat error and every descriptor closed", k, err, f.closed)
	}
}

func TestInotifyArm_WatchesEachDirectoryOnceWithTheSpecMask(t *testing.T) {
	t.Parallel()
	f := newFakeInotify()
	k := newFakeKernelInotify(t, f, nil)

	_, err := k.arm(Targets{Dirs: []string{"/ch/loop"}, Files: []string{"/ch/loop/seg-0", "/ch/cycle/seg-0"}})

	mask := uint32(syscall.IN_MODIFY | syscall.IN_CREATE | syscall.IN_DELETE | syscall.IN_MOVED_FROM | syscall.IN_MOVED_TO | syscall.IN_DELETE_SELF | syscall.IN_MOVE_SELF)
	if err != nil || !slices.Equal(f.watches[k.inotify], []string{"/ch/loop", "/ch/cycle"}) || !slices.Equal(f.masks, []uint32{mask, mask}) {
		t.Errorf("arm = %v, watches %v, masks %v, want /ch/loop and /ch/cycle once each with the spec mask", err, f.watches, f.masks)
	}
	if !slices.Contains(f.added, epollAdd{k.inotify, syscall.EPOLLIN}) {
		t.Errorf("epoll adds = %+v, want the inotify instance %d", f.added, k.inotify)
	}
}

func TestInotifyArm_ARearmReplacesTheInstance(t *testing.T) {
	t.Parallel()
	f := newFakeInotify()
	k := newFakeKernelInotify(t, f, nil)
	if _, err := k.arm(Targets{Dirs: []string{"/ch/loop"}}); err != nil {
		t.Fatal(err)
	}
	old := k.inotify

	_, err := k.arm(Targets{Dirs: []string{"/ch/loop"}})

	if err != nil || k.inotify == old || !slices.Equal(f.closed, []int{old}) {
		t.Errorf("rearm = %v, inotify %d, closed %v, want a new instance and the old %d closed", err, k.inotify, f.closed, old)
	}
	f.closeErr = syscall.EIO
	if _, err := k.arm(Targets{}); !errors.Is(err, syscall.EIO) {
		t.Errorf("rearm = %v, want the EIO of the old close", err)
	}
}

func TestInotifyArm_InstanceAndEpollFailuresAreRefused(t *testing.T) {
	t.Parallel()
	f := newFakeInotify()
	k := newFakeKernelInotify(t, f, nil)
	f.failAt["inotify"] = syscall.EMFILE
	if _, err := k.arm(Targets{}); !errors.Is(err, ErrRefused) || !errors.Is(err, syscall.EMFILE) {
		t.Errorf("arm = %v, want a refusal wrapping EMFILE (the inotify instance limit)", err)
	}
	delete(f.failAt, "inotify")
	f.failAt["add "+strconv.Itoa(f.nextFD+1)] = syscall.ENOMEM

	_, err := k.arm(Targets{})

	if !errors.Is(err, ErrRefused) || !errors.Is(err, syscall.ENOMEM) || !slices.Equal(f.closed, []int{f.nextFD}) {
		t.Errorf("arm = %v, closed %v, want a refusal wrapping ENOMEM and the instance %d closed", err, f.closed, f.nextFD)
	}
}

func TestInotifyArm_AStatfsFailureIsAPathError(t *testing.T) {
	t.Parallel()
	f := newFakeInotify()
	f.statfsErr = syscall.ENOENT
	k := newFakeKernelInotify(t, f, nil)

	_, err := k.arm(Targets{Files: []string{"/ch/loop/seg"}})

	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) || pathErr.Path != "/ch/loop" || !errors.Is(err, fs.ErrNotExist) || errors.Is(err, ErrRefused) {
		t.Errorf("arm = %v, want a statfs path error for /ch/loop that is not a refusal", err)
	}
}

func TestInotifyArm_ArmsEachPidOnceAndClosesTheOnesLeftOut(t *testing.T) {
	t.Parallel()
	f := newFakeInotify()
	k := newFakeKernelInotify(t, f, nil)
	if _, err := k.arm(Targets{Pids: []int{41, 42}}); err != nil {
		t.Fatal(err)
	}
	fd41, fd42 := f.pidfds[41], f.pidfds[42]
	f.closed = nil

	gone, err := k.arm(Targets{Pids: []int{42, 43}})

	if err != nil || gone != nil || !reflect.DeepEqual(k.pidfds, map[int]int{42: fd42, 43: f.pidfds[43]}) {
		t.Errorf("arm = %v, %v, pidfds %v, want 42 kept and 43 added", gone, err, k.pidfds)
	}
	if !slices.Contains(f.closed, fd41) || slices.Contains(f.closed, fd42) {
		t.Errorf("closed %v, want the pidfd %d of 41 closed and %d kept", f.closed, fd41, fd42)
	}
}

func TestInotifyArm_AGonePidIsReportedAndOtherFailuresAreRefused(t *testing.T) {
	t.Parallel()
	f := newFakeInotify()
	k := newFakeKernelInotify(t, f, nil)
	f.failAt["pidfd"] = syscall.ESRCH

	gone, err := k.arm(Targets{Pids: []int{41}})

	if err != nil || !slices.Equal(gone, []int{41}) || len(k.pidfds) != 0 {
		t.Errorf("arm = %v, %v, pidfds %v, want 41 gone and not watched", gone, err, k.pidfds)
	}
	f.failAt["pidfd"] = syscall.ENOSYS
	if _, err := k.arm(Targets{Pids: []int{42}}); !errors.Is(err, ErrRefused) || !errors.Is(err, syscall.ENOSYS) {
		t.Errorf("arm = %v, want a refusal wrapping ENOSYS", err)
	}
	delete(f.failAt, "pidfd")
	f.failAt["add "+strconv.Itoa(f.nextFD+2)] = syscall.EPERM
	f.closed = nil
	_, err = k.arm(Targets{Pids: []int{43}})
	if !errors.Is(err, ErrRefused) || !errors.Is(err, syscall.EPERM) || !slices.Contains(f.closed, f.pidfds[43]) {
		t.Errorf("arm = %v, closed %v, want a refusal wrapping EPERM and the pidfd closed", err, f.closed)
	}
}

func TestInotifyWait_DecodesEachDescriptorAndDrainsTheQueues(t *testing.T) {
	t.Parallel()
	r, out, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer out.Close()
	f := newFakeInotify()
	k := newFakeKernelInotify(t, f, out)
	if _, err := k.arm(Targets{Dirs: []string{"/ch/loop"}, Pids: []int{42}}); err != nil {
		t.Fatal(err)
	}
	f.reads[k.inotify] = []error{nil, nil}
	f.reads[fakeCancelR] = []error{nil}
	f.events = []syscall.EpollEvent{{Fd: fakeCancelR}, {Fd: 99}, {Fd: int32(k.inotify)}, {Fd: int32(f.pidfds[42])}, {Fd: int32(out.Fd())}}

	got, err := k.wait(1500*time.Millisecond + time.Microsecond)

	want := Wake{Changed: true, Exited: []int{42}, Hangup: true}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("wait = %+v, %v, want %+v: a cancel byte and an unknown descriptor are no wake", got, err, want)
	}
	if len(f.reads[k.inotify]) != 0 || len(f.reads[fakeCancelR]) != 0 {
		t.Errorf("reads left %v, want each queue read to EAGAIN", f.reads)
	}
	if !slices.Equal(f.msecs, []int{1501}) || len(k.pidfds) != 0 || !slices.Contains(f.closed, f.pidfds[42]) {
		t.Errorf("msecs %v, pidfds %v, closed %v, want 1501 ms rounded up and the exited pidfd closed", f.msecs, k.pidfds, f.closed)
	}
}

func TestInotifyWait_NoTimeoutTheKernelErrorAndAReadError(t *testing.T) {
	t.Parallel()
	f := newFakeInotify()
	k := newFakeKernelInotify(t, f, nil)
	f.waitErr = syscall.EINTR

	got, err := k.wait(noTimeout)

	if !errors.Is(err, syscall.EINTR) || !reflect.DeepEqual(got, Wake{}) || !slices.Equal(f.msecs, []int{-1}) {
		t.Errorf("wait = %+v, %v with msecs %v, want EINTR after one call with no timeout", got, err, f.msecs)
	}
	f.waitErr = nil
	f.reads[fakeCancelR] = []error{syscall.EBADF}
	f.events = []syscall.EpollEvent{{Fd: fakeCancelR}}
	if _, err := k.wait(noTimeout); !errors.Is(err, syscall.EBADF) {
		t.Errorf("wait = %v, want the EBADF of the drain", err)
	}
}

func TestInotifyCancel_WritesOneByteAndAFullPipeIsAlreadyACancel(t *testing.T) {
	t.Parallel()
	f := newFakeInotify()
	k := newFakeKernelInotify(t, f, nil)

	first := k.cancel()
	f.writeErr = syscall.EAGAIN
	full := k.cancel()
	f.writeErr = syscall.EBADF
	broken := k.cancel()

	if first != nil || full != nil || !errors.Is(broken, syscall.EBADF) || !slices.Equal(f.written, []int{fakeCancelW, fakeCancelW, fakeCancelW}) {
		t.Errorf("cancel = %v, %v, %v on %v, want nil, nil (a pending cancel) and EBADF", first, full, broken, f.written)
	}
}

func TestInotifyClose_ClosesEveryDescriptorAndJoinsTheErrors(t *testing.T) {
	t.Parallel()
	f := newFakeInotify()
	k := newFakeKernelInotify(t, f, nil)
	if _, err := k.arm(Targets{Dirs: []string{"/ch/loop"}, Pids: []int{42}}); err != nil {
		t.Fatal(err)
	}
	f.closeErr = syscall.EIO

	err := k.close()

	want := []int{k.inotify, fakeCancelR, fakeCancelW, f.pidfds[42], fakeEpoll}
	if !errors.Is(err, syscall.EIO) || !slices.Equal(f.closed, want) {
		t.Errorf("close = %v, closed %v, want EIO and %v closed", err, f.closed, want)
	}
}

func TestPidfdOpen_AnInvalidPidIsAKernelError(t *testing.T) {
	t.Parallel()
	fd, err := pidfdOpen(-1)

	var errno syscall.Errno
	if fd != -1 || !errors.As(err, &errno) {
		t.Errorf("pidfdOpen(-1) = %d, %v, want -1 and a kernel errno", fd, err)
	}
}

func TestInotifyArm_ASignExtendedMagicIsStillALocalFilesystem(t *testing.T) {
	t.Parallel()
	f := newFakeInotify()
	f.fstype["/btrfs"] = int64(int32(-0x6edc97c2))
	k := newFakeKernelInotify(t, f, nil)

	_, err := k.arm(Targets{Dirs: []string{"/btrfs"}})

	if err != nil || !slices.Equal(f.watches[k.inotify], []string{"/btrfs"}) {
		t.Errorf("arm on btrfs with a 32-bit magic sign-extended to %d = %v, want it armed", f.fstype["/btrfs"], err)
	}
}

func TestInotifyArm_AFailedCloseOfTheOldInstanceStillArmsThePids(t *testing.T) {
	t.Parallel()
	f := newFakeInotify()
	k := newFakeKernelInotify(t, f, nil)
	if _, err := k.arm(Targets{Dirs: []string{"/ch/loop"}}); err != nil {
		t.Fatal(err)
	}
	f.closeErr = syscall.EIO

	_, err := k.arm(Targets{Dirs: []string{"/ch/loop"}, Pids: []int{42}})

	if !errors.Is(err, syscall.EIO) || k.pidfds[42] != f.pidfds[42] || f.pidfds[42] == 0 {
		t.Errorf("arm = %v, pidfds %v, want the EIO of the old close and pid 42 armed", err, k.pidfds)
	}
}

func TestInotifyWait_ADrainStopsAtItsCapAndTheRestWakesTheNextWait(t *testing.T) {
	t.Parallel()
	f := newFakeInotify()
	k := newFakeKernelInotify(t, f, nil)
	if _, err := k.arm(Targets{Dirs: []string{"/ch/loop"}}); err != nil {
		t.Fatal(err)
	}
	f.reads[k.inotify] = make([]error, maxDrainReads+3)
	f.events = []syscall.EpollEvent{{Fd: int32(k.inotify)}}

	got, err := k.wait(noTimeout)

	if err != nil || !got.Changed || len(f.reads[k.inotify]) != 3 {
		t.Errorf("wait = %+v, %v with %d reads left, want Changed and 3 reads left for the next wait", got, err, len(f.reads[k.inotify]))
	}
}
