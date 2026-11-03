//go:build !darwin && !linux

package proctree

import (
	"errors"
	"os"
	"testing"
)

func TestStartTime_HasNoReaderOnThisPlatform(t *testing.T) {
	t.Parallel()
	got, err := StartTime(os.Getpid())

	if !errors.Is(err, errProcStartUnsupported) || !got.IsZero() {
		t.Errorf("StartTime = %v, %v, want %v and a zero time", got, err, errProcStartUnsupported)
	}
}

func TestStartOf_HasNoReaderOnThisPlatform(t *testing.T) {
	t.Parallel()
	got, err := StartOf(os.Getpid())

	if !errors.Is(err, errProcStartUnsupported) || got != "" {
		t.Errorf("StartOf = %q, %v, want %v", got, err, errProcStartUnsupported)
	}
}
