package swarmrunner

import (
	"fmt"
	"syscall"
)

const initProcessGroup = 1

type signalSender func(pid int, sig syscall.Signal) error

func groupKiller(pgid int) error {
	return killGroup(pgid, syscall.Kill)
}

func killGroup(pgid int, send signalSender) error {
	if pgid <= initProcessGroup {
		return fmt.Errorf("refusing to kill process group %d", pgid)
	}
	return send(-pgid, syscall.SIGKILL)
}
