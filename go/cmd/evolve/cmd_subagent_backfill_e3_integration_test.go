//go:build integration

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const e3HeadlessScout = `{"name":"scout","cli":"codex","model_tier_default":"deep","output_artifact":"e3-out.md"}`

func e3RunScout(t *testing.T) (root, ws, record string) {
	t.Helper()
	root = e3SubagentProject(t, e3HeadlessScout, nil)
	ws = filepath.Join(root, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	record = bfStub(t, "codex", "cat > /dev/null; exit 0")
	return root, ws, record
}

func TestRunSubagentRun_AnAgentThatWritesNoArtifactIsAnIntegrityFailWithItsWarnings(t *testing.T) {
	root, ws, record := e3RunScout(t)
	prompt := filepath.Join(root, "prompt.txt")
	writeFileForPublishTest(t, prompt, "scan the repo\n")
	t.Setenv("PROMPT_FILE_OVERRIDE", prompt)
	var out, errb bytes.Buffer

	rc := runSubagentRun([]string{"scout", "7", ws}, &out, &errb)

	if rc != 2 {
		t.Fatalf("rc=%d, want 2 for INTEGRITY_FAIL; stderr=%s", rc, errb.String())
	}
	got := errb.String()
	if !strings.Contains(got, "[subagent-run] WARN agent=scout cycle=7: WorktreePath not propagated") {
		t.Errorf("the run's warnings were not printed: %s", got)
	}
	if !strings.Contains(got, "[subagent-run] verdict=INTEGRITY_FAIL agent=scout cycle=7 artifact="+filepath.Join(root, "e3-out.md")+" exit=0 ") {
		t.Errorf("verdict line missing: %s", got)
	}
	if bfCalls(t, record) == "" {
		t.Error("the codex binary never ran")
	}
	e3WantPrompt(t, ws, "scan the repo")
}

func TestRunSubagentRun_ReadsThePromptFromStdinWithoutAnOverride(t *testing.T) {
	_, ws, record := e3RunScout(t)
	t.Setenv("PROMPT_FILE_OVERRIDE", "")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString("prompt via stdin\n"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	prev := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = prev; _ = r.Close() })
	var out, errb bytes.Buffer

	rc := runSubagentRun([]string{"scout", "8", ws}, &out, &errb)

	if rc != 2 || !strings.Contains(errb.String(), "[subagent-run] verdict=INTEGRITY_FAIL agent=scout cycle=8 ") {
		t.Fatalf("rc=%d stderr=%s", rc, errb.String())
	}
	if bfCalls(t, record) == "" {
		t.Error("the codex binary never ran")
	}
	e3WantPrompt(t, ws, "prompt via stdin")
}

func e3WantPrompt(t *testing.T, ws, want string) {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(ws, "agent-prompt.txt"))
	if err != nil || !strings.Contains(string(got), want) {
		t.Errorf("the dispatched prompt lacks %q (err %v):\n%s", want, err, got)
	}
}
