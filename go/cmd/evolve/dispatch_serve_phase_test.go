package main

import (
	"bytes"
	"strings"
	"testing"
)

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
