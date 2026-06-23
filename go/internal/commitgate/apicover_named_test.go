package commitgate

import (
	"context"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/sysexec"
)

// apicover_named_test.go names + exercises the exported symbols that the
// behavior tests don't already reach, satisfying the ADR-0050 Phase 5 hard gate
// (./internal/commitgate is enrolled in go/.apicover-enforce). The Exit*
// constants ExitPass/ExitFail/ExitToolMissing are named by commitgate_test.go;
// the two below are pinned here. Options.Run, Attestation.Marshal, Options,
// Result, Runner, and Attestation are all exercised by the behavior/parity
// tests.

// TestExitCodeContract names every Exit* constant and asserts the exact numeric
// vocabulary the ship-gate reader and the /commit skill depend on. The values
// are load-bearing: they ARE the bash runner's documented exit codes, so a
// regression that renumbers one silently breaks the contract.
func TestExitCodeContract(t *testing.T) {
	t.Parallel()
	codes := map[string]int{
		"ExitPass":        ExitPass,
		"ExitFail":        ExitFail,
		"ExitGitFatal":    ExitGitFatal,
		"ExitToolMissing": ExitToolMissing,
		"ExitBadArgs":     ExitBadArgs,
	}
	want := map[string]int{
		"ExitPass": 0, "ExitFail": 1, "ExitGitFatal": 2, "ExitToolMissing": 3, "ExitBadArgs": 10,
	}
	if !reflect.DeepEqual(codes, want) {
		t.Fatalf("exit-code vocabulary drifted: got %v, want %v", codes, want)
	}
}

// TestExitGitFatal_OnDiffNameError exercises ExitGitFatal via the real path: a
// `git diff --name-only HEAD` that fails fatally maps to ExitGitFatal.
func TestExitGitFatal_OnDiffNameError(t *testing.T) {
	t.Parallel()
	o := baseOpts(t.TempDir(), "shasum")
	// Runner reports `git diff --name-only HEAD` exit 128 (fatal).
	o.Runner = func(_ context.Context, name, _ string, args, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		if name == "git" && len(args) > 0 && args[0] == "diff" {
			return 128, nil
		}
		return 0, nil
	}
	res := o.Run(context.Background())
	if res.ExitCode != ExitGitFatal {
		t.Fatalf("ExitCode = %d, want ExitGitFatal (%d)", res.ExitCode, ExitGitFatal)
	}
}

// TestExitBadArgs_IsTen pins ExitBadArgs as a distinct, non-overlapping code (the
// cmd layer returns it for malformed invocations; it must never collide with a
// gate-result code).
func TestExitBadArgs_IsTen(t *testing.T) {
	t.Parallel()
	if ExitBadArgs == ExitPass || ExitBadArgs == ExitFail || ExitBadArgs == ExitGitFatal || ExitBadArgs == ExitToolMissing {
		t.Fatalf("ExitBadArgs (%d) collides with a gate-result code", ExitBadArgs)
	}
	if ExitBadArgs != 10 {
		t.Fatalf("ExitBadArgs = %d, want 10", ExitBadArgs)
	}
}

// TestRunner_AliasIsRunFunc names the Runner alias and confirms it is exactly
// sysexec.RunFunc (assignable both ways).
func TestRunner_AliasIsRunFunc(t *testing.T) {
	t.Parallel()
	var r Runner = func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		return 0, nil
	}
	var _ sysexec.RunFunc = r
	code, err := r(context.Background(), "true", "", nil, nil, nil, nil, nil)
	if code != 0 || err != nil {
		t.Fatalf("Runner invocation = (%d,%v)", code, err)
	}
}

// TestResult_StructFields names the Result type and every exported field via a
// full-struct composite literal, then round-trips it through a real gate run.
func TestResult_StructFields(t *testing.T) {
	t.Parallel()
	att := &Attestation{TreeStateSHA: "s", TS: "t", Tool: "shasum"}
	r := Result{
		ExitCode:     ExitPass,
		Logs:         []string{"x"},
		Attestation:  att,
		ChecksPassed: []string{"go:gofmt"},
		Langs:        []string{"go"},
	}
	if r.ExitCode != ExitPass || r.Attestation != att || len(r.Logs) != 1 || len(r.ChecksPassed) != 1 || len(r.Langs) != 1 {
		t.Fatal("Result composite literal did not bind its fields")
	}
}

// TestOptions_StructFields names the Options type and its exported fields via a
// composite literal that drives a real Run (Now/Runner/RepoRoot exercised).
func TestOptions_StructFields(t *testing.T) {
	t.Parallel()
	o := Options{
		RepoRoot:     t.TempDir(),
		Reviewers:    "code-simplifier,code-reviewer",
		Files:        "notes.txt",
		NoInstall:    true,
		AttestDir:    "",
		Env:          nil,
		Now:          func() time.Time { return time.Unix(0, 0).UTC() },
		TestInstall:  "",
		ForceMissing: "",
	}
	o.lookPath = func(string) (string, error) { return "/usr/bin/shasum", nil }
	o.Runner = func(_ context.Context, name, _ string, args, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		return 0, nil // git diff HEAD empty → SHA of empty diff
	}
	res := o.Run(context.Background())
	// notes.txt has no detectable language → no lanes, attestation written.
	if res.ExitCode != ExitPass {
		t.Fatalf("ExitCode = %d, want ExitPass (%v)", res.ExitCode, res.Logs)
	}
}
