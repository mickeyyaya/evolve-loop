package main

import (
	"os"
	"strings"
	"testing"
)

func TestSubagentRun_NoInProcessFallbackSignal(t *testing.T) {
	body, err := os.ReadFile("cmd_subagent.go")
	if err != nil {
		t.Fatalf("read cmd_subagent.go: %v", err)
	}
	src := string(body)

	banned := []string{
		"fall back to in-process Agent tool", // the instruction to the orchestrator
		`"LEGACY_DISPATCH"`,                  // the stdout escape-hatch token
		"res.LegacyDispatch",                 // the result field that carried it
	}
	for _, b := range banned {
		if strings.Contains(src, b) {
			t.Errorf("cmd_subagent.go still emits in-process fallback signal %q — bridge-only invariant violated", b)
		}
	}

	for i, line := range strings.Split(src, "\n") {
		if !strings.Contains(line, "LEGACY_AGENT_DISPATCH") {
			continue
		}
		ok := strings.Contains(line, "retired") ||
			strings.Contains(line, "os.Getenv") ||
			strings.Contains(line, "envchain.")
		if !ok {
			t.Errorf("cmd_subagent.go:%d advertises LEGACY_AGENT_DISPATCH as honored (must be retired-note or env-read only): %q", i+1, strings.TrimSpace(line))
		}
	}
}
