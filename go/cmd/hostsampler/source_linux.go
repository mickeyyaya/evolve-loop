//go:build linux

package main

import "time"

const procRoot = "/proc"

func newSource() (reader, error) {
	return func() (Counters, error) { return readLinux(procRoot, time.Now) }, nil
}

func readHost() HostInfo {
	return readLinuxHost(procRoot, execOutput)
}
