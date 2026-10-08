//go:build !darwin && !linux

package proctree

import "errors"

func readProcArgs(int) ([]string, map[string]string, error) {
	return nil, nil, errors.New("procargs: this platform has no reader for process arguments")
}
