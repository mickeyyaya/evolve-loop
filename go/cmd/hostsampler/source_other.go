//go:build !linux && !darwin

package main

import "runtime"

func newSource() (reader, error) {
	return unsupportedSource()
}

func readHost() HostInfo {
	return HostInfo{OS: runtime.GOOS}
}
