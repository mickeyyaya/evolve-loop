//go:build darwin

package proctree

import (
	"syscall"
	"unsafe"
)

const (
	ctlKern       = 1
	kernProcArgs2 = 49
)

type sysctlCall func(mib []int32, out *byte, size *uintptr) error

func readProcArgs(pid int) ([]string, map[string]string, error) {
	return readProcArgsVia(sysctl, pid)
}

func readProcArgsVia(call sysctlCall, pid int) ([]string, map[string]string, error) {
	mib := []int32{ctlKern, kernProcArgs2, int32(pid)}
	size := uintptr(0)
	if err := call(mib, nil, &size); err != nil {
		return nil, nil, err
	}
	if size == 0 {
		return nil, nil, errProcArgsTruncated
	}
	buf := make([]byte, size)
	if err := call(mib, &buf[0], &size); err != nil {
		return nil, nil, err
	}
	return parseProcArgs2(buf[:size])
}

func sysctl(mib []int32, out *byte, size *uintptr) error {
	_, _, errno := syscall.Syscall6(syscall.SYS___SYSCTL, uintptr(unsafe.Pointer(&mib[0])), uintptr(len(mib)), uintptr(unsafe.Pointer(out)), uintptr(unsafe.Pointer(size)), 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}
