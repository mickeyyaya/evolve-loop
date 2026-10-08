package proctree

import (
	"strconv"
	"strings"
	"time"
)

type Process struct {
	Pid     int
	Ppid    int
	Pgid    int
	Started time.Time
	Comm    string
	Args    []string
	Env     map[string]string
}

const (
	psNumericColumns = 4
	psStartColumns   = 5
	psStartLayout    = "Mon Jan _2 15:04:05 2006"
)

func parsePS(out string, uid int) []Process {
	var procs []Process
	for _, line := range strings.Split(out, "\n") {
		if p, ok := parsePSRow(line, uid); ok {
			procs = append(procs, p)
		}
	}
	return procs
}

func parsePSRow(line string, uid int) (Process, bool) {
	fields := strings.Fields(line)
	if len(fields) <= psNumericColumns+psStartColumns {
		return Process{}, false
	}
	nums := make([]int, psNumericColumns)
	for i := range nums {
		n, err := strconv.Atoi(fields[i])
		if err != nil {
			return Process{}, false
		}
		nums[i] = n
	}
	if nums[3] != uid {
		return Process{}, false
	}
	started, err := time.ParseInLocation(psStartLayout, strings.Join(fields[psNumericColumns:psNumericColumns+psStartColumns], " "), time.Local)
	if err != nil {
		return Process{}, false
	}
	comm := strings.Join(fields[psNumericColumns+psStartColumns:], " ")
	return Process{Pid: nums[0], Ppid: nums[1], Pgid: nums[2], Started: started, Comm: comm}, true
}
