// gitops_unit_test.go — seam-injected unit tests for the lowest-coverage
// helpers in dryrun.go / gitops.go that the integration matrix doesn't
// hit (the matrix uses real git via os/exec, which exercises end-to-end
// happy paths but misses small branches). Phase 3 of the v12.1 plan:
// raise ship package coverage from 53% to ≥95%.
package ship

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scriptedRunner returns a CmdRunner that responds to (binary, firstArg)
// pairs with scripted stdout + exit code. Unknown pairs return exit 0.
// Records the call sequence so tests can assert order.
type scriptedRunner struct {
	scripts map[string]struct {
		stdout string
		stderr string
		exit   int
		err    error
	}
	calls []string
}

func (s *scriptedRunner) runner() CmdRunner {
	if s.scripts == nil {
		s.scripts = map[string]struct {
			stdout string
			stderr string
			exit   int
			err    error
		}{}
	}
	return func(ctx context.Context, name, cwd string, args, env []string,
		stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		// Scan past leading flags (-C path, -c foo=bar) to find the
		// git subcommand, so test keys read as "git symbolic-ref" rather
		// than "git -C".
		key := name
		for i := 0; i < len(args); i++ {
			a := args[i]
			if !strings.HasPrefix(a, "-") {
				key = name + " " + a
				break
			}
			// Skip the next arg if this flag takes a value.
			if a == "-C" || a == "-c" {
				i++
			}
		}
		s.calls = append(s.calls, key)
		script, ok := s.scripts[key]
		if !ok {
			return 0, nil
		}
		if stdout != nil && script.stdout != "" {
			_, _ = stdout.Write([]byte(script.stdout))
		}
		if stderr != nil && script.stderr != "" {
			_, _ = stderr.Write([]byte(script.stderr))
		}
		return script.exit, script.err
	}
}

// TestTryGitOneShot_Success — exit 0 + stdout returns trimmed.
func TestTryGitOneShot_Success(t *testing.T) {
	r := &scriptedRunner{scripts: map[string]struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{}}
	r.scripts["git rev-parse"] = struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{stdout: "abc123\n  ", exit: 0}
	opts := &Options{Runner: r.runner(), ProjectRoot: "/tmp"}
	got := tryGitOneShot(context.Background(), opts, "rev-parse", "HEAD")
	if got != "abc123" {
		t.Errorf("tryGitOneShot=%q, want abc123", got)
	}
}

// TestTryGitOneShot_NonZeroExit_ReturnsEmpty — failure means empty.
func TestTryGitOneShot_NonZeroExit_ReturnsEmpty(t *testing.T) {
	r := &scriptedRunner{scripts: map[string]struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{}}
	r.scripts["git rev-parse"] = struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{stdout: "should-be-ignored", exit: 1}
	opts := &Options{Runner: r.runner(), ProjectRoot: "/tmp"}
	if got := tryGitOneShot(context.Background(), opts, "rev-parse", "HEAD"); got != "" {
		t.Errorf("non-zero exit should yield empty; got %q", got)
	}
}

// TestTryGitOneShot_RunnerError_ReturnsEmpty — runner error swallowed.
func TestTryGitOneShot_RunnerError_ReturnsEmpty(t *testing.T) {
	r := &scriptedRunner{scripts: map[string]struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{}}
	r.scripts["git rev-parse"] = struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{err: errors.New("spawn fail")}
	opts := &Options{Runner: r.runner(), ProjectRoot: "/tmp"}
	if got := tryGitOneShot(context.Background(), opts, "rev-parse", "HEAD"); got != "" {
		t.Errorf("runner error should yield empty; got %q", got)
	}
}

// TestWriteDryRunJournal_NoOpWhenDryRunFalse — opts.DryRun=false is a
// no-op; no file written, no DryRunPath set.
func TestWriteDryRunJournal_NoOpWhenDryRunFalse(t *testing.T) {
	root := t.TempDir()
	r := &scriptedRunner{}
	opts := &Options{DryRun: false, ProjectRoot: root, Runner: r.runner()}
	res := &RunResult{}
	writeDryRunJournal(context.Background(), opts, res, "test-reason")
	if res.DryRunPath != "" {
		t.Errorf("DryRunPath=%q, want empty", res.DryRunPath)
	}
	// No journal directory should exist.
	if _, err := os.Stat(filepath.Join(root, ".evolve", "release-journal")); err == nil {
		t.Errorf("journal dir created despite DryRun=false")
	}
}

// TestWriteDryRunJournal_HappyPath_WritesJSON — DryRun=true writes a
// valid JSON file with the expected fields.
func TestWriteDryRunJournal_HappyPath_WritesJSON(t *testing.T) {
	root := t.TempDir()
	r := &scriptedRunner{scripts: map[string]struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{}}
	r.scripts["git rev-parse"] = struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{stdout: "abc123\n", exit: 0}
	opts := &Options{
		DryRun:        true,
		ProjectRoot:   root,
		Class:         Class("manual"),
		CommitMessage: "test commit",
		Runner:        r.runner(),
	}
	res := &RunResult{
		Logs: []string{
			"[ship] [DRY-RUN] would commit on main",
			"[ship] not a dry-run line",
			"[ship] [DRY-RUN] would push to origin",
		},
	}
	writeDryRunJournal(context.Background(), opts, res, "verified-ok")

	if res.DryRunPath == "" {
		t.Fatal("DryRunPath empty after writeDryRunJournal")
	}
	body, err := os.ReadFile(res.DryRunPath)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("journal is not valid JSON: %v\n%s", err, body)
	}
	if got["class"] != "manual" {
		t.Errorf("class=%v, want manual", got["class"])
	}
	if got["commit_msg"] != "test commit" {
		t.Errorf("commit_msg=%v, want 'test commit'", got["commit_msg"])
	}
	if got["exit_reason"] != "verified-ok" {
		t.Errorf("exit_reason=%v, want verified-ok", got["exit_reason"])
	}
	wouldHave, ok := got["would_have"].([]any)
	if !ok || len(wouldHave) != 2 {
		t.Errorf("would_have should contain 2 DRY-RUN log lines; got %v", got["would_have"])
	}
}

// TestWriteDryRunJournal_MissingGitDefaults_PlaceholderBranchSHA —
// when git probes return empty, branch/headSHA default to "unknown".
func TestWriteDryRunJournal_MissingGitDefaults_PlaceholderBranchSHA(t *testing.T) {
	root := t.TempDir()
	r := &scriptedRunner{scripts: map[string]struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{}}
	// Don't script any git command; runner returns exit 0 with empty stdout.
	r.scripts["git rev-parse"] = struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{stdout: "", exit: 1} // simulate failure
	opts := &Options{DryRun: true, ProjectRoot: root, Class: Class("cycle"), Runner: r.runner()}
	res := &RunResult{}
	writeDryRunJournal(context.Background(), opts, res, "test")

	body, _ := os.ReadFile(res.DryRunPath)
	var got map[string]any
	_ = json.Unmarshal(body, &got)
	if got["branch"] != "unknown" {
		t.Errorf("branch=%v, want unknown (git failed)", got["branch"])
	}
	if got["head_sha_at_dry_run"] != "unknown" {
		t.Errorf("head_sha_at_dry_run=%v, want unknown", got["head_sha_at_dry_run"])
	}
}

// TestWriteDryRunJournal_LogAppended — successful write appends a log
// line announcing the journal path.
func TestWriteDryRunJournal_LogAppended(t *testing.T) {
	root := t.TempDir()
	r := &scriptedRunner{}
	opts := &Options{DryRun: true, ProjectRoot: root, Class: Class("cycle"), Runner: r.runner()}
	res := &RunResult{}
	writeDryRunJournal(context.Background(), opts, res, "test")
	lastLog := res.Logs[len(res.Logs)-1]
	if !strings.Contains(lastLog, "DRY-RUN: journal preview written to") {
		t.Errorf("expected journal log line; got %q", lastLog)
	}
}

// TestCaptureGitOutputAtDir_Success — happy path: subprocess succeeds,
// returns trimmed stdout.
func TestCaptureGitOutputAtDir_Success(t *testing.T) {
	r := &scriptedRunner{scripts: map[string]struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{}}
	r.scripts["git symbolic-ref"] = struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{stdout: "feature-branch\n", exit: 0}
	opts := &Options{Runner: r.runner(), ProjectRoot: "/tmp"}
	got, err := captureGitOutputAtDir(context.Background(), opts, "/tmp/wt", "symbolic-ref", "--short", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(got) != "feature-branch" {
		t.Errorf("got %q, want feature-branch", got)
	}
}

// TestCaptureGitOutputAtDir_ExitGreaterThan1_ReturnsError — captureGitOutput
// only errors on exit > 1 (rc=1 means "differences exist" for git diff).
func TestCaptureGitOutputAtDir_ExitGreaterThan1_ReturnsError(t *testing.T) {
	r := &scriptedRunner{scripts: map[string]struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{}}
	r.scripts["git symbolic-ref"] = struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{stderr: "fatal: not a git repo", exit: 128}
	opts := &Options{Runner: r.runner(), ProjectRoot: "/tmp"}
	_, err := captureGitOutputAtDir(context.Background(), opts, "/tmp/wt", "symbolic-ref", "--short", "HEAD")
	if err == nil {
		t.Error("expected error on exit > 1")
	}
}

// TestCaptureGitOutputAtDir_Exit1_NotError — rc=1 is the "differences
// exist" signal from git diff; the function treats it as success.
func TestCaptureGitOutputAtDir_Exit1_NotError(t *testing.T) {
	r := &scriptedRunner{scripts: map[string]struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{}}
	r.scripts["git diff"] = struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{stdout: "diff output\n", exit: 1}
	opts := &Options{Runner: r.runner(), ProjectRoot: "/tmp"}
	out, err := captureGitOutputAtDir(context.Background(), opts, "/tmp", "diff")
	if err != nil {
		t.Errorf("rc=1 should not be an error: %v", err)
	}
	if !strings.Contains(out, "diff output") {
		t.Errorf("stdout lost: %q", out)
	}
}

// TestMaybeCreateRelease_NonReleaseClass_SkipsEarly — Class=cycle skips
// the release-creation path entirely.
func TestMaybeCreateRelease_NonReleaseClass_SkipsEarly(t *testing.T) {
	r := &scriptedRunner{}
	opts := &Options{Class: Class("cycle"), ProjectRoot: t.TempDir(), Runner: r.runner()}
	res := &RunResult{}
	if err := maybeCreateRelease(context.Background(), opts, res); err != nil {
		t.Fatal(err)
	}
	for _, call := range r.calls {
		if strings.HasPrefix(call, "gh") {
			t.Errorf("gh should not be called for class=cycle; got %q", call)
		}
	}
}

// TestMaybeCreateRelease_MissingPluginJson_LogsAndContinues — release
// class without a plugin.json logs a skip and returns nil (not fatal).
func TestMaybeCreateRelease_MissingPluginJson_LogsAndContinues(t *testing.T) {
	r := &scriptedRunner{}
	root := t.TempDir()
	opts := &Options{
		Class:       Class("release"),
		ProjectRoot: root,
		PluginRoot:  root, // no plugin.json present
		Runner:      r.runner(),
	}
	res := &RunResult{}
	err := maybeCreateRelease(context.Background(), opts, res)
	if err != nil {
		t.Errorf("missing plugin.json should not be fatal; got %v", err)
	}
}

// TestScriptedRunner_DefaultExitZero — sanity test on the test helper.
func TestScriptedRunner_DefaultExitZero(t *testing.T) {
	r := &scriptedRunner{}
	var stdout bytes.Buffer
	exit, err := r.runner()(context.Background(), "git", "", []string{"status"}, nil, nil, &stdout, nil)
	if err != nil || exit != 0 {
		t.Errorf("unscripted call should return (0, nil); got (%d, %v)", exit, err)
	}
}
