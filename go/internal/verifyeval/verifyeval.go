// Package verifyeval re-executes the commands of an eval Markdown file and checks the outcome.
// See docs/architecture/packages/internal-verifyeval.md.
package verifyeval

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type CmdRunner func(ctx context.Context, workdir, command string) (stdout, stderr string, exitCode int, err error)

type Expectations struct {
	ExitCode       *int
	StdoutContains string
	StdoutAbsent   string
	StderrContains string
}

type Options struct {
	Path      string
	Workspace string
	Runner    CmdRunner
}

type CommandResult struct {
	Command  string
	Stdout   string
	Stderr   string
	ExitCode int
	Passed   bool
	Reason   string
}

type Result struct {
	Path     string
	Verdict  string
	Commands []CommandResult
}

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

	scripts, expect, err := parseEval(opts.Path)
	if err != nil {
		return Result{}, err
	}
	if len(scripts) == 0 {
		return Result{}, fmt.Errorf("verifyeval: no bash scripts found in %s", opts.Path)
	}
	if expect.ExitCode == nil {
		zero := 0
		expect.ExitCode = &zero
	}

	res := Result{Path: opts.Path, Verdict: "PASS"}
	ctx := context.Background()
	for _, script := range scripts {
		cr := runScript(ctx, runner, opts.Workspace, script, expect)
		if !cr.Passed {
			res.Verdict = "FAIL"
		}
		res.Commands = append(res.Commands, cr)
	}
	return res, nil
}

func runScript(ctx context.Context, runner CmdRunner, workspace, script string, expect Expectations) CommandResult {
	stdout, stderr, exit, runErr := runner(ctx, workspace, script)
	cr := CommandResult{
		Command:  script,
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exit,
	}
	if runErr != nil {
		cr.Reason = fmt.Sprintf("runner error: %v", runErr)
		return cr
	}
	if reason := matchExpectations(cr, expect); reason != "" {
		cr.Reason = reason
	} else if reason := executionEvidenceReason(cr); reason != "" {
		cr.Reason = reason
	} else {
		cr.Passed = true
	}
	return cr
}

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

func parseEval(path string) ([]string, Expectations, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, Expectations{}, fmt.Errorf("verifyeval: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	var scripts []string
	var expect Expectations
	var script strings.Builder
	inBash := false
	inExpected := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if inBash {
			if strings.HasPrefix(trimmed, "```") {
				appendScript(&scripts, &script)
				inBash = false
			} else {
				script.WriteString(line)
				script.WriteByte('\n')
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") && strings.Contains(trimmed, "bash") {
			inBash = true
			script.Reset()
			continue
		}
		if strings.HasPrefix(trimmed, "## Expected") || strings.HasPrefix(trimmed, "## expected") {
			inExpected = true
			continue
		}
		if strings.HasPrefix(trimmed, "## ") && inExpected {
			inExpected = false
		}
		if inExpected {
			parseExpectedLine(trimmed, &expect)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, Expectations{}, fmt.Errorf("verifyeval: read %s: %w", path, err)
	}
	if inBash {
		appendScript(&scripts, &script)
	}
	return scripts, expect, nil
}

func appendScript(scripts *[]string, script *strings.Builder) {
	content := strings.TrimSpace(script.String())
	if scriptHasCommand(content) {
		*scripts = append(*scripts, content)
	}
	script.Reset()
}

func scriptHasCommand(script string) bool {
	for _, line := range strings.Split(script, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			return true
		}
	}
	return false
}

func parseExpectedLine(line string, e *Expectations) {
	colon := strings.Index(line, ":")
	if colon < 0 {
		return
	}
	key := strings.TrimSpace(line[:colon])
	val := strings.TrimSpace(line[colon+1:])
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

func DefaultRunner(ctx context.Context, workdir, command string) (string, string, int, error) {
	cmd := sysexec.Command(ctx, "/bin/bash", "-o", "pipefail", "-c", command)
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
		err = nil
	}
	return stdoutBuf.String(), stderrBuf.String(), exitCode, err
}
