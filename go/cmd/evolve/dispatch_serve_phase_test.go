package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestDispatch_RoutesServePhase confirms the dispatcher table routes the
// "serve-phase" command to phasecmd.RunServePhase — exercised via the no-arg
// path (exit 10 + "missing phase name" usage). The handler's envelope round-trip
// behavior is covered in internal/cli/phasecmd/serve_phase_test.go.
func TestDispatch_RoutesServePhase(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"serve-phase"}, nil, &stdout, &stderr)
	if code != 10 {
		t.Errorf("want exit 10, got %d", code)
	}
	if !strings.Contains(stderr.String(), "missing phase name") {
		t.Errorf("stderr=%q", stderr.String())
	}
}
