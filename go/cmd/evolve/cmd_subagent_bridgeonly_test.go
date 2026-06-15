package main

import (
	"os"
	"strings"
	"testing"
)

// TestSubagentRun_NoInProcessFallbackSignal is the B1 invariant proof: the
// subagent-run command must NOT emit any signal telling the orchestrator to
// fall back to the in-process Agent tool. The agent-bridge is the only
// dispatch path; the historical LEGACY_DISPATCH escape hatch is retired.
//
// This is a source-level guard because the "fallback" was a printed contract
// signal consumed by the (LLM) orchestrator, not a reachable Go branch.
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

	// Every surviving mention of LEGACY_AGENT_DISPATCH must either mark it as
	// retired (help/usage text) or be the env read that feeds the hard error
	// (os.Getenv, or the envchain.* getter it was migrated onto in Phase 1.3).
	// It must NEVER be advertised as an honored/supported env var — that
	// duplicated-and-stale advertisement is exactly what the auditor caught.
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
