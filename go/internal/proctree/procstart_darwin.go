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
	record, err := kinfoRecord(call, pid)
	if err != nil {
		return "", err
	}
	sec, usec := startFields(record)
	return fmt.Sprintf("%d.%06d", sec, usec), nil
}

func readProcStartTime(pid int) (time.Time, error) {
	return startTimeVia(sysctl, pid)
}

func startTimeVia(call sysctlCall, pid int) (time.Time, error) {
	record, err := kinfoRecord(call, pid)
	if err != nil {
		return time.Time{}, err
	}
	sec, usec := startFields(record)
	return time.Unix(sec, int64(usec)*int64(time.Microsecond)), nil
}

func kinfoRecord(call sysctlCall, pid int) ([]byte, error) {
	buf := make([]byte, kinfoProcBytes)
	size := uintptr(len(buf))
	if err := call([]int32{ctlKern, kernProc, kernProcPID, int32(pid)}, &buf[0], &size); err != nil {
		return nil, err
	}
	if size < kinfoProcBytes {
		return nil, syscall.ESRCH
	}
	return buf, nil
}

func startFields(record []byte) (int64, int32) {
	return int64(binary.LittleEndian.Uint64(record[kinfoStartOffset : kinfoStartOffset+8])), int32(binary.LittleEndian.Uint32(record[kinfoStartOffset+8 : kinfoStartOffset+12]))
}
