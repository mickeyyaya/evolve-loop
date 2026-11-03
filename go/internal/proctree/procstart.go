package proctree

import "time"

func StartOf(pid int) (string, error) {
	return readProcStart(pid)
}

func StartTime(pid int) (time.Time, error) {
	return readProcStartTime(pid)
}
