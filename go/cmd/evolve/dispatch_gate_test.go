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
	{[]string{"cycle", "run", "--bogus"}, "cycle run", "class D"},
	{[]string{"boundary", "run", "--bogus"}, "boundary run", "class D"},
	{[]string{"bridge", "launch", "--bogus"}, "bridge launch", "class D"},
	{[]string{"routing", "plan", "--bogus"}, "routing plan", "class D"},
	{[]string{"campaign", "run", "--bogus"}, "campaign", "class D"},
	{[]string{"compose", "--bogus"}, "compose", "class D"},
	{[]string{"plan-and-execute", "--bogus"}, "plan-and-execute", "class D"},
	{[]string{"serve-phase", "--bogus"}, "serve-phase", "class D"},
	{[]string{"fleet", "--bogus"}, "fleet", "class D"},
	{[]string{"consensus-dispatch", "--bogus"}, "consensus-dispatch", "class D"},
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
		{"cycle", "reset"}, {"boundary"}, {"bridge", "probe"}, {"routing", "explain"}, {"campaign", "status"},
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

var commandsWithNoDispatch = map[string]string{
	"version": "prints", "help": "prints", "rebuild": "builds the plane binary", "console-lease": "a lease file",
	"install": "copies files", "uninstall": "removes files", "guard": "a hook check of the cell", "ledger": "reads",
	"dossier": "reads", "audit": "reads", "salvage": "reads", "soak-report": "reads", "dashboard": "serves reads",
	"names": "scans files", "acs": "runs predicates", "apicover": "measures", "inbox": "inbox files",
	"phases": "the phase catalog", "worktree": "git worktrees", "land": "applies a patch", "branches": "git refs",
	"continuation": "state files", "carryover": "state files", "swarm": "tmux sessions of workers", "checkpoint": "git refs",
	"treewalk": "scans files", "kb": "reads", "gc": "removes leftovers", "verdicts": "a verdict store",
	"failures": "state files", "loop-stop": "a brake file", "pr": "gh merges", "postmortem": "reads transcripts",
	"handoff": "reads", "ci": "gh reads", "comments": "scans files", "status": "reads", "ship": "git commit and push",
	"reset-sha": "a pin file", "sync-main": "git merge", "quality": "readings", "commit-gate": "lint and tests",
	"scan": "scans a diff", "detect-cli": "reads the env", "detect-nested-claude": "reads the env", "phase-order": "reads",
	"convergence": "a pure decision", "estimate-quota-reset": "a pure estimate", "build-invocation-context": "prints a prefix",
	"resolve-llm": "a routing decision", "cycle-simulator": "no LLM", "phase-watchdog": "watches files",
	"aggregator": "merges files", "preflight-environment": "probes the host", "phase-observer": "tails a stream",
	"changelog-gen": "git log", "version-bump": "edits files", "marketplace-poll": "gh reads", "release-preflight": "checks",
	"rollback": "git revert", "release": "the release pipeline", "prune-ephemeral": "removes files",
	"postedit-validate": "a hook check of the cell", "inbox-mover": "inbox files", "commit-prefix-gate": "a check",
	"release-consistency": "reads", "release-verify-binaries": "gh reads", "release-promote": "gh edits", "backups": "reads",
	"skill-inventory": "a cache", "skills": "generates files", "flags": "generates files", "signals": "generates files",
	"phase-inventory": "a cache", "eval": "checks", "docs": "lint", "solution": "checks", "cycle-health": "reads",
	"selfcheck": "the build probe of the cell", "tokens": "reads", "lessons": "reads", "reachability": "go list",
	"ratchet": "checks", "floor": "go test tiers", "cover": "coverage", "code": "the code index", "ladder": "reads",
	"context-fill":        "reads",
	"doctor":              launchAPIOnly,
	"setup":               launchAPIOnly,
	"cli-routing":         launchAPIOnly,
	"release-verify-clis": launchAPIOnly,
	"models":              launchAPIOnly,
	"clihealth":           launchAPIOnly,
	"cli":                 launchAPIOnly,
	"practice":            launchAPIOnly,
}

const launchAPIOnly = "its LLM execs go through the bridge launch API, which checks the credential (requireDispatchCredential)"

func TestDispatchGate_EveryRegistryCommandIsGatedOrNamedWithNoDispatch(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, c := range commands {
		seen[c.Name] = true
		_, gated := dispatchRules[c.Name]
		reason, named := commandsWithNoDispatch[c.Name]
		switch {
		case gated && named:
			t.Errorf("evolve %s is gated and also named with no dispatch (%q)", c.Name, reason)
		case !gated && !named:
			t.Errorf("evolve %s is a new command: add it to dispatchRules if it can reach an LLM launch, else to commandsWithNoDispatch with the reason", c.Name)
		}
	}
	for name := range dispatchRules {
		if !seen[name] {
			t.Errorf("dispatchRules names %q, which the registry does not list", name)
		}
	}
	for name := range commandsWithNoDispatch {
		if !seen[name] {
			t.Errorf("commandsWithNoDispatch names %q, which the registry does not list", name)
		}
	}
	called := map[string]bool{}
	for _, c := range dispatchVerbCalls {
		called[c.args[0]] = true
	}
	for name := range dispatchRules {
		if !called[name] {
			t.Errorf("the gated command %s has no refusal case in dispatchVerbCalls", name)
		}
	}
}
