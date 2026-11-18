package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunInboxMover_DelegatesToTheInboxcmdMoverWithTheProductionDeps(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if rc := runInboxMover([]string{"--help"}, strings.NewReader(""), &stdout, &stderr); rc != 0 || !strings.Contains(stdout.String(), "inbox-mover") {
		t.Errorf("rc %d stdout %q stderr %q", rc, stdout.String(), stderr.String())
	}
}
