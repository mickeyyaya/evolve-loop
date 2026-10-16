package consensusdispatch

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type runFixture struct {
	dir       string
	workspace string
	profile   string
	prompt    string
}

func newRunFixture(t *testing.T, consensus map[string]any) runFixture {
	t.Helper()
	dir := t.TempDir()
	f := runFixture{
		dir:       dir,
		workspace: filepath.Join(dir, "ws"),
		profile:   filepath.Join(dir, "prof.json"),
		prompt:    filepath.Join(dir, "prompt.md"),
	}
	writeProfile(t, f.profile, consensus)
	writeFile(t, f.prompt, "audit")
	return f
}

func (f runFixture) inputs() Inputs {
	return Inputs{
		Cycle:         "1",
		WorkspacePath: f.workspace,
		ProfilePath:   f.profile,
		PromptFile:    f.prompt,
		TierFor:       func(string) (string, error) { return "full", nil },
	}
}

func twoVoterConsensus() map[string]any {
	return map[string]any{"enabled": true, "cli_voters": []string{"claude", "gemini"}, "quorum": 2}
}

func runCapturingStderr(in Inputs) (int, string) {
	var stdout, stderr bytes.Buffer
	rc := Run(in, &stdout, &stderr)
	return rc, stderr.String()
}

func TestRun_MissingPromptFileIsRuntimeError(t *testing.T) {
	t.Parallel()
	f := newRunFixture(t, twoVoterConsensus())
	in := f.inputs()
	in.PromptFile = filepath.Join(f.dir, "absent.md")

	rc, stderr := runCapturingStderr(in)

	if rc != ExitRuntimeErr {
		t.Fatalf("rc=%d want %d (stderr=%s)", rc, ExitRuntimeErr, stderr)
	}
	if !strings.Contains(stderr, "prompt file missing") {
		t.Errorf("stderr must name the missing prompt file: %s", stderr)
	}
}

func TestRun_UnparsableProfileIsProfileError(t *testing.T) {
	t.Parallel()
	f := newRunFixture(t, twoVoterConsensus())
	writeFile(t, f.profile, "{not json")

	rc, stderr := runCapturingStderr(f.inputs())

	if rc != ExitProfileErr {
		t.Fatalf("rc=%d want %d (stderr=%s)", rc, ExitProfileErr, stderr)
	}
	if !strings.Contains(stderr, "PROFILE-ERROR") {
		t.Errorf("stderr must flag a profile error: %s", stderr)
	}
}

func TestRun_NegativeQuorumIsProfileError(t *testing.T) {
	t.Parallel()
	f := newRunFixture(t, map[string]any{"enabled": true, "cli_voters": []string{"claude", "gemini"}, "quorum": -1})

	rc, stderr := runCapturingStderr(f.inputs())

	if rc != ExitProfileErr {
		t.Fatalf("rc=%d want %d (stderr=%s)", rc, ExitProfileErr, stderr)
	}
	if !strings.Contains(stderr, "quorum must be integer") {
		t.Errorf("stderr must name the quorum: %s", stderr)
	}
}

func TestRun_WorkersDirBlockedIsRuntimeError(t *testing.T) {
	t.Parallel()
	f := newRunFixture(t, twoVoterConsensus())
	writeFile(t, filepath.Join(f.workspace, "consensus-workers"), "a file where the dir belongs")

	rc, stderr := runCapturingStderr(f.inputs())

	if rc != ExitRuntimeErr {
		t.Fatalf("rc=%d want %d (stderr=%s)", rc, ExitRuntimeErr, stderr)
	}
	if !strings.Contains(stderr, "cannot create workers dir") {
		t.Errorf("stderr must name the workers dir failure: %s", stderr)
	}
}

func TestRun_CommandsTSVWriteFailureIsRuntimeError(t *testing.T) {
	t.Parallel()
	f := newRunFixture(t, twoVoterConsensus())
	if err := os.MkdirAll(filepath.Join(f.workspace, "consensus-workers", ".commands.tsv"), 0o755); err != nil {
		t.Fatal(err)
	}

	rc, stderr := runCapturingStderr(f.inputs())

	if rc != ExitRuntimeErr {
		t.Fatalf("rc=%d want %d (stderr=%s)", rc, ExitRuntimeErr, stderr)
	}
	if !strings.Contains(stderr, "cannot write commands TSV") {
		t.Errorf("stderr must name the TSV write failure: %s", stderr)
	}
}

func TestRun_FanoutResolutionFailureIsRuntimeError(t *testing.T) {
	t.Parallel()
	f := newRunFixture(t, twoVoterConsensus())
	var subcommands []string
	in := f.inputs()
	in.resolveDispatch = func(_, subcmd string, _ []string) (*exec.Cmd, error) {
		subcommands = append(subcommands, subcmd)
		return nil, errors.New("no native binary")
	}

	rc, stderr := runCapturingStderr(in)

	if rc != ExitRuntimeErr {
		t.Fatalf("rc=%d want %d (stderr=%s)", rc, ExitRuntimeErr, stderr)
	}
	if len(subcommands) != 1 || subcommands[0] != "fanout-dispatch" {
		t.Errorf("only fanout-dispatch may be resolved before the failure, got %v", subcommands)
	}
}

func TestRun_AggregatorResolutionFailureIsRuntimeError(t *testing.T) {
	t.Parallel()
	trueBin, err := exec.LookPath("true")
	if err != nil {
		t.Skip("no true binary on PATH")
	}
	f := newRunFixture(t, twoVoterConsensus())
	for _, cli := range []string{"claude", "gemini"} {
		writeFile(t, filepath.Join(f.workspace, "consensus-workers", cli+"-audit.md"), "PASS")
	}
	var subcommands []string
	in := f.inputs()
	in.resolveDispatch = func(_, subcmd string, _ []string) (*exec.Cmd, error) {
		subcommands = append(subcommands, subcmd)
		if subcmd == "aggregator" {
			return nil, errors.New("no native binary")
		}
		return exec.Command(trueBin), nil
	}

	rc, stderr := runCapturingStderr(in)

	if rc != ExitRuntimeErr {
		t.Fatalf("rc=%d want %d (stderr=%s)", rc, ExitRuntimeErr, stderr)
	}
	if len(subcommands) != 2 || subcommands[1] != "aggregator" {
		t.Errorf("aggregator must be resolved after fanout, got %v", subcommands)
	}
}

func TestResolveEvolveBin_FallsBackToPathLookup(t *testing.T) {
	pathDir := t.TempDir()
	want := filepath.Join(pathDir, "evolve")
	writeExec(t, want, "#!/bin/sh\nexit 0\n")
	t.Setenv("EVOLVE_GO_BIN", "")
	t.Setenv("PATH", pathDir)

	got := resolveEvolveBin(filepath.Join(t.TempDir(), "hint"))

	if got != want {
		t.Fatalf("resolveEvolveBin=%q want the PATH entry %q", got, want)
	}
}
