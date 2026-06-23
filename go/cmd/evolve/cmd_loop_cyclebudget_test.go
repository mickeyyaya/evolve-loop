package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/policy"
)

func TestReadCarryoverCount(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	// Two carryover todos → count 2, ok.
	p := write("two.json", `{"carryoverTodos":[{"id":"a"},{"id":"b"}],"x":1}`)
	if n, ok := readCarryoverCount(p); !ok || n != 2 {
		t.Errorf("two todos: got (%d,%v), want (2,true)", n, ok)
	}
	// Drained backlog → 0, ok (the completion signal).
	p = write("empty.json", `{"carryoverTodos":[]}`)
	if n, ok := readCarryoverCount(p); !ok || n != 0 {
		t.Errorf("empty: got (%d,%v), want (0,true)", n, ok)
	}
	// Missing key → 0, ok.
	p = write("nokey.json", `{"other":1}`)
	if n, ok := readCarryoverCount(p); !ok || n != 0 {
		t.Errorf("no key: got (%d,%v), want (0,true)", n, ok)
	}
	// Absent file → ok=false (caller must NOT treat as complete).
	if n, ok := readCarryoverCount(filepath.Join(dir, "absent.json")); ok || n != 0 {
		t.Errorf("absent: got (%d,%v), want (0,false)", n, ok)
	}
	// Malformed JSON → ok=false.
	p = write("bad.json", `{not json`)
	if _, ok := readCarryoverCount(p); ok {
		t.Errorf("malformed: ok=true, want false")
	}
}

func TestWorkflowMaxCyclesCap(t *testing.T) {
	tests := []struct {
		name     string
		workflow *policy.WorkflowPolicy
		want     int
	}{
		{"absent defaults to 25", nil, 25},
		{"configured value", &policy.WorkflowPolicy{MaxCyclesCap: 8}, 8},
		{"zero defaults to 25", &policy.WorkflowPolicy{}, 25},
		{"negative defaults to 25", &policy.WorkflowPolicy{MaxCyclesCap: -3}, 25},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := (policy.Policy{Workflow: tc.workflow}).WorkflowConfig().MaxCyclesCap; got != tc.want {
				t.Errorf("WorkflowConfig().MaxCyclesCap = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestParseLoopArgs_MaxCyclesExplicit(t *testing.T) {
	// Explicit --cycles ⇒ explicit true, honored as the value.
	cfg, rc := parseLoopArgs([]string{"--goal-text", "g", "--cycles", "3"}, os.Stderr)
	if rc != 0 || !cfg.MaxCyclesExplicit || cfg.MaxCycles != 3 {
		t.Fatalf("explicit: rc=%d explicit=%v max=%d, want 0/true/3", rc, cfg.MaxCyclesExplicit, cfg.MaxCycles)
	}
	// No cycles flag ⇒ not explicit (so enforce-mode may default to the cap).
	cfg, rc = parseLoopArgs([]string{"--goal-text", "g"}, os.Stderr)
	if rc != 0 || cfg.MaxCyclesExplicit {
		t.Fatalf("implicit: rc=%d explicit=%v, want 0/false", rc, cfg.MaxCyclesExplicit)
	}
}
