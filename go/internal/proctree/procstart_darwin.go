//go:build darwin

package proctree

import (
	"encoding/binary"
	"fmt"
	"syscall"
	"time"
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
	sec, usec, err := kinfoStart(call, pid)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d.%06d", sec, usec), nil
}

func readProcStartTime(pid int) (time.Time, error) {
	return startTimeVia(sysctl, pid)
}

func startTimeVia(call sysctlCall, pid int) (time.Time, error) {
	sec, usec, err := kinfoStart(call, pid)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(sec, int64(usec)*int64(time.Microsecond)), nil
}

func kinfoStart(call sysctlCall, pid int) (int64, int32, error) {
	buf := make([]byte, kinfoProcBytes)
	size := uintptr(len(buf))
	if err := call([]int32{ctlKern, kernProc, kernProcPID, int32(pid)}, &buf[0], &size); err != nil {
		return 0, 0, err
	}
	if size < kinfoProcBytes {
		return 0, 0, syscall.ESRCH
	}
	return int64(binary.LittleEndian.Uint64(buf[0:8])), int32(binary.LittleEndian.Uint32(buf[8:12])), nil
}
