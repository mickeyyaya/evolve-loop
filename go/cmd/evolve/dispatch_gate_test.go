package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchcred"
)

func unheldStore(t *testing.T, stage string) func() (dispatchcred.Store, error) {
	t.Helper()
	s := dispatchcred.Store{Dir: filepath.Join(t.TempDir(), "dispatch")}
	if err := s.Ensure(stage); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(s.HeldDir(), "credential"), 0o644); err != nil {
		t.Fatal(err)
	}
	return func() (dispatchcred.Store, error) { return s, nil }
}

func heldStore(t *testing.T) func() (dispatchcred.Store, error) {
	t.Helper()
	s := dispatchcred.Store{Dir: filepath.Join(t.TempDir(), "dispatch")}
	return func() (dispatchcred.Store, error) { return s, nil }
}

var dispatchVerbCalls = []struct {
	args  []string
	verb  string
	class string
}{
	{[]string{"agent", "run", "--bogus"}, "agent run", "class R"},
	{[]string{"phase", "scout", "--bogus"}, "phase scout", "class R"},
	{[]string{"subagent", "run"}, "subagent run", "class D"},
	{[]string{"subagent", "dispatch-parallel"}, "subagent dispatch-parallel", "class D"},
	{[]string{"loop", "--bogus"}, "loop", "class D"},
	{[]string{"wave", "next", "--bogus"}, "wave next", "class D"},
}

func TestDispatchVerbs_RefuseWithoutTheDispatchCredential(t *testing.T) {
	t.Parallel()
	gate := dispatchGate{open: unheldStore(t, "enforce")}
	for _, c := range dispatchVerbCalls {
		t.Run(c.verb, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			code := dispatchWith(gate, c.args, strings.NewReader(""), &stdout, &stderr)
			out := stderr.String()
			if code != exitDispatchRefused || !strings.Contains(out, "dispatch credential") || !strings.Contains(out, c.verb+" is "+c.class) {
				t.Fatalf("evolve %v with no credential = exit %d %q, want exit %d naming the credential and %q", c.args, code, out, exitDispatchRefused, c.verb+" is "+c.class)
			}
			if !strings.Contains(out, "ISOLATION_I3_VIOLATION") || !strings.Contains(out, "verb.class.") {
				t.Fatalf("the refusal must emit ISOLATION_I3_VIOLATION with the class rule, got %q", out)
			}
		})
	}
}

func TestDispatchVerbs_TheRefusalNamesTheRequestPathForClassROnly(t *testing.T) {
	t.Parallel()
	gate := dispatchGate{open: unheldStore(t, "enforce")}
	var r, d bytes.Buffer
	dispatchWith(gate, []string{"agent", "run", "--bogus"}, strings.NewReader(""), &bytes.Buffer{}, &r)
	dispatchWith(gate, []string{"wave", "next", "--bogus"}, strings.NewReader(""), &bytes.Buffer{}, &d)
	if !strings.Contains(r.String(), dispatchRequestPath) {
		t.Fatalf("a class R refusal must name the request path %q, got %q", dispatchRequestPath, r.String())
	}
	if strings.Contains(d.String(), dispatchRequestPath) || !strings.Contains(d.String(), "operator or boundary verb") {
		t.Fatalf("a class D refusal names no request path, got %q", d.String())
	}
}

func TestDispatchVerbs_ShadowReportsAndRuns(t *testing.T) {
	t.Parallel()
	gate := dispatchGate{open: unheldStore(t, "shadow")}
	var stderr bytes.Buffer
	code := dispatchWith(gate, []string{"subagent", "run"}, strings.NewReader(""), &bytes.Buffer{}, &stderr)
	if code == exitDispatchRefused || !strings.Contains(stderr.String(), "ISOLATION_I3_VIOLATION") || !strings.Contains(stderr.String(), "expected <agent> <cycle> <workspace>") {
		t.Fatalf("shadow must report the violation and run the verb, got exit %d %q", code, stderr.String())
	}
}

func TestDispatchVerbs_OffAndAHeldCredentialRunWithNoSignal(t *testing.T) {
	t.Parallel()
	for name, open := range map[string]func() (dispatchcred.Store, error){
		"off":  unheldStore(t, "off"),
		"held": heldStore(t),
	} {
		var stderr bytes.Buffer
		code := dispatchWith(dispatchGate{open: open}, []string{"subagent", "run"}, strings.NewReader(""), &bytes.Buffer{}, &stderr)
		if code == exitDispatchRefused || strings.Contains(stderr.String(), "ISOLATION") {
			t.Errorf("%s: the verb must run with no signal, got exit %d %q", name, code, stderr.String())
		}
	}
}

func TestDispatchVerbs_AnUnresolvableStoreIsShadow(t *testing.T) {
	t.Parallel()
	gate := dispatchGate{open: func() (dispatchcred.Store, error) { return dispatchcred.Store{}, errors.New("no passwd") }}
	var stderr bytes.Buffer
	code := dispatchWith(gate, []string{"subagent", "run"}, strings.NewReader(""), &bytes.Buffer{}, &stderr)
	if code == exitDispatchRefused || !strings.Contains(stderr.String(), "no passwd") {
		t.Fatalf("a store that cannot resolve reports its cause and does not refuse, got exit %d %q", code, stderr.String())
	}
}

func TestDispatchVerbs_ReadVerbsNeedNoCredential(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"phase", "verify"}, {"phase", "lint"}, {"phase"}, {"loop", "status"}, {"wave", "status"}, {"wave"},
		{"subagent", "resolve-tier"}, {"subagent"}, {"agent", "list"}, {"agent"}, {"version"},
	} {
		if verb, gated := dispatchVerbOf(args); gated {
			t.Errorf("evolve %v is gated as %q; only the dispatch verbs need the credential", args, verb.name)
		}
	}
	for _, c := range dispatchVerbCalls {
		if verb, gated := dispatchVerbOf(c.args); !gated || verb.name != c.verb {
			t.Errorf("evolve %v = %q gated %v, want %q", c.args, verb.name, gated, c.verb)
		}
	}
}

func TestDispatch_TheOperatorShellHoldsTheCredential(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := dispatch([]string{"subagent", "run"}, strings.NewReader(""), &stdout, &stderr); code == exitDispatchRefused || strings.Contains(stderr.String(), "ISOLATION") {
		t.Fatalf("the operator path must hold the credential and run the verb, got exit %d %q", code, stderr.String())
	}
}

func TestDispatchWith_AVerbOutsideTheGateAndABadCallNeedNoCredential(t *testing.T) {
	t.Parallel()
	gate := dispatchGate{open: unheldStore(t, "enforce")}
	for _, c := range []struct {
		args []string
		code int
		out  string
	}{
		{nil, 2, "Usage:"},
		{[]string{"no-such-verb"}, 2, `unknown command "no-such-verb"`},
		{[]string{"version"}, 0, ""},
	} {
		var stdout, stderr bytes.Buffer
		code := dispatchWith(gate, c.args, strings.NewReader(""), &stdout, &stderr)
		if code != c.code || !strings.Contains(stderr.String(), c.out) || strings.Contains(stderr.String(), "dispatch credential") {
			t.Errorf("evolve %v = exit %d %q, want exit %d with %q and no credential check", c.args, code, stderr.String(), c.code, c.out)
		}
	}
}
