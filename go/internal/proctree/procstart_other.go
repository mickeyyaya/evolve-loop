//go:build !darwin && !linux

package proctree

import (
	"errors"
	"time"
)

var errProcStartUnsupported = errors.New("procstart: this platform has no reader for the process start time")

func readProcStart(int) (string, error) {
	return "", errProcStartUnsupported
}

func readProcStartTime(int) (time.Time, error) {
	return time.Time{}, errProcStartUnsupported
}
