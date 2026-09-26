package phasecmd

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// The -h path keeps these handlers from spawning observers or touching real workspaces.
func TestApicover_HandlerSmoke(t *testing.T) {
	cases := []struct {
		name string
		run  func([]string, io.Reader, io.Writer, io.Writer) int
	}{
		{"phase-observer", RunPhaseObserver},
		{"phase-order", RunPhaseOrder},
		{"phase-watchdog", RunPhaseWatchdog},
	}
	for _, c := range cases {
		var out, errb bytes.Buffer
		_ = c.run([]string{"-h"}, strings.NewReader(""), &out, &errb)
		if out.Len()+errb.Len() == 0 {
			t.Errorf("%s: expected help output on the -h path, got none", c.name)
		}
	}
}
