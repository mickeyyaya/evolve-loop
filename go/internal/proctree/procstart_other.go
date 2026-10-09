//go:build !darwin && !linux

package proctree

import "errors"

func readProcStart(int) (string, error) {
	return "", errors.New("procstart: this platform has no reader for the process start time")
}
