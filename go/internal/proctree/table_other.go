//go:build !darwin && !linux

package proctree

import (
	"fmt"
	"runtime"
)

type platformError struct{ goos string }

func (e platformError) Error() string {
	return fmt.Sprintf("proctree: this platform (%s) has no reader for the process table", e.goos)
}

func errTableUnsupported() error {
	return platformError{goos: runtime.GOOS}
}

func readTable(int) ([]Process, error) {
	return nil, errTableUnsupported()
}
