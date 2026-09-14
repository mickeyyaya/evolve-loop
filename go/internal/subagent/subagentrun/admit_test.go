package subagentrun

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Test 20 — the six admission checks reject in their fixed order, each ONE
// REQUEST_REJECTED with its reason_class, phase = the full agent name, an
// empty run id, the run-id resolver never called and no port reached.
func TestAdmit_RejectsInOrderWithReasonClass(t *testing.T) {
	f := newFixture(t)
	deps := happyDeps(t)
	runIDCalls := 0
	deps.RunID = func(string) string { runIDCalls++; return "run-7" }
	deps.Profile = func(string) (Profile, error) { panic("no port is reached after a rejection") }
	deps.GuardDepth = func(depth int) error {
		if depth > 3 {
			return errDepth
		}
		return nil
	}
	req := Request{Agent: "bogus-worker-deep", Cycle: -1, WorkspacePath: "/non/existent", LegacyAgentDispatch: true, DispatchDepth: 4}
	steps := []struct {
		class string
		fix   func(*Request)
		want  string
		is    error
		extra map[string]string
	}{
		{"prompt_reader", func(*Request) {}, "subagent/run: PromptReader required (PROMPT_FILE_OVERRIDE or stdin)", nil, nil},
		{"unknown_agent", func(r *Request) { r.Prompt = strings.NewReader("hi") }, "subagent/run: unknown agent: bogus-worker-deep", nil, nil},
		{"cycle", func(r *Request) { r.Agent = "scout-worker-deep" }, "subagent/run: cycle must be >= 0, got -1", nil, nil},
		{"workspace", func(r *Request) { r.Cycle = 7 }, "subagent/run: workspace dir does not exist: /non/existent", nil, map[string]string{"workspace": "/non/existent"}},
		{"legacy_dispatch", func(r *Request) { r.WorkspacePath = f.ws }, "", ErrInProcessDispatchBanned, nil},
		{"depth", func(r *Request) { r.LegacyAgentDispatch = false }, "", errDepth, map[string]string{"depth": "4"}},
	}
	for _, s := range steps {
		s.fix(&req)
		d, r := observed(t, deps)
		out, err := d.Dispatch(context.Background(), req)
		if err == nil || (s.want != "" && err.Error() != s.want) || (s.is != nil && !errors.Is(err, s.is)) || out.Verdict != "" {
			t.Fatalf("%s: err %v out %+v", s.class, err, out)
		}
		e := r.only(t, CodeRequestRejected)
		if e.Fields["reason_class"] != s.class || e.Fields["step"] != "validate" || e.Phase != req.Agent || e.RunID != "" || e.Cycle != req.Cycle || e.Reason != err.Error() {
			t.Fatalf("%s: %+v", s.class, e)
		}
		for k, v := range s.extra {
			if e.Fields[k] != v {
				t.Errorf("%s: field %s=%q, want %q", s.class, k, e.Fields[k], v)
			}
		}
		if e.Fields["role"] != strings.TrimSuffix(req.Agent, "-worker-deep") || e.Fields["worker"] != "deep" {
			t.Errorf("%s: role/worker parsed: %+v", s.class, e.Fields)
		}
	}
	if runIDCalls != 0 {
		t.Fatalf("a rejected request never becomes a run: %d run-id reads", runIDCalls)
	}
}

// Test 21 — moved verbatim from the host: the worker-name grammar.
func TestParseAgentName(t *testing.T) {
	tests := []struct {
		in, role, worker string
	}{
		{"scout", "scout", ""},
		{"auditor", "auditor", ""},
		{"scout-worker-codebase", "scout", "codebase"},
		{"tdd-engineer-worker-unit-tests", "tdd-engineer", "unit-tests"},
		{"weird_name", "weird_name", ""},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			role, worker := parseAgentName(tc.in)
			if role != tc.role || worker != tc.worker {
				t.Errorf("got (%q,%q), want (%q,%q)", role, worker, tc.role, tc.worker)
			}
		})
	}
}

// Test 63 (review fold, architecture M3) — admission returns the identity
// COMPLETE: the run id is stamped as the gate's last act, after the depth
// guard, so no later step completes it by a pointer side effect and a
// producer inserted between admit and resolve carries it. A rejected request
// still never reads it (test 20); role → depth → run_id → profile is test
// 49's order — kills `stamp the run id in resolve`, `read the run id before
// the depth guard`.
func TestAdmit_ReturnsTheIdentityCompleteAsTheGatesLastAct(t *testing.T) {
	f := newFixture(t)
	deps := happyDeps(t)
	calls := 0
	deps.RunID = func(string) string { calls++; return "run-7" }
	deps.GuardDepth = func(int) error {
		if calls != 0 {
			t.Fatal("the run id is read after the depth guard, never before")
		}
		return nil
	}
	d, _ := observed(t, deps)
	req := f.request()
	req.Agent = "auditor-worker-deep"
	id, err := d.admit(req)
	if err != nil || calls != 1 {
		t.Fatalf("admit: %v, %d run-id reads", err, calls)
	}
	if want := (identity{agent: "auditor-worker-deep", role: "auditor", worker: "deep", runID: "run-7"}); id != want {
		t.Fatalf("admit returns the complete identity:\n got %+v\nwant %+v", id, want)
	}
}
