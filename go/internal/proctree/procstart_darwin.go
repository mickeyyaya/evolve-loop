//go:build darwin

package proctree

import (
	"encoding/binary"
	"fmt"
	"syscall"
)

const (
	kernProc       = 14
	kernProcPID    = 1
	kinfoProcBytes = 648
)

func readProcStart(pid int) (string, error) {
	return startOfVia(sysctl, pid)
}

func startOfVia(call sysctlCall, pid int) (string, error) {
	buf := make([]byte, kinfoProcBytes)
	size := uintptr(len(buf))
	if err := call([]int32{ctlKern, kernProc, kernProcPID, int32(pid)}, &buf[0], &size); err != nil {
		return "", err
	}
	if size < kinfoProcBytes {
		return "", syscall.ESRCH
	}
	sec := int64(binary.LittleEndian.Uint64(buf[0:8]))
	usec := int32(binary.LittleEndian.Uint32(buf[8:12]))
	return fmt.Sprintf("%d.%06d", sec, usec), nil
}
