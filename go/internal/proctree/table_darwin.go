//go:build darwin

package proctree

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

const (
	kernProcUID      = 5
	kinfoStartOffset = 0
	kinfoPidOffset   = 40
	kinfoCommOffset  = 243
	kinfoCommBytes   = 17
	kinfoPpidOffset  = 560
	kinfoPgidOffset  = 564
	tableAttempts    = 5
	tableSlackRows   = 32
)

var errKinfoLayout = errors.New("proctree: the kinfo_proc layout of this host does not match the decoder")

func readTable(uid int) ([]Process, error) {
	return readTableVia(sysctl, uid)
}

func readTableVia(call sysctlCall, uid int) ([]Process, error) {
	raw, err := kernTable(call, []int32{ctlKern, kernProc, kernProcUID, int32(uid)})
	if err != nil {
		return nil, err
	}
	table := decodeTable(raw)
	if err := verifySelfRow(call, table); err != nil {
		return nil, err
	}
	return table, nil
}

func kernTable(call sysctlCall, mib []int32) ([]byte, error) {
	for range tableAttempts {
		size := uintptr(0)
		if err := call(mib, nil, &size); err != nil {
			return nil, err
		}
		buf := make([]byte, int(size)+tableSlackRows*kinfoProcBytes)
		size = uintptr(len(buf))
		err := call(mib, &buf[0], &size)
		if err == nil {
			return buf[:size], nil
		}
		if !errors.Is(err, syscall.ENOMEM) {
			return nil, err
		}
	}
	return nil, syscall.ENOMEM
}

func decodeTable(raw []byte) []Process {
	table := make([]Process, 0, len(raw)/kinfoProcBytes)
	for rest := raw; len(rest) >= kinfoProcBytes; rest = rest[kinfoProcBytes:] {
		table = append(table, decodeRow(rest[:kinfoProcBytes]))
	}
	return table
}

func decodeRow(record []byte) Process {
	sec, _ := startFields(record)
	return Process{
		Pid:     int(int32(binary.LittleEndian.Uint32(record[kinfoPidOffset:]))),
		Ppid:    int(int32(binary.LittleEndian.Uint32(record[kinfoPpidOffset:]))),
		Pgid:    int(int32(binary.LittleEndian.Uint32(record[kinfoPgidOffset:]))),
		Started: time.Unix(sec, 0),
		Comm:    kinfoComm(record),
	}
}

func kinfoComm(record []byte) string {
	name := record[kinfoCommOffset : kinfoCommOffset+kinfoCommBytes]
	if end := bytes.IndexByte(name, 0); end >= 0 {
		name = name[:end]
	}
	return string(name)
}

func verifySelfRow(call sysctlCall, table []Process) error {
	record, err := kinfoRecord(call, os.Getpid())
	if err != nil {
		return err
	}
	self := decodeRow(record)
	row, found := selfOf(table)
	if !found || self.Pid != os.Getpid() || row.Ppid != os.Getppid() || row.Pgid != syscall.Getpgrp() || !row.Started.Equal(self.Started) {
		return fmt.Errorf("%w: the row of pid %d reads %+v", errKinfoLayout, os.Getpid(), row)
	}
	return nil
}

func selfOf(table []Process) (Process, bool) {
	for _, p := range table {
		if p.Pid == os.Getpid() {
			return p, true
		}
	}
	return Process{}, false
}
