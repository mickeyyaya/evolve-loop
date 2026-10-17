//go:build !linux && !darwin

package main

func newSource() (reader, error) {
	return unsupportedSource()
}
