package main

import (
	"os"
	"path/filepath"
	"testing"
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

func TestResolveMaxCyclesCap(t *testing.T) {
	t.Setenv("EVOLVE_MAX_CYCLES_CAP", "")
	if got := resolveMaxCyclesCap(); got != 25 {
		t.Errorf("unset cap = %d, want default 25", got)
	}
	t.Setenv("EVOLVE_MAX_CYCLES_CAP", "8")
	if got := resolveMaxCyclesCap(); got != 8 {
		t.Errorf("cap = %d, want 8", got)
	}
	for _, bad := range []string{"0", "-3", "abc"} {
		t.Setenv("EVOLVE_MAX_CYCLES_CAP", bad)
		if got := resolveMaxCyclesCap(); got != 25 {
			t.Errorf("cap %q = %d, want default 25", bad, got)
		}
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
