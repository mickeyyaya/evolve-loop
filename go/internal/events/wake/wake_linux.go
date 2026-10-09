//go:build linux

package wake

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"time"
)

const (
	termiosRequest = syscall.TCGETS
	sysPidfdOpen   = 434
	dirMask        = syscall.IN_MODIFY | syscall.IN_CREATE | syscall.IN_DELETE | syscall.IN_MOVED_FROM | syscall.IN_MOVED_TO | syscall.IN_DELETE_SELF | syscall.IN_MOVE_SELF
	hangupEvents   = syscall.EPOLLHUP | syscall.EPOLLERR
	maxDrainReads  = 16
)

var localFilesystems = map[int64]string{
	0xef53:     "ext4",
	0x58465342: "xfs",
	0x9123683e: "btrfs",
	0x01021994: "tmpfs",
	0x794c7630: "overlay",
}

type inotifyPort struct {
	inotifyInit func(flags int) (int, error)
	addWatch    func(fd int, path string, mask uint32) (int, error)
	epollCreate func(flags int) (int, error)
	epollCtl    func(epfd, op, fd int, event *syscall.EpollEvent) error
	epollWait   func(epfd int, events []syscall.EpollEvent, msec int) (int, error)
	pipe        func(fds []int, flags int) error
	pidfdOpen   func(pid int) (int, error)
	read        func(fd int, buf []byte) (int, error)
	write       func(fd int, buf []byte) (int, error)
	statfs      func(path string, buf *syscall.Statfs_t) error
	close       func(fd int) error
	isTerminal  func(fd int) bool
}

var hostInotify = inotifyPort{
	syscall.InotifyInit1, syscall.InotifyAddWatch, syscall.EpollCreate1, syscall.EpollCtl, syscall.EpollWait,
	syscall.Pipe2, pidfdOpen, syscall.Read, syscall.Write, syscall.Statfs, syscall.Close, isTerminal,
}

func pidfdOpen(pid int) (int, error) {
	fd, _, errno := syscall.Syscall(sysPidfdOpen, uintptr(pid), 0, 0)
	if errno != 0 {
		return -1, errno
	}
	return int(fd), nil
}

type inotifyKernel struct {
	port    inotifyPort
	epoll   int
	inotify int
	cancelR int
	cancelW int
	output  int
	pidfds  map[int]int
}

func newKernel(output *os.File) (kernel, error) {
	return newInotify(hostInotify, output)
}

func newInotify(port inotifyPort, output *os.File) (*inotifyKernel, error) {
	epoll, err := port.epollCreate(syscall.EPOLL_CLOEXEC)
	if err != nil {
		return nil, refuseWatch("a new epoll set", err)
	}
	k := &inotifyKernel{port: port, epoll: epoll, inotify: -1, cancelR: -1, cancelW: -1, output: -1, pidfds: map[int]int{}}
	if err := k.armOwnEvents(output); err != nil {
		return nil, errors.Join(err, k.close())
	}
	return k, nil
}

func (k *inotifyKernel) armOwnEvents(output *os.File) error {
	fds := make([]int, 2)
	if err := k.port.pipe(fds, syscall.O_NONBLOCK|syscall.O_CLOEXEC); err != nil {
		return refuseWatch("the cancel pipe", err)
	}
	k.cancelR, k.cancelW = fds[0], fds[1]
	if err := k.add(k.cancelR, syscall.EPOLLIN); err != nil {
		return refuseWatch("the cancel pipe", err)
	}
	fd, armed, err := hangupFD(output, k.port.isTerminal)
	if err != nil || !armed {
		return err
	}
	if err := k.add(fd, hangupEvents); err != nil {
		return refuseWatch("the output hangup", err)
	}
	k.output = fd
	return nil
}

func (k *inotifyKernel) add(fd int, events uint32) error {
	return k.port.epollCtl(k.epoll, syscall.EPOLL_CTL_ADD, fd, &syscall.EpollEvent{Events: events, Fd: int32(fd)})
}

func (k *inotifyKernel) arm(t Targets) ([]int, error) {
	in, err := k.watchDirs(watchedDirs(t))
	if err != nil {
		return nil, err
	}
	old := k.inotify
	k.inotify = in
	closeErr := k.closeIfOpen(old)
	gone, err := k.watchPids(t.Pids)
	return gone, errors.Join(closeErr, err)
}

func watchedDirs(t Targets) []string {
	dirs := slices.Clone(t.Dirs)
	for _, f := range t.Files {
		if dir := filepath.Dir(f); !slices.Contains(dirs, dir) {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

func (k *inotifyKernel) watchDirs(dirs []string) (int, error) {
	for _, dir := range dirs {
		if err := k.checkFilesystem(dir); err != nil {
			return -1, err
		}
	}
	in, err := k.port.inotifyInit(syscall.IN_NONBLOCK | syscall.IN_CLOEXEC)
	if err != nil {
		return -1, refuseWatch("a new inotify instance", err)
	}
	for _, dir := range dirs {
		if _, err := k.port.addWatch(in, dir, dirMask); err != nil {
			return -1, errors.Join(refuseWatch(dir, err), k.port.close(in))
		}
	}
	if err := k.add(in, syscall.EPOLLIN); err != nil {
		return -1, errors.Join(refuseWatch("the inotify instance", err), k.port.close(in))
	}
	return in, nil
}

func (k *inotifyKernel) checkFilesystem(path string) error {
	var st syscall.Statfs_t
	if err := k.port.statfs(path, &st); err != nil {
		return statfsError(path, err)
	}
	magic := int64(st.Type) & 0xffffffff
	if _, ok := localFilesystems[magic]; !ok {
		return fmt.Errorf("%w: %s is on the filesystem 0x%x, which has no kernel wake", ErrRefused, path, magic)
	}
	return nil
}

func (k *inotifyKernel) watchPids(pids []int) ([]int, error) {
	var gone []int
	for _, pid := range pids {
		if _, ok := k.pidfds[pid]; ok {
			continue
		}
		fd, err := k.watchPid(pid)
		if errors.Is(err, syscall.ESRCH) {
			gone = append(gone, pid)
			continue
		}
		if err != nil {
			return gone, err
		}
		k.pidfds[pid] = fd
	}
	return gone, k.forgetPidsOutside(pids)
}

func (k *inotifyKernel) watchPid(pid int) (int, error) {
	fd, err := k.port.pidfdOpen(pid)
	if err != nil {
		return -1, refuseWatch(fmt.Sprintf("the exit of pid %d", pid), err)
	}
	if err := k.add(fd, syscall.EPOLLIN); err != nil {
		return -1, errors.Join(refuseWatch(fmt.Sprintf("the exit of pid %d", pid), err), k.port.close(fd))
	}
	return fd, nil
}

func (k *inotifyKernel) forgetPidsOutside(pids []int) error {
	var errs []error
	for pid, fd := range k.pidfds {
		if !slices.Contains(pids, pid) {
			delete(k.pidfds, pid)
			errs = append(errs, k.port.close(fd))
		}
	}
	return errors.Join(errs...)
}

func (k *inotifyKernel) wait(left time.Duration) (Wake, error) {
	var events [16]syscall.EpollEvent
	n, err := k.port.epollWait(k.epoll, events[:], milliseconds(left))
	if err != nil {
		return Wake{}, err
	}
	return k.decode(events[:n])
}

func milliseconds(left time.Duration) int {
	if left == noTimeout {
		return -1
	}
	return int((left + time.Millisecond - 1) / time.Millisecond)
}

func (k *inotifyKernel) decode(events []syscall.EpollEvent) (Wake, error) {
	var wake Wake
	var errs []error
	for _, ev := range events {
		fd := int(ev.Fd)
		switch fd {
		case k.inotify:
			wake.Changed = true
			errs = append(errs, k.drain(fd))
		case k.cancelR:
			errs = append(errs, k.drain(fd))
		case k.output:
			wake.Hangup = true
		default:
			exited, err := k.exited(wake.Exited, fd)
			wake.Exited = exited
			errs = append(errs, err)
		}
	}
	return wake, errors.Join(errs...)
}

func (k *inotifyKernel) drain(fd int) error {
	buf := make([]byte, 4096)
	for range maxDrainReads {
		_, err := k.port.read(fd, buf)
		if errors.Is(err, syscall.EAGAIN) {
			return nil
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (k *inotifyKernel) exited(exited []int, fd int) ([]int, error) {
	for pid, pidfd := range k.pidfds {
		if pidfd == fd {
			delete(k.pidfds, pid)
			return append(exited, pid), k.port.close(fd)
		}
	}
	return exited, nil
}

func (k *inotifyKernel) cancel() error {
	_, err := k.port.write(k.cancelW, []byte{1})
	if errors.Is(err, syscall.EAGAIN) {
		return nil
	}
	return err
}

func (k *inotifyKernel) close() error {
	errs := []error{k.closeIfOpen(k.inotify), k.closeIfOpen(k.cancelR), k.closeIfOpen(k.cancelW)}
	for _, fd := range k.pidfds {
		errs = append(errs, k.port.close(fd))
	}
	return errors.Join(append(errs, k.port.close(k.epoll))...)
}

func (k *inotifyKernel) closeIfOpen(fd int) error {
	if fd < 0 {
		return nil
	}
	return k.port.close(fd)
}
