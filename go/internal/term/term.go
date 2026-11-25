//go:build darwin || linux

package term

import (
	"io"
	"os"
	"syscall"
	"unsafe"
)

func IsFD(fd int) bool {
	var termios syscall.Termios
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), termiosRequest, uintptr(unsafe.Pointer(&termios)))
	return errno == 0
}

func IsTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	return ok && IsFD(int(f.Fd()))
}
