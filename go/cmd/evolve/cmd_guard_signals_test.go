package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestGuardVerb_AShipDenialReachesTheProjectStreamThroughTheRegisteredVerb(t *testing.T) {
	t.Parallel()
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	cwd := t.TempDir()
	input := `{"tool_name":"Bash","cwd":"` + cwd + `","tool_input":{"command":"git push origin main"}}`
	var stdout, stderr bytes.Buffer

	rc := lookupCommand("guard").Run([]string{"ship", "--evolve-dir", evolveDir}, strings.NewReader(input), &stdout, &stderr)

	chunk, err := signalcenter.ReadStream(filepath.Join(evolveDir, signalcenter.StreamFileName), 0)
	if err != nil || rc != 2 || len(chunk.Events) != 1 {
		t.Fatalf("rc=%d err=%v events=%d stderr=%q: the verb that the hook runs must emit exactly one denial", rc, err, len(chunk.Events), stderr.String())
	}
	e := chunk.Events[0]
	if e.Code != core.CodeGuardDenied || e.Fields[core.DenialFieldNeutral] != "true" || e.Fields[core.DenialFieldCWD] != cwd || e.Fields[core.DenialFieldPath] != "git push origin main" {
		t.Fatalf("event %+v", e)
	}
	if strings.Contains(stderr.String(), "[events]") || strings.Contains(stderr.String(), "ISOLATION_GUARD_DENIED") {
		t.Fatalf("the hook stderr carries only the deny line: %q", stderr.String())
	}
}

func TestGuardVerb_AnAllowedCallWritesNoStream(t *testing.T) {
	t.Parallel()
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	var stdout, stderr bytes.Buffer

	rc := lookupCommand("guard").Run([]string{"ship", "--evolve-dir", evolveDir}, strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`), &stdout, &stderr)

	if _, err := signalcenter.ReadStream(filepath.Join(evolveDir, signalcenter.StreamFileName), 0); rc != 0 || err == nil {
		t.Fatalf("rc=%d err=%v: an allow creates no stream", rc, err)
	}
}
