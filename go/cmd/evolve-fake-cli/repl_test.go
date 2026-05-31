package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// composedPrompt builds a prompt with the same shape the bridge composes:
// agent heading first, body, then a Cycle Context block with the workspace
// line that the REPL triggers on.
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

// The REPL must print the boot marker, read the pasted prompt, and write the
// phase artifact resolved from workspace+basename — the core of tmux full-cycle.
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

// Triggering on the workspace line (not a stray absolute path in the body)
// must write to workspace/<basename> for the heading's phase — even when the
// body mentions an upstream artifact's absolute path.
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
	// The upstream scout path mentioned in the body must NOT have been written.
	if _, err := os.Stat("/some/other/cycle/scout-report.md"); err == nil {
		t.Error("REPL wrote to the upstream path mentioned in the body — should only write workspace/build-report.md")
	}
}

// Audit verdict injection: FAIL must shape both the report verdict line and
// the fused acs-verdict.json red_count (so the EGPS gate blocks the cycle).
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

// parseArgs must infer the CLI family (for exit injection) and the
// interactive/REPL flag from the invocation flags alone.
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

// Per-CLI exit injection: the matching FAKE_CLI_<STYLE>_EXIT makes the
// headless run fail with that code and write NO artifact.
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

// Exit injection is scoped to the matching style: a codex-targeted injection
// must not affect a claude invocation.
func TestRun_ExitInjectionScopedToStyle(t *testing.T) {
	t.Setenv("FAKE_CLI_CODEX_EXIT", "81")
	dir := t.TempDir()
	artifact := filepath.Join(dir, "scout-report.md")
	args := []string{"-p", "write to " + artifact + " please", "--model", "sonnet"} // claude style

	var stdout, stderr bytes.Buffer
	if rc := run(args, bytes.NewReader(nil), &stdout, &stderr); rc != 0 {
		t.Fatalf("rc=%d, want 0 (codex injection must not affect claude); stderr=%s", rc, stderr.String())
	}
	if _, err := os.Stat(artifact); err != nil {
		t.Errorf("artifact should still be written for the non-targeted style: %v", err)
	}
}

// auditVerdict normalises the env var; garbage and unset both mean PASS.
func TestAuditVerdict_Normalisation(t *testing.T) {
	cases := map[string]string{"": "PASS", "pass": "PASS", "warn": "WARN", "FAIL": "FAIL", "garbage": "PASS"}
	for in, want := range cases {
		t.Setenv("FAKE_CLI_AUDIT_VERDICT", in)
		if got := auditVerdict(); got != want {
			t.Errorf("auditVerdict(%q)=%q, want %q", in, got, want)
		}
	}
}
