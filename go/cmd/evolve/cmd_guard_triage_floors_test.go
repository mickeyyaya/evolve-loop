package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cmd_guard_triage_floors_test.go — ADR-0046 Layer 1 (cycle 305): the
// agent-runnable `evolve guard triage-floors <workspace>` self-check. It reads
// the workspace's triage-report.md + triage-decision.json companion, reports
// committed/deferred declaration-vs-prose divergence, and exits non-zero when
// they disagree so the triage persona can repair its artifact before the
// correction ladder fires (ADR-0045 gate-admission rule).
//
// RED: `triage-floors` is not a recognized subcommand, so runGuard falls through
// to buildGuard, which errors → exit 10 for every case below. Each pin asserts
// the BUILT behavior (exit 0 clean / non-zero divergence / informative --help),
// so all are RED until Builder adds the subcommand branch before stdin parsing.

// triageFloorsFixture lays out a self-contained project root so the guard can
// source its package vocabulary from go/internal regardless of cwd, with the
// workspace at <root>/.evolve/runs/cycle-305. Returns the workspace path.
func triageFloorsFixture(t *testing.T, report, companion string) string {
	t.Helper()
	root := t.TempDir()
	for _, pkg := range []string{"core", "bridge"} {
		dir := filepath.Join(root, "go", "internal", pkg)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, pkg+".go"), []byte("package "+pkg+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ws := filepath.Join(root, ".evolve", "runs", "cycle-305")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "triage-report.md"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(companion), 0o644); err != nil {
		t.Fatal(err)
	}
	return ws
}

// cleanReport commits core (top_n) and defers bridge (## deferred); the clean
// companion declares exactly that, so both committed and deferred declarations
// agree with prose → no divergence.
const cleanReport = `## top_n (commit to THIS cycle)
- coverage-core: push internal/core coverage to ≥98% — priority=H

## deferred (carry to NEXT cycle's carryoverTodos)
- coverage-bridge: push bridge coverage to ≥98% — defer_reason=too large
`

func TestGuardTriageFloors_CleanWorkspaceExitsZero(t *testing.T) {
	ws := triageFloorsFixture(t, cleanReport,
		`{"cycle":305,"committed_floors":["core"],"deferred_floors":["bridge"]}`)
	var stdout, stderr bytes.Buffer
	rc := runGuard([]string{"triage-floors", ws}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("clean workspace (declarations match prose) must exit 0; rc=%d stdout=%s stderr=%s",
			rc, stdout.String(), stderr.String())
	}
}

func TestGuardTriageFloors_DivergenceExitsNonZero(t *testing.T) {
	// Companion declares deferred core, but prose defers bridge → divergence.
	ws := triageFloorsFixture(t, cleanReport,
		`{"cycle":305,"committed_floors":["core"],"deferred_floors":["core"]}`)
	var stdout, stderr bytes.Buffer
	rc := runGuard([]string{"triage-floors", ws}, nil, &stdout, &stderr)
	if rc == 0 {
		t.Fatalf("declaration/prose divergence must exit non-zero; rc=0 stdout=%s", stdout.String())
	}
}

func TestGuardTriageFloors_HelpIsInformative(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runGuard([]string{"triage-floors", "--help"}, nil, &stdout, &stderr)
	out := stdout.String() + stderr.String()
	if rc != 0 {
		t.Fatalf("triage-floors --help must exit 0; rc=%d out=%s", rc, out)
	}
	if !strings.Contains(strings.ToLower(out), "floor") {
		t.Errorf("triage-floors --help must describe the floor self-check; got %q", out)
	}
}
