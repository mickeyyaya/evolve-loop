package fixtures

import (
	"context"
	"strings"
	"testing"
)

func TestFakeExecRun_WritesScriptedStdoutAndStderrToTheirOwnSinks(t *testing.T) {
	f := &FakeExec{Scripts: map[string]ExecResponse{
		"git": {Stdout: "out-text", Stderr: "err-text", ExitCode: 3},
	}}
	var out, errb strings.Builder

	code, err := f.Run(context.Background(), "git", "/dir", []string{"status"}, nil, nil, &out, &errb)

	if err != nil || code != 3 {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if out.String() != "out-text" || errb.String() != "err-text" {
		t.Errorf("stdout=%q stderr=%q: each stream goes to its own sink", out.String(), errb.String())
	}
	if len(f.Calls) != 1 || f.Calls[0].Name != "git" || f.Calls[0].Dir != "/dir" {
		t.Errorf("calls = %+v", f.Calls)
	}
}

func TestFakeExecRun_NilSinksAndEmptyScriptedStreamsAreHarmless(t *testing.T) {
	f := &FakeExec{Scripts: map[string]ExecResponse{"git": {Stderr: "only-stderr"}}}
	var out strings.Builder

	if code, err := f.Run(context.Background(), "git", "", nil, nil, nil, &out, nil); err != nil || code != 0 {
		t.Fatalf("nil stderr sink: code=%d err=%v", code, err)
	}
	if out.Len() != 0 {
		t.Errorf("an empty scripted stdout writes nothing: %q", out.String())
	}
	var errb strings.Builder
	if _, err := f.Run(context.Background(), "git", "", nil, nil, nil, nil, &errb); err != nil || errb.String() != "only-stderr" {
		t.Errorf("nil stdout sink must not stop stderr: %q %v", errb.String(), err)
	}
}
