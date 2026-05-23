package fanoutdispatch

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRun_RejectsMissingArgs(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	if rc := Run(Config{}, &b); rc != ExitSetupErr {
		t.Errorf("rc=%d, want %d", rc, ExitSetupErr)
	}
}

func TestRun_RejectsMissingCommandsFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var b bytes.Buffer
	rc := Run(Config{
		CommandsFile: "/nonexistent.tsv",
		ResultsFile:  filepath.Join(dir, "r.tsv"),
	}, &b)
	if rc != ExitSetupErr {
		t.Errorf("rc=%d, want %d", rc, ExitSetupErr)
	}
}

func TestRun_RejectsMissingCachePrefixFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmds := filepath.Join(dir, "cmds.tsv")
	writeFile(t, cmds, "")
	var b bytes.Buffer
	rc := Run(Config{
		CommandsFile:    cmds,
		ResultsFile:     filepath.Join(dir, "r.tsv"),
		CachePrefixFile: "/nonexistent-prefix",
	}, &b)
	if rc != ExitSetupErr {
		t.Errorf("rc=%d, want %d", rc, ExitSetupErr)
	}
}

func TestRun_EmptyCommandsFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmds := filepath.Join(dir, "cmds.tsv")
	results := filepath.Join(dir, "r.tsv")
	writeFile(t, cmds, "")
	var b bytes.Buffer
	rc := Run(Config{CommandsFile: cmds, ResultsFile: results}, &b)
	if rc != ExitOK {
		t.Errorf("rc=%d, want 0", rc)
	}
	body, _ := os.ReadFile(results)
	if len(body) != 0 {
		t.Errorf("empty input should give empty output, got %d bytes", len(body))
	}
}

func TestRun_AllWorkersSucceed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmds := filepath.Join(dir, "cmds.tsv")
	results := filepath.Join(dir, "r.tsv")
	writeFile(t, cmds, "alpha\techo hello\nbeta\techo world\n")
	var b bytes.Buffer
	rc := Run(Config{
		CommandsFile: cmds, ResultsFile: results,
		Concurrency: 2, TimeoutSecs: 30,
	}, &b)
	if rc != ExitOK {
		t.Fatalf("rc=%d (log=%s)", rc, b.String())
	}
	body, _ := os.ReadFile(results)
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("results lines: got %d, want 2 (body=%q)", len(lines), body)
	}
	// preserve input order
	if !strings.HasPrefix(lines[0], "alpha\t") {
		t.Errorf("order broken: first line %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "beta\t") {
		t.Errorf("order broken: second line %q", lines[1])
	}
	// exit codes are 0
	for _, line := range lines {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		if fields[1] != "0" {
			t.Errorf("non-zero exit for %s: %s", fields[0], fields[1])
		}
	}
}

func TestRun_AnyWorkerFailedReturns1(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmds := filepath.Join(dir, "cmds.tsv")
	results := filepath.Join(dir, "r.tsv")
	writeFile(t, cmds, "good\techo ok\nbad\texit 7\n")
	var b bytes.Buffer
	rc := Run(Config{CommandsFile: cmds, ResultsFile: results, Concurrency: 2, TimeoutSecs: 10}, &b)
	if rc != ExitWorkerFail {
		t.Errorf("rc=%d, want 1", rc)
	}
	body, _ := os.ReadFile(results)
	if !strings.Contains(string(body), "bad\t7\t") {
		t.Errorf("missing bad worker line: %s", body)
	}
}

func TestRun_TimeoutKillsWorker(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmds := filepath.Join(dir, "cmds.tsv")
	results := filepath.Join(dir, "r.tsv")
	writeFile(t, cmds, "slow\tsleep 30\n")
	var b bytes.Buffer
	rc := Run(Config{
		CommandsFile: cmds, ResultsFile: results,
		Concurrency: 1, TimeoutSecs: 1,
	}, &b)
	if rc != ExitWorkerFail {
		t.Errorf("rc=%d, want 1 (timeout)", rc)
	}
	body, _ := os.ReadFile(results)
	if !strings.Contains(string(body), "slow\t124\t") {
		t.Errorf("expected timeout exit 124, got: %s", body)
	}
}

func TestRun_StdoutCapturedPerWorker(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmds := filepath.Join(dir, "cmds.tsv")
	results := filepath.Join(dir, "r.tsv")
	writeFile(t, cmds, "w1\techo capture-this\n")
	var b bytes.Buffer
	rc := Run(Config{CommandsFile: cmds, ResultsFile: results, Concurrency: 1, TimeoutSecs: 10}, &b)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	out, err := os.ReadFile(filepath.Join(dir, "w1.out"))
	if err != nil {
		t.Fatalf("missing .out file: %v", err)
	}
	if !strings.Contains(string(out), "capture-this") {
		t.Errorf("stdout not captured: %s", out)
	}
}

func TestRun_BoundedConcurrency(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmds := filepath.Join(dir, "cmds.tsv")
	results := filepath.Join(dir, "r.tsv")
	// 4 workers, concurrency 2 — should still complete
	var sb strings.Builder
	for i := 1; i <= 4; i++ {
		fmt.Fprintf(&sb, "w%d\techo %d\n", i, i)
	}
	writeFile(t, cmds, sb.String())
	var b bytes.Buffer
	rc := Run(Config{CommandsFile: cmds, ResultsFile: results, Concurrency: 2, TimeoutSecs: 10}, &b)
	if rc != ExitOK {
		t.Errorf("rc=%d", rc)
	}
	body, _ := os.ReadFile(results)
	for i := 1; i <= 4; i++ {
		if !strings.Contains(string(body), fmt.Sprintf("w%d\t0\t", i)) {
			t.Errorf("missing w%d in results: %s", i, body)
		}
	}
}

func TestReadCommands_IgnoresMalformedLines(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "cmds.tsv")
	writeFile(t, p, "\nno-tab-here\nname\tcmd\n\nempty-tab\t\n")
	cmds, err := ReadCommands(p)
	if err != nil {
		t.Fatal(err)
	}
	// should pick up "name\tcmd" and "empty-tab\t" (empty command is valid)
	if len(cmds) != 2 {
		t.Errorf("got %d commands, want 2: %v", len(cmds), cmds)
	}
}

func TestCheckFailConsensus(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.out"), "## Findings\nVerdict: PASS\n")
	writeFile(t, filepath.Join(dir, "b.out"), "Verdict: FAIL\n")
	writeFile(t, filepath.Join(dir, "c.out"), "verdict: **FAIL**\n")
	writeFile(t, filepath.Join(dir, "d.out"), "")

	if !checkFailConsensus(dir, 2) {
		t.Errorf("want true for 2 FAILs (k=2)")
	}
	if checkFailConsensus(dir, 3) {
		t.Errorf("want false for k=3 (only 2 FAILs)")
	}
}

func TestRun_TrackWorkersInvokesHelper(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmds := filepath.Join(dir, "cmds.tsv")
	results := filepath.Join(dir, "r.tsv")
	writeFile(t, cmds, "alpha\techo ok\n")

	// fake cycle-state helper that records each invocation
	logFile := filepath.Join(dir, "helper.log")
	helper := filepath.Join(dir, "cycle-state.sh")
	writeFile(t, helper, `#!/usr/bin/env bash
echo "$@" >> `+logFile+`
exit 0
`)
	_ = os.Chmod(helper, 0o755)

	var stderr bytes.Buffer
	rc := Run(Config{
		CommandsFile:        cmds,
		ResultsFile:         results,
		Concurrency:         1,
		TimeoutSecs:         10,
		TrackWorkers:        true,
		CycleStateHelperBin: helper,
	}, &stderr)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	body, _ := os.ReadFile(logFile)
	// Should have set-worker-status running + done
	if !strings.Contains(string(body), "running") {
		t.Errorf("helper not called with running: %s", body)
	}
	if !strings.Contains(string(body), "done") {
		t.Errorf("helper not called with done: %s", body)
	}
}

func TestRun_CachePrefixFileExported(t *testing.T) {
	t.Parallel()
	// Verify EVOLVE_FANOUT_CACHE_PREFIX_FILE is set by checking it from inside the worker.
	// Each test runs in its own process, so the env is contained.
	dir := t.TempDir()
	prefix := filepath.Join(dir, "prefix.txt")
	writeFile(t, prefix, "static cache bytes")
	cmds := filepath.Join(dir, "cmds.tsv")
	results := filepath.Join(dir, "r.tsv")
	writeFile(t, cmds, `w1	test -n "$EVOLVE_FANOUT_CACHE_PREFIX_FILE" && echo PREFIX_SET || echo PREFIX_UNSET
`)
	var b bytes.Buffer
	rc := Run(Config{
		CommandsFile:    cmds,
		ResultsFile:     results,
		CachePrefixFile: prefix,
		Concurrency:     1, TimeoutSecs: 10,
	}, &b)
	if rc != ExitOK {
		t.Fatalf("rc=%d", rc)
	}
	out, _ := os.ReadFile(filepath.Join(dir, "w1.out"))
	if !strings.Contains(string(out), "PREFIX_SET") {
		t.Errorf("prefix env not exported: %s", out)
	}
}

// safeStr is a sentinel for the parallel race detector path.
var _ = sync.Mutex{}
