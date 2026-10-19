//go:build darwin

package main

import "time"

func newSource() (reader, error) {
	return func() (Counters, error) { return readMacOS(execOutput, time.Now), nil }, nil
}
