package main

import (
	"bytes"
	"io"
	"testing"
)

func TestFlagsCoverfixNoArgsPrintsUsageAndExits10(t *testing.T) {
	t.Parallel()
	var errB bytes.Buffer
	if rc := runFlags(nil, nil, io.Discard, &errB); rc != 10 {
		t.Fatalf("rc=%d, want 10", rc)
	}
	if got := errB.String(); got != "usage: evolve flags <generate|check>\n" {
		t.Errorf("stderr=%q, want the usage line", got)
	}
}
