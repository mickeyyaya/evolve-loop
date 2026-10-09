//go:build darwin

package wake

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"syscall"
	"time"
)

const (
	termiosRequest = syscall.TIOCGETA
	userIdent      = 1
	dirNotes       = syscall.NOTE_WRITE
	fileNotes      = syscall.NOTE_WRITE | syscall.NOTE_EXTEND | syscall.NOTE_DELETE | syscall.NOTE_RENAME | syscall.NOTE_ATTRIB
	addCleared     = syscall.EV_ADD | syscall.EV_CLEAR
	watchOpenMode  = syscall.O_EVTONLY | syscall.O_CLOEXEC
)

var localFilesystems = map[string]bool{"apfs": true, "hfs": true}

type kqueuePort struct {
	kqueue     func() (int, error)
	kevent     func(kq int, changes, events []syscall.Kevent_t, timeout *syscall.Timespec) (int, error)
	open       func(path string, mode int, perm uint32) (int, error)
	statfs     func(path string, buf *syscall.Statfs_t) error
	close      func(fd int) error
	isTerminal func(fd int) bool
}

var hostKqueue = kqueuePort{syscall.Kqueue, syscall.Kevent, syscall.Open, syscall.Statfs, syscall.Close, isTerminal}

type kqueueKernel struct {
	port    kqueuePort
	kq      int
	watched []int
	pids    map[int]bool
}

func newKernel(output *os.File) (kernel, error) {
	return newKqueue(hostKqueue, output)
}

func newKqueue(port kqueuePort, output *os.File) (*kqueueKernel, error) {
	kq, err := port.kqueue()
	if err != nil {
		return nil, refuseWatch("a new kqueue", err)
	}
	k := &kqueueKernel{port: port, kq: kq, pids: map[int]bool{}}
	if err := k.armOwnEvents(output); err != nil {
		return nil, errors.Join(err, port.close(kq))
	}
	return k, nil
}

func (k *kqueueKernel) armOwnEvents(output *os.File) error {
	if err := k.register(syscall.Kevent_t{Ident: userIdent, Filter: syscall.EVFILT_USER, Flags: addCleared}); err != nil {
		return refuseWatch("the cancel event", err)
	}
	fd, armed, err := hangupFD(output, k.port.isTerminal)
	if err != nil || !armed {
		return err
	}
	if err := k.register(syscall.Kevent_t{Ident: uint64(fd), Filter: syscall.EVFILT_WRITE, Flags: addCleared}); err != nil {
		return refuseWatch("the output hangup", err)
	}
	return nil
}

func (k *kqueueKernel) register(change syscall.Kevent_t) error {
	_, err := k.port.kevent(k.kq, []syscall.Kevent_t{change}, nil, nil)
	return err
}

func (k *kqueueKernel) arm(t Targets) ([]int, error) {
	fds, err := k.watchPaths(t)
	if err != nil {
		return nil, err
	}
	old := k.watched
	k.watched = fds
	if err := k.closeAll(old); err != nil {
		return nil, err
	}
	return k.watchPids(t.Pids)
}

func (k *kqueueKernel) watchPaths(t Targets) ([]int, error) {
	var fds []int
	for _, p := range append(notedPaths(t.Dirs, dirNotes), notedPaths(t.Files, fileNotes)...) {
		fd, err := k.watchPath(p.path, p.notes)
		if err != nil {
			return nil, errors.Join(err, k.closeAll(fds))
		}
		fds = append(fds, fd)
	}
	return fds, nil
}

type notedPath struct {
	path  string
	notes uint32
}

func notedPaths(paths []string, notes uint32) []notedPath {
	out := make([]notedPath, 0, len(paths))
	for _, p := range paths {
		out = append(out, notedPath{p, notes})
	}
	return out
}

func (k *kqueueKernel) watchPath(path string, notes uint32) (int, error) {
	if err := k.checkFilesystem(path); err != nil {
		return -1, err
	}
	fd, err := k.port.open(path, watchOpenMode, 0)
	if err != nil {
		return -1, refuseWatch(path, err)
	}
	if err := k.register(syscall.Kevent_t{Ident: uint64(fd), Filter: syscall.EVFILT_VNODE, Flags: addCleared, Fflags: notes}); err != nil {
		return -1, errors.Join(refuseWatch(path, err), k.port.close(fd))
	}
	return fd, nil
}

func (k *kqueueKernel) checkFilesystem(path string) error {
	var st syscall.Statfs_t
	if err := k.port.statfs(path, &st); err != nil {
		return statfsError(path, err)
	}
	name := filesystemName(st.Fstypename)
	if !localFilesystems[name] {
		return fmt.Errorf("%w: %s is on the filesystem %q, which has no kernel wake", ErrRefused, path, name)
	}
	return nil
}

func filesystemName(raw [16]int8) string {
	name := make([]byte, 0, len(raw))
	for _, c := range raw {
		if c == 0 {
			break
		}
		name = append(name, byte(c))
	}
	return string(name)
}

func (k *kqueueKernel) watchPids(pids []int) ([]int, error) {
	var gone []int
	for _, pid := range pids {
		if k.pids[pid] {
			continue
		}
		err := k.register(syscall.Kevent_t{Ident: uint64(pid), Filter: syscall.EVFILT_PROC, Flags: addCleared, Fflags: syscall.NOTE_EXIT})
		if errors.Is(err, syscall.ESRCH) {
			gone = append(gone, pid)
			continue
		}
		if err != nil {
			return gone, refuseWatch(fmt.Sprintf("the exit of pid %d", pid), err)
		}
		k.pids[pid] = true
	}
	return gone, k.forgetPidsOutside(pids)
}

func (k *kqueueKernel) forgetPidsOutside(pids []int) error {
	var errs []error
	for pid := range k.pids {
		if slices.Contains(pids, pid) {
			continue
		}
		delete(k.pids, pid)
		err := k.register(syscall.Kevent_t{Ident: uint64(pid), Filter: syscall.EVFILT_PROC, Flags: syscall.EV_DELETE})
		if !errors.Is(err, syscall.ENOENT) && !errors.Is(err, syscall.ESRCH) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (k *kqueueKernel) wait(left time.Duration) (Wake, error) {
	var events [16]syscall.Kevent_t
	n, err := k.port.kevent(k.kq, nil, events[:], timespec(left))
	if err != nil {
		return Wake{}, err
	}
	return k.decode(events[:n]), nil
}

func timespec(left time.Duration) *syscall.Timespec {
	if left == noTimeout {
		return nil
	}
	ts := syscall.NsecToTimespec(int64(left))
	return &ts
}

func (k *kqueueKernel) decode(events []syscall.Kevent_t) Wake {
	var wake Wake
	for _, ev := range events {
		switch ev.Filter {
		case syscall.EVFILT_VNODE:
			wake.Changed = true
		case syscall.EVFILT_WRITE:
			wake.Hangup = wake.Hangup || ev.Flags&syscall.EV_EOF != 0
		case syscall.EVFILT_PROC:
			wake.Exited = k.exited(wake.Exited, int(ev.Ident))
		}
	}
	return wake
}

func (k *kqueueKernel) exited(exited []int, pid int) []int {
	if !k.pids[pid] {
		return exited
	}
	delete(k.pids, pid)
	return append(exited, pid)
}

func (k *kqueueKernel) cancel() error {
	return k.register(syscall.Kevent_t{Ident: userIdent, Filter: syscall.EVFILT_USER, Fflags: syscall.NOTE_TRIGGER})
}

func (k *kqueueKernel) close() error {
	return errors.Join(k.closeAll(k.watched), k.port.close(k.kq))
}

func (k *kqueueKernel) closeAll(fds []int) error {
	var errs []error
	for _, fd := range fds {
		errs = append(errs, k.port.close(fd))
	}
	return errors.Join(errs...)
}
