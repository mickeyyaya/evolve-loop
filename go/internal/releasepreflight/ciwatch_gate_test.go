package releasepreflight

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// redCIOpts wires a happy-path repo whose release-commit CI verdict is a
// faked red run — no network, no live gh (AC2 of push-ci-watch-remote-parity,
// cycle-748).
func redCIOpts(t *testing.T) (Options, *bytes.Buffer) {
	t.Helper()
	opts := stubOpts(makeRepo(t, "1.0.0"), "1.0.1")
	var buf bytes.Buffer
	opts.Stderr = &buf
	opts.CIConclusion = func(string) (CIRunStatus, error) {
		return CIRunStatus{Conclusion: "failure", RunURL: "https://github.com/mickeyyaya/evolve-loop/actions/runs/42"}, nil
	}
	return opts, &buf
}

// TestRun_RefusesTagOnRedReleaseCommitCI pins the hard-gate: when the release
// commit's go CI run conclusion is not success, preflight refuses (v22.0.0
// was cut on red CI; this makes that structurally impossible without an
// explicit override).
func TestRun_RefusesTagOnRedReleaseCommitCI(t *testing.T) {
	opts, buf := redCIOpts(t)
	_, err := Run(opts)
	if !errors.Is(err, ErrCheckFailed) {
		t.Fatalf("err = %v, want ErrCheckFailed\nlog=%s", err, buf.String())
	}
	if !strings.Contains(err.Error(), "not success") || !strings.Contains(err.Error(), "failure") {
		t.Errorf("err = %v, want the red conclusion named", err)
	}
	if !strings.Contains(err.Error(), "--allow-red-ci") {
		t.Errorf("err = %v, want the override flag named for the operator", err)
	}

	// A pending (not yet completed) run must also refuse — "pushed" is not "green".
	opts2, _ := redCIOpts(t)
	opts2.CIConclusion = func(string) (CIRunStatus, error) {
		return CIRunStatus{Conclusion: "pending", RunURL: "https://x/runs/9"}, nil
	}
	if _, err := Run(opts2); !errors.Is(err, ErrCheckFailed) {
		t.Errorf("pending CI: err = %v, want ErrCheckFailed", err)
	}

	// Green CI passes the gate.
	opts3, _ := redCIOpts(t)
	opts3.CIConclusion = func(string) (CIRunStatus, error) {
		return CIRunStatus{Conclusion: "success"}, nil
	}
	res, err := Run(opts3)
	if err != nil {
		t.Fatalf("green CI: err = %v, want nil", err)
	}
	if res.CIConclusion != "success" || res.CIOverridden {
		t.Errorf("green CI: Result = {CIConclusion:%q CIOverridden:%v}, want success/false", res.CIConclusion, res.CIOverridden)
	}

	// Unavailable verdict (no run visible / gh absent) is advisory-skipped —
	// the determinism rule: absent tooling never blocks, only a present red does.
	opts4, buf4 := redCIOpts(t)
	opts4.CIConclusion = func(string) (CIRunStatus, error) { return CIRunStatus{}, nil }
	if _, err := Run(opts4); err != nil {
		t.Errorf("unavailable CI verdict: err = %v, want advisory skip\nlog=%s", err, buf4.String())
	}
}

// TestRun_CIOverrideAllowsRedCIAndLogsLoudly pins the override edge: the
// explicit operator flag lets a red-CI release proceed, is visible in the
// preflight log (never silent), and is recorded in the Result.
func TestRun_CIOverrideAllowsRedCIAndLogsLoudly(t *testing.T) {
	opts, buf := redCIOpts(t)
	opts.AllowRedCI = true

	res, err := Run(opts)
	if err != nil {
		t.Fatalf("Run with AllowRedCI: %v\nlog=%s", err, buf.String())
	}
	if !res.CIOverridden {
		t.Error("Result.CIOverridden = false, want true — the override must be recorded")
	}
	if res.CIConclusion != "failure" {
		t.Errorf("Result.CIConclusion = %q, want the real red verdict preserved", res.CIConclusion)
	}
	log := buf.String()
	if !strings.Contains(log, "OVERRIDE") {
		t.Errorf("preflight log must shout OVERRIDE, got:\n%s", log)
	}
	if !strings.Contains(log, "failure") || !strings.Contains(log, "--allow-red-ci") {
		t.Errorf("override log must name the red conclusion and the flag that allowed it:\n%s", log)
	}
}
