//go:build darwin

package proctree

import "strings"

func commFrom(p Process) string {
	if len(p.Args) == 0 {
		return p.Comm
	}
	if name := strings.TrimSpace(p.Args[0]); name != "" {
		return name
	}
	return p.Comm
}

func readComm(pid int) (string, error) {
	return commOfVia(sysctl, pid)
}

func commOfVia(call sysctlCall, pid int) (string, error) {
	record, err := kinfoRecord(call, pid)
	if err != nil {
		return "", err
	}
	p := Process{Pid: pid, Comm: kinfoComm(record)}
	if args, _, err := readProcArgsVia(call, pid); err == nil {
		p.Args = args
	}
	return commFrom(p), nil
}
