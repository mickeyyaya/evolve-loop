package commitgate

import (
	"context"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

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

func TestExitGitFatal_OnDiffNameError(t *testing.T) {
	t.Parallel()
	o := baseOpts(t.TempDir(), "shasum")
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

func TestExitBadArgs_IsTen(t *testing.T) {
	t.Parallel()
	if ExitBadArgs == ExitPass || ExitBadArgs == ExitFail || ExitBadArgs == ExitGitFatal || ExitBadArgs == ExitToolMissing {
		t.Fatalf("ExitBadArgs (%d) collides with a gate-result code", ExitBadArgs)
	}
	if ExitBadArgs != 10 {
		t.Fatalf("ExitBadArgs = %d, want 10", ExitBadArgs)
	}
}

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
		return 0, nil
	}
	res := o.Run(context.Background())
	if res.ExitCode != ExitPass {
		t.Fatalf("ExitCode = %d, want ExitPass (%v)", res.ExitCode, res.Logs)
	}
}
