//go:build !darwin && !linux

package proctree

import (
	"errors"
	"runtime"
	"testing"
)

func TestReadTable_AnUnsupportedPlatformFailsLoudly(t *testing.T) {
	t.Parallel()
	got, err := readTable(1)

	if !namesThisPlatform(err) || got != nil {
		t.Errorf("readTable = %v, %v, want no rows and an error that names %s", got, err, runtime.GOOS)
	}
}

func TestReadComm_AnUnsupportedPlatformFailsLoudly(t *testing.T) {
	t.Parallel()
	got, err := readComm(1)

	if !namesThisPlatform(err) || got != "" {
		t.Errorf("readComm = %q, %v, want an error that names %s", got, err, runtime.GOOS)
	}
}

func TestCommFrom_IsTheTableComm(t *testing.T) {
	t.Parallel()
	if got := commFrom(Process{Comm: "x", Args: []string{"y"}}); got != "x" {
		t.Errorf("commFrom = %q, want x", got)
	}
}

func namesThisPlatform(err error) bool {
	var named platformError
	return errors.As(err, &named) && named.goos == runtime.GOOS
}
