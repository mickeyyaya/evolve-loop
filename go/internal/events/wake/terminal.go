//go:build darwin || linux

package wake

import (
	"syscall"
	"unsafe"
)

func isTerminal(fd int) bool {
	var termios syscall.Termios
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), termiosRequest, uintptr(unsafe.Pointer(&termios)))
	return errno == 0
}
