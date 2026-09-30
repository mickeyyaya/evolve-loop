package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func composedPrompt(heading, workspace string) string {
	return strings.Join([]string{
		heading,
		"You will do synthetic work.",
		"",
		"## Cycle Context",
		"- cycle: 1",
		"- workspace: " + workspace,
		"- goal_hash: g",
	}, "\n")
}

func TestRunREPL_WritesArtifactFromWorkspaceLine(t *testing.T) {
	ws := t.TempDir()
	stdin := strings.NewReader(composedPrompt("# Evolve Scout", ws))
	var stdout, stderr bytes.Buffer

	rc := runREPL(stdin, &stdout, &stderr, "PASS")
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), replBootMarkers) {
		t.Errorf("stdout must contain boot marker %q; got %q", replBootMarkers, stdout.String())
	}
	body, err := os.ReadFile(filepath.Join(ws, "scout-report.md"))
	if err != nil {
		t.Fatalf("scout-report.md not written: %v", err)
	}
	if !strings.Contains(string(body), "## Proposed Tasks") {
		t.Errorf("scout artifact missing marker; got %q", string(body))
	}
}

func TestRunREPL_BuilderHeadingNotMisroutedByUpstreamMention(t *testing.T) {
	ws := t.TempDir()
	prompt := strings.Join([]string{
		"# Evolve Builder",
		"Read /some/other/cycle/scout-report.md, then write build-report.md.",
		"",
		"## Cycle Context",
		"- workspace: " + ws,
	}, "\n")
	var stdout, stderr bytes.Buffer

	if rc := runREPL(strings.NewReader(prompt), &stdout, &stderr, "PASS"); rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(ws, "build-report.md")); err != nil {
		t.Fatalf("build-report.md not written to workspace: %v", err)
	}
	if _, err := os.Stat("/some/other/cycle/scout-report.md"); err == nil {
		t.Error("REPL wrote to the upstream path mentioned in the body — should only write workspace/build-report.md")
	}
}

func TestRunREPL_AuditVerdictInjection(t *testing.T) {
	cases := []struct {
		verdict      string
		wantRedCount int
	}{
		{"PASS", 0},
		{"WARN", 0},
		{"FAIL", 1},
	}
	for _, tc := range cases {
		t.Run(tc.verdict, func(t *testing.T) {
			ws := t.TempDir()
			stdin := strings.NewReader(composedPrompt("# Evolve Auditor", ws))
			var stdout, stderr bytes.Buffer
			if rc := runREPL(stdin, &stdout, &stderr, tc.verdict); rc != 0 {
				t.Fatalf("rc=%d stderr=%s", rc, stderr.String())
			}
			report, err := os.ReadFile(filepath.Join(ws, "audit-report.md"))
			if err != nil {
				t.Fatalf("audit-report.md not written: %v", err)
			}
			if !strings.Contains(string(report), "**"+tc.verdict+"**") {
				t.Errorf("audit-report verdict line missing **%s**; got %q", tc.verdict, string(report))
			}
			acs, err := os.ReadFile(filepath.Join(ws, "acs-verdict.json"))
			if err != nil {
				t.Fatalf("acs-verdict.json not written: %v", err)
			}
			var probe struct {
				RedCount int `json:"red_count"`
			}
			if err := json.Unmarshal(acs, &probe); err != nil {
				t.Fatalf("acs-verdict.json invalid: %v", err)
			}
			if probe.RedCount != tc.wantRedCount {
				t.Errorf("red_count=%d, want %d for verdict=%s", probe.RedCount, tc.wantRedCount, tc.verdict)
			}
		})
	}
}

func TestRunREPL_EOFFallback_WritesFromAbsolutePath(t *testing.T) {
	ws := t.TempDir()
	artifact := filepath.Join(ws, "scout-report.md")
	promptWithoutWorkspaceLine := "# Evolve Scout\n\nplease write " + artifact + "\n"
	var stdout, stderr bytes.Buffer

	rc := runREPL(strings.NewReader(promptWithoutWorkspaceLine), &stdout, &stderr, "PASS")

	if rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, stderr.String())
	}
	body, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("EOF-fallback should have written %s: %v", artifact, err)
	}
	if !strings.Contains(string(body), "## Proposed Tasks") {
		t.Errorf("scout artifact missing marker; got %q", string(body))
	}
}

func TestEmitREPLArtifacts_UnknownPhase_ReturnsFalse(t *testing.T) {
	var stdout, stderr bytes.Buffer
	ok := emitREPLArtifacts("nonesuch", "/tmp/x.md", "PASS", &stdout, &stderr)

	if ok {
		t.Error("emitREPLArtifacts should return false for an unknown phase")
	}
	if !strings.Contains(stderr.String(), "artifactsFor") {
		t.Errorf("stderr should report the artifactsFor failure; got %q", stderr.String())
	}
}

func TestEmitREPLArtifacts_WriteFailure_ReturnsFalse(t *testing.T) {
	parent := t.TempDir()
	ro := filepath.Join(parent, "ro")
	if err := os.Mkdir(ro, 0o555); err != nil {
		t.Fatalf("mkdir ro: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0o755) })
	artifact := filepath.Join(ro, "sub", "scout-report.md")

	var stdout, stderr bytes.Buffer
	ok := emitREPLArtifacts("scout", artifact, "PASS", &stdout, &stderr)

	if ok {
		t.Error("emitREPLArtifacts should return false when the write fails")
	}
	if stderr.Len() == 0 {
		t.Error("write failure should be logged to stderr")
	}
}

func TestWriteArtifacts_UnwritableDestination_Errors(t *testing.T) {
	parent := t.TempDir()
	ro := filepath.Join(parent, "ro")
	if err := os.Mkdir(ro, 0o555); err != nil {
		t.Fatalf("mkdir ro: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0o755) })

	fileDirectlyInReadOnlyDir := filepath.Join(ro, "out.md")
	err := writeArtifacts(map[string]string{fileDirectlyInReadOnlyDir: "x"})

	if err == nil {
		t.Fatal("writeArtifacts should error writing into a read-only directory")
	}
	if !strings.Contains(err.Error(), "write") {
		t.Errorf("error should mention the write op; got %v", err)
	}
}

func TestParseArgs_StyleAndInteractive(t *testing.T) {
	cases := []struct {
		name            string
		args            []string
		wantStyle       string
		wantInteractive bool
	}{
		{"claude headless", []string{"-p", "# Evolve Scout\n- workspace: /tmp/ws", "--model", "sonnet"}, "claude", false},
		{"agy headless", []string{"-p", "# Evolve Scout\n- workspace: /tmp/ws", "--dangerously-skip-permissions"}, "agy", false},
		{"codex headless", []string{"exec", "--output-last-message", "/tmp/ws/scout-report.md"}, "codex", false},
		{"claude-tmux interactive", []string{"--model", "sonnet", "--setting-sources", "project"}, "claude", true},
		{"agy-tmux interactive", []string{"--dangerously-skip-permissions"}, "agy", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inv, err := parseArgs(tc.args, bytes.NewReader(nil))
			if err != nil {
				t.Fatalf("parseArgs: %v", err)
			}
			if inv.Style != tc.wantStyle {
				t.Errorf("Style=%q, want %q", inv.Style, tc.wantStyle)
			}
			if inv.Interactive != tc.wantInteractive {
				t.Errorf("Interactive=%v, want %v", inv.Interactive, tc.wantInteractive)
			}
		})
	}
}

func TestRun_PerCLIExitInjection(t *testing.T) {
	t.Setenv("FAKE_CLI_CLAUDE_EXIT", "81")
	dir := t.TempDir()
	artifact := filepath.Join(dir, "scout-report.md")
	args := []string{"-p", "write to " + artifact + " please", "--model", "sonnet"}

	var stdout, stderr bytes.Buffer
	rc := run(args, bytes.NewReader(nil), &stdout, &stderr)
	if rc != 81 {
		t.Fatalf("rc=%d, want 81 (injected); stderr=%s", rc, stderr.String())
	}
	if _, err := os.Stat(artifact); err == nil {
		t.Error("injected-exit run must NOT write an artifact")
	}
}

func TestRun_ExitInjectionScopedToStyle(t *testing.T) {
	t.Setenv("FAKE_CLI_CODEX_EXIT", "81")
	dir := t.TempDir()
	artifact := filepath.Join(dir, "scout-report.md")
	claudeStyleArgs := []string{"-p", "write to " + artifact + " please", "--model", "sonnet"}

	var stdout, stderr bytes.Buffer
	if rc := run(claudeStyleArgs, bytes.NewReader(nil), &stdout, &stderr); rc != 0 {
		t.Fatalf("rc=%d, want 0 (codex injection must not affect claude); stderr=%s", rc, stderr.String())
	}
	if _, err := os.Stat(artifact); err != nil {
		t.Errorf("artifact should still be written for the non-targeted style: %v", err)
	}
}

func TestAuditVerdict_Normalisation(t *testing.T) {
	cases := map[string]string{"": "PASS", "pass": "PASS", "warn": "WARN", "FAIL": "FAIL", "garbage": "PASS"}
	for in, want := range cases {
		t.Setenv("FAKE_CLI_AUDIT_VERDICT", in)
		if got := auditVerdict(); got != want {
			t.Errorf("auditVerdict(%q)=%q, want %q", in, got, want)
		}
	}
}
