//go:build darwin

package proctree

import (
	"syscall"
	"unsafe"
)

const (
	ctlKern        = 1
	kernProcArgs2  = 49
	procArgsMibLen = 3
)

func readProcArgs(pid int) ([]string, map[string]string, error) {
	mib := [procArgsMibLen]int32{ctlKern, kernProcArgs2, int32(pid)}
	size := uintptr(0)
	if err := sysctl(&mib, nil, &size); err != nil {
		return nil, nil, err
	}
	if size == 0 {
		return nil, nil, errProcArgsTruncated
	}
	buf := make([]byte, size)
	if err := sysctl(&mib, &buf[0], &size); err != nil {
		return nil, nil, err
	}
	return parseProcArgs2(buf[:size])
}

func sysctl(mib *[procArgsMibLen]int32, out *byte, size *uintptr) error {
	_, _, errno := syscall.Syscall6(syscall.SYS___SYSCTL, uintptr(unsafe.Pointer(mib)), procArgsMibLen, uintptr(unsafe.Pointer(out)), uintptr(unsafe.Pointer(size)), 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}
