package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// TestDispatch_NoArgs prints usage and returns 2.
func TestDispatch_NoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch(nil, nil, &stdout, &stderr)
	if code != 2 {
		t.Errorf("want exit 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "evolve — autonomous improvement loop") {
		t.Errorf("usage missing from stderr: %s", stderr.String())
	}
}

func TestDispatch_Version(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"version"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Errorf("want 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "evolve") {
		t.Errorf("want version output, got %q", stdout.String())
	}
}

func TestDispatch_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"--help"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Errorf("want 0, got %d", code)
	}
}

func TestDispatch_Unknown(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"flarp"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Errorf("want 2, got %d", code)
	}
}

func TestDispatch_PhaseRoutesToRunPhase(t *testing.T) {
	// Empty args after "phase" → runPhase emits "missing phase name" and exit 10.
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"phase"}, nil, &stdout, &stderr)
	if code != 10 {
		t.Errorf("want 10, got %d", code)
	}
	if !strings.Contains(stderr.String(), "missing phase name") {
		t.Errorf("stderr=%q", stderr.String())
	}
}

func TestDispatch_CycleRoutesToRunCycle(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"cycle"}, nil, &stdout, &stderr)
	if code != 10 {
		t.Errorf("want 10, got %d", code)
	}
	if !strings.Contains(stderr.String(), "missing subcommand") {
		t.Errorf("stderr=%q", stderr.String())
	}
}

func TestDispatch_WorktreeRoutesToRunWorktree(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"worktree"}, nil, &stdout, &stderr)
	if code != 10 {
		t.Errorf("want 10, got %d", code)
	}
	if !strings.Contains(stderr.String(), "missing subcommand") {
		t.Errorf("stderr=%q", stderr.String())
	}
}

func TestDispatch_LoopRoutesToRunLoop(t *testing.T) {
	// loop with no args still reaches runLoop which then errors on
	// missing goal (v11.5.0 M1: accepts --goal-hash, --goal-text,
	// positional goal, or --resume; error wording reflects the menu).
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"loop"}, nil, &stdout, &stderr)
	if code != 10 {
		t.Errorf("want 10, got %d", code)
	}
	if !strings.Contains(stderr.String(), "a goal is required") {
		t.Errorf("stderr=%q", stderr.String())
	}
}

func TestDispatch_DoctorProbe_Found(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"doctor", "probe", "go"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Errorf("doctor probe go: want 0, got %d, stderr=%s", code, stderr.String())
	}
}

func TestDispatch_DoctorProbe_JSON_ReorderedFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	// flag after positional — exercises reorderArgs.
	code := dispatch([]string{"doctor", "probe", "go", "--json"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Errorf("want 0, got %d, stderr=%s", code, stderr.String())
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v / %q", err, stdout.String())
	}
	if got["tool"] != "go" {
		t.Errorf("want tool=go, got %v", got)
	}
}

func TestDispatch_DoctorProbe_NotFound(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"doctor", "probe", "no_such_binary_evolve_test_xyzzy"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Errorf("want 1, got %d", code)
	}
}

func TestDispatch_DoctorProbe_Quiet(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"doctor", "probe", "--quiet", "go"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Errorf("want 0, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("want empty stdout in --quiet mode, got %q", stdout.String())
	}
}

func TestDispatch_DoctorProbe_NoTool(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"doctor", "probe"}, nil, &stdout, &stderr)
	if code != 10 {
		t.Errorf("want 10, got %d", code)
	}
}

func TestDispatch_Doctor_NoSub(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"doctor"}, nil, &stdout, &stderr)
	if code != 10 {
		t.Errorf("want 10, got %d", code)
	}
}

func TestDispatch_Doctor_BadSub(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"doctor", "flarp"}, nil, &stdout, &stderr)
	if code != 10 {
		t.Errorf("want 10, got %d", code)
	}
}

func TestDispatch_GuardShip_Allow(t *testing.T) {
	// ship.sh-shaped command — guard should allow.
	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"bash scripts/lifecycle/ship.sh msg"}}`)
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"guard", "ship"}, in, &stdout, &stderr)
	if code != 0 {
		t.Errorf("want 0, got %d, stderr=%s", code, stderr.String())
	}
}

func TestDispatch_GuardShip_Deny(t *testing.T) {
	// Hermetic: a guard-deny assertion must not be flipped by an ambient
	// operator bypass env (e.g. EVOLVE_BYPASS_SHIP_GATE=1 set in a dev
	// session's settings.local.json). Clear it for this test.
	t.Setenv("EVOLVE_BYPASS_SHIP_GATE", "")
	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"git commit -m bypass"}}`)
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"guard", "ship"}, in, &stdout, &stderr)
	if code != 2 {
		t.Errorf("want 2 (deny), got %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "DENY") {
		t.Errorf("want DENY in stderr, got %q", stderr.String())
	}
}

func TestDispatch_GuardUnknown(t *testing.T) {
	in := strings.NewReader(`{}`)
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"guard", "ghostguard"}, in, &stdout, &stderr)
	if code != 10 {
		t.Errorf("want 10, got %d", code)
	}
}

func TestDispatch_Guard_NoName(t *testing.T) {
	in := strings.NewReader(`{}`)
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"guard"}, in, &stdout, &stderr)
	if code != 10 {
		t.Errorf("want 10, got %d", code)
	}
}

func TestDispatch_Guard_BadJSON(t *testing.T) {
	in := strings.NewReader(`{not json`)
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"guard", "ship"}, in, &stdout, &stderr)
	if code != 10 {
		t.Errorf("want 10 (bad input), got %d", code)
	}
}

func TestDispatch_Ledger_Verify_Empty(t *testing.T) {
	tmp := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"ledger", "verify", "--evolve-dir", tmp}, nil, &stdout, &stderr)
	if code != 0 {
		t.Errorf("empty ledger should verify OK, got %d (%s)", code, stderr.String())
	}
}

func TestDispatch_Ledger_Verify_Broken(t *testing.T) {
	tmp := t.TempDir()
	// Write a malformed ledger to provoke a chain break.
	if err := os.WriteFile(filepath.Join(tmp, "ledger.jsonl"), []byte("garbage\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "ledger.tip"), []byte("0:deadbeef"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"ledger", "verify", "--evolve-dir", tmp}, nil, &stdout, &stderr)
	if code != 2 {
		t.Errorf("broken ledger should exit 2, got %d", code)
	}
}

func TestDispatch_Ledger_Tail_Empty(t *testing.T) {
	tmp := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"ledger", "tail", "--evolve-dir", tmp}, nil, &stdout, &stderr)
	if code != 0 {
		t.Errorf("empty tail should be OK, got %d", code)
	}
}

func TestDispatch_Ledger_Tail_Populated(t *testing.T) {
	tmp := t.TempDir()
	l := ledger.New(tmp)
	for i := 0; i < 3; i++ {
		if err := l.Append(context.Background(), core.LedgerEntry{TS: "x", Cycle: i, Role: "test"}); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"ledger", "tail", "--evolve-dir", tmp, "--n", "2"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("tail returned %d (%s)", code, stderr.String())
	}
	lines := strings.Count(stdout.String(), "\n")
	if lines != 2 {
		t.Errorf("want 2 tail lines, got %d (%s)", lines, stdout.String())
	}
}

func TestDispatch_Ledger_NoSub(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"ledger"}, nil, &stdout, &stderr)
	if code != 10 {
		t.Errorf("want 10, got %d", code)
	}
}

func TestDispatch_Ledger_BadSub(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"ledger", "flarp"}, nil, &stdout, &stderr)
	if code != 10 {
		t.Errorf("want 10, got %d", code)
	}
}

func TestDispatch_ACS_NoSub(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"acs"}, nil, &stdout, &stderr)
	if code != 10 {
		t.Errorf("want 10, got %d", code)
	}
}

func TestDispatch_ACS_BadSub(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"acs", "flarp"}, nil, &stdout, &stderr)
	if code != 10 {
		t.Errorf("want 10, got %d", code)
	}
}

func TestDispatch_ACS_Run_NoCycle(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"acs", "run", "./..."}, nil, &stdout, &stderr)
	if code != 10 {
		t.Errorf("want 10 (missing --cycle), got %d", code)
	}
}

func TestDispatch_ACS_Run_NoPkg(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"acs", "run", "--cycle", "1"}, nil, &stdout, &stderr)
	if code != 10 {
		t.Errorf("want 10 (missing pkg), got %d", code)
	}
}

// Helper to silence unused io import warning when needed.
var _ = io.EOF

func TestReorderArgs(t *testing.T) {
	cases := []struct {
		in, want []string
	}{
		{nil, []string{}},
		{[]string{"foo"}, []string{"foo"}},
		{[]string{"--json", "foo"}, []string{"--json", "foo"}},
		{[]string{"foo", "--json"}, []string{"--json", "foo"}},
		{[]string{"foo", "--json", "--quiet", "bar"}, []string{"--json", "--quiet", "foo", "bar"}},
	}
	for _, c := range cases {
		got := reorderArgs(c.in)
		if !equalStrSlice(got, c.want) {
			t.Errorf("reorderArgs(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func equalStrSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
