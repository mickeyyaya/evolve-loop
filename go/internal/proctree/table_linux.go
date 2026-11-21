//go:build linux

package proctree

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type fileReader func(path string) ([]byte, error)

func readTable(uid int) ([]Process, error) {
	return readTableAt("/proc", os.ReadFile, uid)
}

func readTableAt(root string, read fileReader, uid int) ([]Process, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	boot, err := bootTime(root)
	if err != nil {
		return nil, err
	}
	scan := procScan{root: root, read: read, boot: boot, uid: uid}
	var table []Process
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		row, keep, err := scan.row(pid)
		if err != nil {
			return nil, err
		}
		if keep {
			table = append(table, row)
		}
	}
	return table, nil
}

type procScan struct {
	root string
	read fileReader
	boot time.Time
	uid  int
}

func (s procScan) row(pid int) (Process, bool, error) {
	dir := s.root + "/" + strconv.Itoa(pid)
	status, err := s.read(dir + "/status")
	if err != nil {
		return Process{}, false, skippable(err)
	}
	owner, err := effectiveUID(string(status))
	if err != nil || owner != s.uid {
		return Process{}, false, err
	}
	stat, err := s.read(dir + "/stat")
	if err != nil {
		return Process{}, false, skippable(err)
	}
	parsed, err := parseStat(string(stat))
	if err != nil {
		return Process{}, false, err
	}
	ticks, err := parsed.startTicks()
	if err != nil {
		return Process{}, false, err
	}
	started := startedAt(s.boot, ticks).Truncate(time.Second)
	return Process{Pid: pid, Ppid: parsed.ppid, Pgid: parsed.pgrp, Started: started, Comm: commFrom(Process{Comm: parsed.comm})}, true, nil
}

func vanished(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) || errors.Is(err, syscall.ESRCH)
}

func skippable(err error) error {
	if vanished(err) {
		return nil
	}
	return err
}

func effectiveUID(status string) (int, error) {
	for _, line := range strings.Split(status, "\n") {
		if rest, found := strings.CutPrefix(line, "Uid:"); found {
			fields := strings.Fields(rest)
			if len(fields) < 2 {
				break
			}
			return strconv.Atoi(fields[1])
		}
	}
	return 0, fmt.Errorf("%w: no Uid line", errProcStatMalformed)
}

func commOfAt(root string, pid int) (string, error) {
	parsed, err := statAt(root, pid)
	if err != nil {
		return "", err
	}
	return commFrom(Process{Comm: parsed.comm}), nil
}
