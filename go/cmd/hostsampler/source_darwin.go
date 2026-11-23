//go:build darwin

package main

import (
	"os"
	"time"
)

func newSource() (reader, error) {
	feed := startCPUFeed(execStream, time.Now, os.Stderr)
	return func() (Counters, error) { return readMacOS(execOutput, time.Now, feed.latest), nil }, nil
}

func readHost() HostInfo {
	return readMacOSHost(execOutput)
}
