package guardcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAppendGuardsLog_AllowLine verifies the audit-trail line format
// matches the bash counterpart in scripts/guards/*.sh — `[ts] [tag] ALLOW`
// or `[ts] [tag] DENY: reason`. Parity is load-bearing: external tooling
// greps guards.log and must see the same lines whether the bash hook or
// the Go shim wrote them.
func TestAppendGuardsLog_AllowLine(t *testing.T) {
	dir := t.TempDir()
	appendGuardsLog(filepath.Join(dir, "guards.log"), "ship", true, "")

	got := readLog(t, filepath.Join(dir, "guards.log"))
	// Format: `[YYYY-MM-DDTHH:MM:SSZ] [ship-gate] ALLOW\n`
	if !strings.Contains(got, "] [ship-gate] ALLOW") {
		t.Errorf("missing tag/decision in line: %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("line not newline-terminated: %q", got)
	}
}

func TestAppendGuardsLog_DenyLine(t *testing.T) {
	dir := t.TempDir()
	appendGuardsLog(filepath.Join(dir, "guards.log"), "ship", false, "ship-class command must invoke ship.sh")

	got := readLog(t, filepath.Join(dir, "guards.log"))
	if !strings.Contains(got, "] [ship-gate] DENY: ship-class command must invoke ship.sh") {
		t.Errorf("DENY line missing reason suffix: %q", got)
	}
}

// Each Go guard subcommand name must map to the bash audit-trail tag so
// parity-audit.sh's diff against scripts/guards/*.sh stays clean.
func TestAppendGuardsLog_TagMapping(t *testing.T) {
	cases := []struct {
		guardName string
		wantTag   string
	}{
		{"ship", "ship-gate"},
		{"phase", "phase-gate-pre"},
		{"role", "role-gate"},
		{"quota", "research-quota-gate"},
		{"docdelete", "doc-deletion-guard"},
		{"chain", "chain"},
	}
	for _, tc := range cases {
		dir := t.TempDir()
		appendGuardsLog(filepath.Join(dir, "guards.log"), tc.guardName, true, "")
		got := readLog(t, filepath.Join(dir, "guards.log"))
		wantFrag := "[" + tc.wantTag + "] ALLOW"
		if !strings.Contains(got, wantFrag) {
			t.Errorf("guard=%q: line %q missing %q", tc.guardName, got, wantFrag)
		}
	}
}

// Unknown guard names fall through to using the raw name as the tag.
// Matches bash behavior where no-op tags pass through.
func TestAppendGuardsLog_UnknownGuardFallsThrough(t *testing.T) {
	dir := t.TempDir()
	appendGuardsLog(filepath.Join(dir, "guards.log"), "novel", true, "")
	got := readLog(t, filepath.Join(dir, "guards.log"))
	if !strings.Contains(got, "[novel] ALLOW") {
		t.Errorf("unknown guard tag not preserved: %q", got)
	}
}

// TestAppendGuardsLog_CustomPath verifies that appendGuardsLog writes to the
// logPath passed directly, not to the default guards.log location.
func TestAppendGuardsLog_CustomPath(t *testing.T) {
	dir := t.TempDir()
	custom := filepath.Join(dir, "subdir", "custom.log")
	appendGuardsLog(custom, "ship", true, "")

	if _, err := os.Stat(custom); err != nil {
		t.Fatalf("custom log path not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "guards.log")); !os.IsNotExist(err) {
		t.Errorf("default path written despite custom logPath")
	}
}

// Best-effort guarantee: appendGuardsLog must never panic or surface an
// error to the caller, since hook latency is in the critical path and the
// audit-log is a secondary concern.
func TestAppendGuardsLog_UnwritablePathSilent(t *testing.T) {
	// Point at a path under a non-existent unwritable parent; MkdirAll
	// will fail, function must return without panic.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic on unwritable path: %v", r)
		}
	}()
	appendGuardsLog("/proc/nonexistent/cannot/write/here.log", "ship", true, "")
}

func readLog(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	return string(b)
}

// TestRunGuard_EmitsAuditLine wires runGuard end-to-end and asserts the
// audit-log side-effect happens. Uses the `chain` guard with empty input
// (no ledger entries → Allow) to avoid needing a full ship-gate fixture.
func TestRunGuard_EmitsAuditLine(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	rc := RunGuard([]string{"--evolve-dir", dir, "chain"}, strings.NewReader(""), &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%q", rc, stderr.String())
	}
	got := readLog(t, filepath.Join(dir, "guards.log"))
	if !strings.Contains(got, "[chain]") {
		t.Errorf("audit line missing tag: %q", got)
	}
}

func TestRunGuard_BypassFlagAfterName(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	input := `{"tool_name":"Bash","tool_input":{"command":"git commit -m x"}}`
	rc := RunGuard([]string{"ship", "--evolve-dir", dir, "--bypass"}, strings.NewReader(input), &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%q", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"allow":true`) {
		t.Fatalf("bypass result missing allow=true: %q", stdout.String())
	}
}
