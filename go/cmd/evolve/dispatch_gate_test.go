package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentclass"
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
	{[]string{"inbox", "add", "--bogus"}, "inbox add", "class R"},
	{[]string{"quality", "report", "--bogus"}, "quality report", "class R"},
	{[]string{"subagent", "run"}, "subagent run", "class D"},
	{[]string{"subagent", "dispatch-parallel"}, "subagent", "class D"},
	{[]string{"loop", "--bogus"}, "loop", "class D"},
	{[]string{"wave", "next", "--bogus"}, "wave next", "class D"},
	{[]string{"wave", "stop", "--bogus"}, "wave stop", "class D"},
	{[]string{"loop-stop", "--bogus"}, "loop-stop", "class D"},
	{[]string{"inbox", "claim", "--bogus"}, "inbox claim", "class D"},
	{[]string{"inbox-mover", "claim"}, "inbox-mover", "class D"},
	{[]string{"landing", "approve", "--bogus"}, "landing", "class D"},
	{[]string{"cycle", "run", "--bogus"}, "cycle", "class D"},
	{[]string{"cycle", "resume", "--bogus"}, "cycle resume", "class D"},
	{[]string{"boundary", "run", "--bogus"}, "boundary run", "class D"},
	{[]string{"bridge", "launch", "--bogus"}, "bridge", "class D"},
	{[]string{"cli-routing", "show", "--bogus"}, "cli-routing show", "class D"},
	{[]string{"ci", "classify", "--rerun", "--bogus"}, "ci classify", "class D"},
	{[]string{"compose", "--bogus"}, "compose", "class D"},
	{[]string{"fanout-dispatch", "--bogus"}, "fanout-dispatch", "class D"},
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
			if code != exitDispatchRefused || !strings.Contains(out, "dispatch credential") || !strings.Contains(out, "refused inside a phase: "+c.verb+" is "+c.class) {
				t.Fatalf("evolve %v with no credential = exit %d %q, want exit %d naming the credential and %q", c.args, code, out, exitDispatchRefused, c.verb+" is "+c.class)
			}
			if !strings.Contains(out, "ISOLATION_I3_VIOLATION") || !strings.Contains(out, "verb.class.") {
				t.Fatalf("the refusal must emit ISOLATION_I3_VIOLATION with the class rule, got %q", out)
			}
			if (c.class == "class R") != strings.Contains(out, "/requests/") {
				t.Fatalf("only a class R refusal names a request path, got %q", out)
			}
		})
	}
}

func TestAgentClass_RefusesWithoutTheCredentialAndNamesTheRequestPath(t *testing.T) {
	t.Parallel()
	gate := dispatchGate{open: unheldStore(t, "enforce")}
	var r, d bytes.Buffer
	rc := dispatchWith(gate, []string{"inbox", "add", "--bogus"}, strings.NewReader(""), &bytes.Buffer{}, &r)
	if want := agentclass.RequestPath("", "inbox-add"); rc != exitDispatchRefused || !strings.Contains(r.String(), want) {
		t.Fatalf("inbox add with no credential = exit %d %q, want exit %d naming %q", rc, r.String(), exitDispatchRefused, want)
	}
	dc := dispatchWith(gate, []string{"wave", "next", "--bogus"}, strings.NewReader(""), &bytes.Buffer{}, &d)
	if dc != exitDispatchRefused || strings.Contains(d.String(), "/requests/") || !strings.Contains(d.String(), "operator or boundary verb") {
		t.Fatalf("wave next with no credential = exit %d %q, want exit %d with no request path", dc, d.String(), exitDispatchRefused)
	}
}

func TestAgentClass_EveryVerbOfTheTableIsGatedByItsClass(t *testing.T) {
	t.Parallel()
	gate := dispatchGate{open: unheldStore(t, "enforce")}
	for _, v := range agentclass.Verbs() {
		args := strings.Fields(strings.ReplaceAll(v.Path, "<name>", "scout"))
		if v.OnlyWith != "" {
			args = append(args, v.OnlyWith)
		}
		var stderr bytes.Buffer
		refused := gate.refuse(args, &stderr)
		if refused != (v.Class != agentclass.Allowed) {
			t.Errorf("evolve %v (class %s): refused %v, stderr %q", args, v.Class, refused, stderr.String())
		}
	}
}

func TestAgentClass_AnAVerbInACellRunsAsOutsideACell(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	run := func(open func() (dispatchcred.Store, error)) (int, string, string) {
		var stdout, stderr bytes.Buffer
		code := dispatchWith(dispatchGate{open: open}, []string{"phase", "verify", "build", "--project-root", root}, strings.NewReader(""), &stdout, &stderr)
		return code, stdout.String(), stderr.String()
	}
	cellCode, cellOut, cellErr := run(unheldStore(t, "enforce"))
	hostCode, hostOut, hostErr := run(heldStore(t))
	if cellCode != hostCode || cellOut != hostOut || cellErr != hostErr || strings.Contains(cellErr, "dispatch credential") {
		t.Fatalf("phase verify build in a cell = %d %q %q, outside = %d %q %q; want the same", cellCode, cellOut, cellErr, hostCode, hostOut, hostErr)
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
		{[]string{"help"}, 0, ""},
	} {
		var stdout, stderr bytes.Buffer
		code := dispatchWith(gate, c.args, strings.NewReader(""), &stdout, &stderr)
		if code != c.code || !strings.Contains(stderr.String(), c.out) || strings.Contains(stderr.String(), "dispatch credential") {
			t.Errorf("evolve %v = exit %d %q, want exit %d with %q and no credential check", c.args, code, stderr.String(), c.code, c.out)
		}
	}
}

func TestBoundaryLoopStep_TheInProcessLoopPassesTheGate(t *testing.T) {
	t.Parallel()
	var refused, held bytes.Buffer
	if code := gatedLoop(dispatchGate{open: unheldStore(t, "enforce")})([]string{"--bogus"}, strings.NewReader(""), &bytes.Buffer{}, &refused); code != exitDispatchRefused || !strings.Contains(refused.String(), "loop is class D") {
		t.Fatalf("the loop step of boundary run must check the credential, got exit %d %q", code, refused.String())
	}
	if code := gatedLoop(dispatchGate{open: heldStore(t)})([]string{"--bogus"}, strings.NewReader(""), &bytes.Buffer{}, &held); code == exitDispatchRefused || strings.Contains(held.String(), "dispatch credential") {
		t.Fatalf("a host that holds the credential runs the loop step, got exit %d %q", code, held.String())
	}
}

func TestAgentClass_AnAliasAndAFlagFirstLoopStatusTakeTheirClass(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	if code := dispatchWith(dispatchGate{open: unheldStore(t, "enforce")}, []string{"--version"}, strings.NewReader(""), &bytes.Buffer{}, &stderr); code != 0 || stderr.Len() > 0 {
		t.Errorf("an alias takes the class of its command: evolve --version in a cell = %d %q", code, stderr.String())
	}
	if m := agentClassOf([]string{"loop", "--project-root", "P", "status"}); m.Class != agentclass.Allowed {
		t.Errorf("loop status after a flag = %+v, want class A", m)
	}
}
