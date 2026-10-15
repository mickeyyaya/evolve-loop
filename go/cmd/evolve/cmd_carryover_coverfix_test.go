package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestCarryoverCoverfixRenderMarshalFailureReportsAndExits1(t *testing.T) {
	t.Parallel()
	failing := func(any, string, string) ([]byte, error) { return nil, errors.New("marshal boom") }
	var out, errB bytes.Buffer
	rc := renderCarryoverItemsWith(failing, []carryoverListItem{{ID: "a"}}, true, &out, &errB)
	if rc != 1 {
		t.Fatalf("rc=%d, want 1", rc)
	}
	if !strings.Contains(errB.String(), "evolve carryover: marshal boom") {
		t.Errorf("stderr=%q, want the marshal error reported", errB.String())
	}
	if out.Len() != 0 {
		t.Errorf("stdout=%q, want empty on failure", out.String())
	}
}

func TestCarryoverCoverfixRenderJSONAndTable(t *testing.T) {
	t.Parallel()
	items := []carryoverListItem{{ID: "todo-1", Priority: "HIGH", Age: 3, CyclesUnpicked: 2, ExpiresAt: "2026-12-01"}}

	var jout, jerr bytes.Buffer
	if rc := renderCarryoverItems(items, true, &jout, &jerr); rc != 0 {
		t.Fatalf("json rc=%d stderr=%s", rc, jerr.String())
	}
	if !strings.Contains(jout.String(), `"id": "todo-1"`) || !strings.Contains(jout.String(), `"cycles_unpicked": 2`) {
		t.Errorf("json output missing fields: %s", jout.String())
	}

	var tout, terr bytes.Buffer
	if rc := renderCarryoverItems(items, false, &tout, &terr); rc != 0 {
		t.Fatalf("table rc=%d stderr=%s", rc, terr.String())
	}
	lines := strings.Split(strings.TrimSpace(tout.String()), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "ID") {
		t.Fatalf("table = %q, want header + 1 row", tout.String())
	}
	for _, want := range []string{"todo-1", "HIGH", "3", "2", "2026-12-01"} {
		if !strings.Contains(lines[1], want) {
			t.Errorf("row %q missing %q", lines[1], want)
		}
	}
}
