package main

// cmd_subagent_run_test.go — ADR-0103 unit 16: the `evolve subagent run` root's
// stderr lines and exit map, pinned before the execution path moved into
// internal/subagent/subagentrun (nothing pinned them on 8e8f080f).

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/subagent"
)

// Test 17 — a PROMPT_FILE_OVERRIDE that cannot be opened is the root's own
// exit-1 FAIL line, verbatim.
func TestRunSubagentRun_PromptFileOverrideMissingExitsOne(t *testing.T) {
	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	missing := filepath.Join(root, "no-such-prompt.txt")
	t.Setenv("PROMPT_FILE_OVERRIDE", missing)
	var stdout, stderr bytes.Buffer
	if code := runSubagentRun([]string{"scout", "7", ws}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if got, want := stderr.String(), "[subagent-run] FAIL: PROMPT_FILE_OVERRIDE missing: "+missing+"\n"; got != want {
		t.Fatalf("stderr %q, want %q", got, want)
	}
}

// Test 59 — the --simulate render twin for the run root: an unknown agent
// renders the human FAIL line AND the structured WARN line on stderr, and the
// cycle-keyed signals.ndjson holds the code with the dispatcher's origin.
func TestRunSubagentRun_UnknownAgentRendersTheCodeAndFilesIt(t *testing.T) {
	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	prompt := filepath.Join(root, "prompt.txt")
	if err := os.WriteFile(prompt, []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PROMPT_FILE_OVERRIDE", prompt)
	var stdout, stderr bytes.Buffer
	if code := runSubagentRun([]string{"bogus", "7", ws}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit %d, want 1: %s", code, stderr.String())
	}
	out := stderr.String()
	if !strings.Contains(out, "[subagent-run] FAIL: subagent/run: unknown agent: bogus\n") {
		t.Fatalf("the human line is kept verbatim: %q", out)
	}
	if !strings.Contains(out, "[bridge] bridge.warning WARN BRIDGE_SUBAGENT_REQUEST_REJECTED cycle=7 phase=bogus") || !strings.Contains(out, "origin=Dispatcher.Dispatch") || !strings.Contains(out, "reason_class=unknown_agent") {
		t.Fatalf("the console sink renders the unit's WARN: %q", out)
	}
	data, err := os.ReadFile(filepath.Join(core.RunWorkspacePath(root, 7), "signals.ndjson"))
	if err != nil || !strings.Contains(string(data), `"code":"BRIDGE_SUBAGENT_REQUEST_REJECTED"`) || !strings.Contains(string(data), `"origin":"Dispatcher.Dispatch"`) || !strings.Contains(string(data), `"module":"bridge"`) {
		t.Fatalf("the cycle-keyed signal is durable: %v %s", err, data)
	}
}

// Test 60 — the verdict line and the exit map are pinned by a golden.
func TestRenderRunOutcome_LineAndExitMapArePinned(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "root-outcome.golden"))
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	res := subagent.RunResult{Verdict: subagent.VerdictIntegrityFail, ArtifactPath: "/r/.evolve/runs/cycle-7/audit.md", ExitCode: 0, DurationMS: 1999}
	if code := renderRunOutcome(res, "auditor", 7, &stderr); code != 2 || stderr.String() != string(want) {
		t.Fatalf("exit %d line %q, want 2 %q", code, stderr.String(), want)
	}
	for verdict, code := range map[string]int{subagent.VerdictPASS: 0, subagent.VerdictIntegrityFail: 2, subagent.VerdictFAIL: 1, "": 1} {
		if got := renderRunOutcome(subagent.RunResult{Verdict: verdict}, "scout", 1, &bytes.Buffer{}); got != code {
			t.Errorf("verdict %q → exit %d, want %d", verdict, got, code)
		}
	}
}

// Test 61 — the root passes its Center, flushes it and keeps the raw Warns
// print (a source scan, the bridgeonly idiom).
func TestSubagentRunRoot_PassesTheRootSignalCenterAndKeepsTheRawWarns(t *testing.T) {
	body, err := os.ReadFile("cmd_subagent.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	start := strings.Index(src, "func runSubagentRun(")
	end := strings.Index(src, "func renderRunOutcome(")
	if start < 0 || end < start {
		t.Fatal("runSubagentRun / renderRunOutcome not found")
	}
	fn := src[start:end]
	for _, want := range []string{"newRootSignalCenter(layout.ProjectRoot, layout.EvolveDir, stderr)", "defer signals.Flush()", "subagent.RunOptions{Signals: signals}", "range res.Warns", "return renderRunOutcome(res, agent, cycle, stderr)"} {
		if !strings.Contains(fn, want) {
			t.Errorf("runSubagentRun lacks %q", want)
		}
	}
	if strings.Contains(fn, "subagent.RunOptions{}") {
		t.Error("the root must not build a Center-less RunOptions")
	}
}
