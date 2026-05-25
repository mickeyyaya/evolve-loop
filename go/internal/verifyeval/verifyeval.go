// Package verifyeval independently re-executes the commands inside an
// eval Markdown file against a workspace and compares the observed
// outcome to the eval's expectations. It is the trust-boundary check
// the orchestrator uses pre-ship: "don't trust Auditor's claim that
// the eval passed — re-run it ourselves."
//
// Eval format (the contract this package consumes):
//
//	```bash
//	go test ./internal/foo/...
//	```
//
//	## Expected
//
//	exit_code: 0
//	stdout_contains: "FAIL"  # (negated when prefixed with !)
//	stderr_contains: ""
//
// The package executes each ```bash``` block in the eval (one command
// per non-comment line), captures stdout/stderr/exit, and applies the
// Expected predicates. Any failure flips the overall verdict to FAIL.
//
// Production execution shells out via opts.Runner. Tests inject a
// fake runner so no real shell ever runs in unit tests. The shell-
// out path itself is exercised via the //go:build integration tag.
//
// v12.1 Phase 2A port. CLI: `evolve eval verify <eval.md> <workspace>`.
package verifyeval

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CmdRunner is the seam for command execution. The production impl
// shells out via /bin/sh; tests inject a fake.
type CmdRunner func(ctx context.Context, workdir, command string) (stdout, stderr string, exitCode int, err error)

// Expectations captures the predicates parsed from the eval's
// ## Expected section. Each field is optional; absent fields are not
// enforced.
type Expectations struct {
	ExitCode       *int   // when non-nil, observed exit must match
	StdoutContains string // when non-empty, stdout must contain this substring
	StdoutAbsent   string // when non-empty, stdout must NOT contain this
	StderrContains string // same for stderr
}

// Options configures Verify. Path is the eval markdown file;
// Workspace is the directory commands run in; Runner overrides
// execution (defaults to DefaultRunner).
type Options struct {
	Path      string
	Workspace string
	Runner    CmdRunner
}

// CommandResult captures the outcome of one executed command.
type CommandResult struct {
	Command  string
	Stdout   string
	Stderr   string
	ExitCode int
	Passed   bool
	Reason   string // populated when Passed is false
}

// Result is the overall verdict + per-command outcomes.
type Result struct {
	Path     string
	Verdict  string // "PASS" or "FAIL"
	Commands []CommandResult
}

// Verify executes every bash command in the eval and checks each
// against the parsed Expectations. Returns Verdict=PASS when every
// command satisfies the predicates; FAIL on first mismatch (subsequent
// commands still run so the operator sees the full failure picture).
func Verify(opts Options) (Result, error) {
	if opts.Path == "" {
		return Result{}, fmt.Errorf("verifyeval: Path required")
	}
	if opts.Workspace == "" {
		return Result{}, fmt.Errorf("verifyeval: Workspace required")
	}
	runner := opts.Runner
	if runner == nil {
		runner = DefaultRunner
	}

	commands, expect, err := parseEval(opts.Path)
	if err != nil {
		return Result{}, err
	}

	res := Result{Path: opts.Path, Verdict: "PASS"}
	ctx := context.Background()
	for _, cmd := range commands {
		stdout, stderr, exit, runErr := runner(ctx, opts.Workspace, cmd)
		cr := CommandResult{
			Command:  cmd,
			Stdout:   stdout,
			Stderr:   stderr,
			ExitCode: exit,
		}
		if runErr != nil {
			cr.Reason = fmt.Sprintf("runner error: %v", runErr)
			res.Verdict = "FAIL"
			res.Commands = append(res.Commands, cr)
			continue
		}
		if reason := matchExpectations(cr, expect); reason != "" {
			cr.Reason = reason
			res.Verdict = "FAIL"
		} else {
			cr.Passed = true
		}
		res.Commands = append(res.Commands, cr)
	}
	return res, nil
}

// matchExpectations checks one command's outcome against the parsed
// predicates. Returns "" when all checks pass; otherwise a one-line
// reason naming the first failed predicate.
func matchExpectations(cr CommandResult, e Expectations) string {
	if e.ExitCode != nil && cr.ExitCode != *e.ExitCode {
		return fmt.Sprintf("exit_code=%d, expected %d", cr.ExitCode, *e.ExitCode)
	}
	if e.StdoutContains != "" && !strings.Contains(cr.Stdout, e.StdoutContains) {
		return fmt.Sprintf("stdout missing %q", e.StdoutContains)
	}
	if e.StdoutAbsent != "" && strings.Contains(cr.Stdout, e.StdoutAbsent) {
		return fmt.Sprintf("stdout unexpectedly contains %q", e.StdoutAbsent)
	}
	if e.StderrContains != "" && !strings.Contains(cr.Stderr, e.StderrContains) {
		return fmt.Sprintf("stderr missing %q", e.StderrContains)
	}
	return ""
}

// parseEval extracts bash commands and the Expected predicate block
// from the eval markdown file.
func parseEval(path string) ([]string, Expectations, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, Expectations{}, fmt.Errorf("verifyeval: open %s: %w", path, err)
	}
	defer f.Close()

	var commands []string
	var expect Expectations
	inBash := false
	inExpected := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inBash {
				inBash = false
			} else if strings.Contains(trimmed, "bash") {
				inBash = true
			}
			continue
		}
		if strings.HasPrefix(trimmed, "## Expected") || strings.HasPrefix(trimmed, "## expected") {
			inExpected = true
			inBash = false
			continue
		}
		if strings.HasPrefix(trimmed, "## ") && inExpected {
			inExpected = false
		}
		if inBash {
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				commands = append(commands, trimmed)
			}
			continue
		}
		if inExpected {
			parseExpectedLine(trimmed, &expect)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, Expectations{}, fmt.Errorf("verifyeval: read %s: %w", path, err)
	}
	return commands, expect, nil
}

// parseExpectedLine populates Expectations from one `key: value` line
// inside the ## Expected section. Unknown keys are silently ignored
// for forward compatibility.
func parseExpectedLine(line string, e *Expectations) {
	colon := strings.Index(line, ":")
	if colon < 0 {
		return
	}
	key := strings.TrimSpace(line[:colon])
	val := strings.TrimSpace(line[colon+1:])
	// Strip optional surrounding quotes.
	val = strings.Trim(val, `"'`)
	switch key {
	case "exit_code":
		var n int
		if _, err := fmt.Sscanf(val, "%d", &n); err == nil {
			e.ExitCode = &n
		}
	case "stdout_contains":
		e.StdoutContains = val
	case "stdout_absent":
		e.StdoutAbsent = val
	case "stderr_contains":
		e.StderrContains = val
	}
}

// DefaultRunner shells out to /bin/sh -c. Marked here for clarity;
// the integration-tag tests exercise it against /bin/true and friends.
func DefaultRunner(ctx context.Context, workdir, command string) (string, string, int, error) {
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	if workdir != "" {
		cmd.Dir = workdir
	}
	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
		err = nil // exit-error is not a runner error
	}
	return stdoutBuf.String(), stderrBuf.String(), exitCode, err
}
