package bridge

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func capableProbe() sandbox.ProbeResult {
	return sandbox.ProbeResult{OS: "darwin", Available: true, CapabilityChecked: true, Capable: true, BinaryPath: "/usr/bin/sandbox-exec"}
}

func gateLine(stderr string) string {
	for _, line := range strings.Split(stderr, "\n") {
		if strings.Contains(line, "sandbox requirement unsatisfied") {
			return line
		}
	}
	return ""
}

const nestedMarker = "nested LLM-CLI session"

func launchRequiredSeat(t *testing.T, wrapFor func(Deps) SandboxWrapper, subpaths []string) (int, error, string) {
	t.Helper()
	var stderr bytes.Buffer
	runner := &fakeRunner{}
	deps := Deps{
		Runner: runner.runner(), Stderr: &stderr, SandboxMode: config.SandboxModeOn,
		Env: map[string]string{"CLAUDECODE": "1", "ANTHROPIC_API_KEY": "", "ANTHROPIC_BASE_URL": ""},
	}
	deps.SandboxWrap = wrapFor(deps)
	logs := t.TempDir()
	rc, err := claudePDriver{}.Launch(context.Background(), &Config{
		CLI: "claude-p", Agent: "build", PromptFile: agyPromptFile(t), Model: "auto", RequireSandbox: true,
		Worktree: t.TempDir(), Workspace: t.TempDir(), SandboxWriteSubpaths: subpaths,
		StdoutLog: filepath.Join(logs, "out.log"), StderrLog: filepath.Join(logs, "err.log"),
	}, deps)
	if len(runner.calls) != 0 {
		t.Fatalf("an unconfined run started: %d calls", len(runner.calls))
	}
	return rc, err, stderr.String()
}

func TestSandboxRefusal_ARefusedWriteGrantIsNamedAndNestingIsNot(t *testing.T) {
	t.Parallel()
	rc, err, stderr := launchRequiredSeat(t, func(deps Deps) SandboxWrapper {
		return defaultSandboxWrapWithProbe(deps, capableProbe)
	}, []string{""})
	if err != nil || rc != ExitSafetyGate {
		t.Fatalf("rc=%d err=%v, want ExitSafetyGate", rc, err)
	}
	if !strings.Contains(stderr, "sandbox requirement unsatisfied: sandbox write grant unavailable") {
		t.Fatalf("stderr = %q, want the refusal to name the write grant", stderr)
	}
	if strings.Contains(stderr, nestedMarker) {
		t.Fatalf("stderr = %q, a refused write grant must not be reported as nesting", stderr)
	}
}

func TestSandboxRefusal_ANestedProbeThatIsNotCapableStillNamesNesting(t *testing.T) {
	t.Parallel()
	probe := func() sandbox.ProbeResult {
		return sandbox.ProbeResult{OS: "darwin", Available: true, CapabilityChecked: true, Reason: "sandbox_apply denied"}
	}
	rc, err, stderr := launchRequiredSeat(t, func(deps Deps) SandboxWrapper {
		return defaultSandboxWrapWithProbe(deps, probe)
	}, nil)
	if err != nil || rc != ExitSafetyGate {
		t.Fatalf("rc=%d err=%v, want ExitSafetyGate", rc, err)
	}
	gate := gateLine(stderr)
	if !strings.Contains(gate, nestedMarker) || !strings.Contains(gate, "sandbox_apply denied") {
		t.Fatalf("gate line = %q, want nesting and the probe reason when the probe was the cause", gate)
	}
}

func TestWrapOutcome_DescribeNamesTheCauseThatRefused(t *testing.T) {
	t.Parallel()
	const confinement = "nested LLM-CLI session: inner sandbox unavailable"
	for _, tc := range []struct {
		name  string
		cause RefusalCause
		out   WrapOutcome
		want  string
	}{
		{"no reason", CauseUnspecified, WrapOutcome{}, confinement},
		{"probe", CauseProbe, WrapOutcome{Cause: CauseProbe, Reason: "bwrap denied"}, confinement + "; probe: bwrap denied"},
		{"profile", CauseProfile, WrapOutcome{Cause: CauseProfile, Reason: "sandbox write grant unavailable: x"}, "sandbox write grant unavailable: x"},
	} {
		if got := tc.out.describe(confinement); got != tc.want || tc.out.Cause != tc.cause {
			t.Errorf("%s: describe=%q cause=%v, want %q and %v", tc.name, got, tc.out.Cause, tc.want, tc.cause)
		}
	}
}

func TestSandboxRefusal_ASeatHolderThatCannotBeCreatedIsNamedAndNestingIsNot(t *testing.T) {
	t.Setenv("HOME", "")
	rc, err, stderr := launchRequiredSeat(t, func(deps Deps) SandboxWrapper {
		return defaultSandboxWrapWithProbe(deps, capableProbe)
	}, nil)
	if err != nil || rc != ExitSafetyGate {
		t.Fatalf("rc=%d err=%v, want ExitSafetyGate", rc, err)
	}
	gate := gateLine(stderr)
	if !strings.Contains(gate, "the seat token holder cannot be created") {
		t.Fatalf("gate line = %q, want it to name the seat holder", gate)
	}
	if strings.Contains(stderr, nestedMarker) {
		t.Fatalf("stderr = %q, a refused seat holder must not be reported as nesting", stderr)
	}
}
