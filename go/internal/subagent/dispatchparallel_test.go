package subagent

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/aggregator"
	"github.com/mickeyyaya/evolve-loop/go/internal/capability"
	"github.com/mickeyyaya/evolve-loop/go/internal/fanoutdispatch"
)

func dispatchHappyOpts(t *testing.T, profileBody string) DispatchParallelOptions {
	t.Helper()
	clock := fixedClock(t, "2026-05-23T17:30:00Z")
	return DispatchParallelOptions{
		ReadProfile: func(string) (string, error) { return profileBody, nil },
		RunFanout: func(cfg fanoutdispatch.Config, _ io.Writer) int {
			// Materialize each worker artifact so aggregator finds them.
			data, _ := os.ReadFile(cfg.CommandsFile)
			for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
				parts := strings.SplitN(line, "\t", 2)
				if len(parts) != 2 {
					continue
				}
				workerName := parts[0]
				artifact := filepath.Join(filepath.Dir(cfg.CommandsFile), workerName+".md")
				_ = os.WriteFile(artifact, []byte("worker "+workerName+" body\n"), 0o644)
			}
			return 0
		},
		RunAggregator: func(in aggregator.Inputs, _ io.Writer) int {
			_ = os.WriteFile(in.Output, []byte("aggregated body\n"), 0o644)
			return 0
		},
		InspectCap: func(string, string) (capability.Inspection, error) {
			return capability.Inspection{
				Manifest: capability.Manifest{BudgetNative: true, PermissionScoping: true},
			}, nil
		},
		WriteFanoutLed: WriteFanoutLedgerEntry,
		WriteCache:     WriteCachePrefix,
		GitState:       func(context.Context, string) (string, string, error) { return "h", "t", nil },
		GenToken:       func() (string, error) { return "0123456789abcdef", nil },
		Now:            clock,
	}
}

const sampleScoutProfile = `{
  "role": "scout",
  "cli": "claude",
  "parallel_eligible": true,
  "output_artifact": "scout-report.md",
  "parallel_subtasks": [
    {"name":"codebase","prompt_template":"scan codebase for {cycle}"},
    {"name":"docs","prompt_template":"scan docs for {cycle}"}
  ]
}`

func TestDispatchParallel_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "ws")
	_ = os.MkdirAll(ws, 0o755)
	res, err := DispatchParallel(context.Background(), DispatchParallelRequest{
		Agent:              "scout",
		Cycle:              5,
		WorkspacePath:      ws,
		ProfilesDir:        "/p",
		AdaptersDir:        "/a",
		ProjectRoot:        tmp,
		LedgerPath:         filepath.Join(tmp, "ledger.jsonl"),
		CachePrefixEnabled: false, // skip to simplify assertions
	}, dispatchHappyOpts(t, sampleScoutProfile))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.WorkerCount != 2 {
		t.Errorf("workers=%d, want 2", res.WorkerCount)
	}
	if res.FanoutExitCode != 0 || res.AggregatorExit != 0 {
		t.Errorf("non-zero exit: %+v", res)
	}
	if res.QualityTier != "full" {
		t.Errorf("tier=%s, want full", res.QualityTier)
	}
	// Aggregate written.
	if _, err := os.Stat(res.AggregatePath); err != nil {
		t.Errorf("aggregate not written: %v", err)
	}
	// Ledger has one agent_fanout entry.
	body, _ := os.ReadFile(filepath.Join(tmp, "ledger.jsonl"))
	if !strings.Contains(string(body), `"kind":"agent_fanout"`) {
		t.Errorf("ledger missing kind: %s", body)
	}
}

func TestDispatchParallel_UnknownAgent(t *testing.T) {
	_, err := DispatchParallel(context.Background(),
		DispatchParallelRequest{Agent: "not-real", Cycle: 0, WorkspacePath: t.TempDir()},
		dispatchHappyOpts(t, sampleScoutProfile))
	if err == nil || !strings.Contains(err.Error(), "unknown agent") {
		t.Errorf("got %v", err)
	}
}

func TestDispatchParallel_NegativeCycle(t *testing.T) {
	_, err := DispatchParallel(context.Background(),
		DispatchParallelRequest{Agent: "scout", Cycle: -1, WorkspacePath: t.TempDir()},
		dispatchHappyOpts(t, sampleScoutProfile))
	if err == nil || !strings.Contains(err.Error(), "cycle must be >= 0") {
		t.Errorf("got %v", err)
	}
}

func TestDispatchParallel_MissingWorkspace(t *testing.T) {
	_, err := DispatchParallel(context.Background(),
		DispatchParallelRequest{Agent: "scout", WorkspacePath: "/no/such/dir"},
		dispatchHappyOpts(t, sampleScoutProfile))
	if err == nil || !strings.Contains(err.Error(), "workspace dir missing") {
		t.Errorf("got %v", err)
	}
}

func TestDispatchParallel_ProfileNotFound(t *testing.T) {
	opts := dispatchHappyOpts(t, "")
	opts.ReadProfile = func(string) (string, error) { return "", os.ErrNotExist }
	_, err := DispatchParallel(context.Background(),
		DispatchParallelRequest{Agent: "scout", Cycle: 0, WorkspacePath: t.TempDir()},
		opts)
	if err == nil || !strings.Contains(err.Error(), "profile not found") {
		t.Errorf("got %v", err)
	}
}

func TestDispatchParallel_NotParallelEligible(t *testing.T) {
	opts := dispatchHappyOpts(t, `{"role":"builder","parallel_eligible":false}`)
	_, err := DispatchParallel(context.Background(),
		DispatchParallelRequest{Agent: "builder", Cycle: 0, WorkspacePath: t.TempDir()},
		opts)
	if err == nil || !strings.Contains(err.Error(), "not parallel_eligible") {
		t.Errorf("got %v", err)
	}
}

func TestDispatchParallel_MissingParallelEligibleFieldRejected(t *testing.T) {
	// Bash default is false when field absent — should reject.
	opts := dispatchHappyOpts(t, `{"role":"scout","parallel_subtasks":[]}`)
	_, err := DispatchParallel(context.Background(),
		DispatchParallelRequest{Agent: "scout", Cycle: 0, WorkspacePath: t.TempDir()},
		opts)
	if err == nil || !strings.Contains(err.Error(), "not parallel_eligible") {
		t.Errorf("got %v", err)
	}
}

func TestDispatchParallel_EmptySubtasksRejected(t *testing.T) {
	opts := dispatchHappyOpts(t, `{"role":"scout","parallel_eligible":true,"parallel_subtasks":[]}`)
	_, err := DispatchParallel(context.Background(),
		DispatchParallelRequest{Agent: "scout", Cycle: 0, WorkspacePath: t.TempDir()},
		opts)
	if err == nil || !strings.Contains(err.Error(), "no parallel_subtasks") {
		t.Errorf("got %v", err)
	}
}

func TestDispatchParallel_FanoutFailureStillWritesLedger(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "ws")
	_ = os.MkdirAll(ws, 0o755)
	opts := dispatchHappyOpts(t, sampleScoutProfile)
	opts.RunFanout = func(fanoutdispatch.Config, io.Writer) int { return 1 }
	res, err := DispatchParallel(context.Background(), DispatchParallelRequest{
		Agent: "scout", Cycle: 0, WorkspacePath: ws,
		ProjectRoot: tmp, LedgerPath: filepath.Join(tmp, "ledger.jsonl"),
	}, opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.FanoutExitCode != 1 {
		t.Errorf("expected fanout rc=1, got %d", res.FanoutExitCode)
	}
	body, _ := os.ReadFile(filepath.Join(tmp, "ledger.jsonl"))
	if !strings.Contains(string(body), `"kind":"agent_fanout"`) {
		t.Errorf("ledger missing entry on failure: %s", body)
	}
	if !strings.Contains(string(body), `"exit_code":1`) {
		t.Errorf("ledger should record exit_code 1: %s", body)
	}
}

func TestDispatchParallel_AggregatorFailureRecordedNotErrored(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "ws")
	_ = os.MkdirAll(ws, 0o755)
	opts := dispatchHappyOpts(t, sampleScoutProfile)
	opts.RunAggregator = func(aggregator.Inputs, io.Writer) int { return 7 }
	res, err := DispatchParallel(context.Background(), DispatchParallelRequest{
		Agent: "scout", Cycle: 0, WorkspacePath: ws, ProjectRoot: tmp,
		LedgerPath: filepath.Join(tmp, "ledger.jsonl"),
	}, opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if res.AggregatorExit != 7 {
		t.Errorf("expected agg=7, got %d", res.AggregatorExit)
	}
}

func TestDispatchParallel_AntigravityRemappedForCapability(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "ws")
	_ = os.MkdirAll(ws, 0o755)
	body := strings.Replace(sampleScoutProfile, `"cli": "claude"`, `"cli": "antigravity"`, 1)
	opts := dispatchHappyOpts(t, body)
	var seenCLI string
	opts.InspectCap = func(_ string, cli string) (capability.Inspection, error) {
		seenCLI = cli
		return capability.Inspection{
			Manifest: capability.Manifest{BudgetNative: true, PermissionScoping: true},
		}, nil
	}
	_, err := DispatchParallel(context.Background(),
		DispatchParallelRequest{Agent: "scout", Cycle: 0, WorkspacePath: ws, ProjectRoot: tmp},
		opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if seenCLI != "agy" {
		t.Errorf("capability lookup got %q, want agy (remapped from antigravity)", seenCLI)
	}
}

func TestExtractParallelSubtasks(t *testing.T) {
	body := `{
		"parallel_subtasks": [
			{"name":"a","prompt_template":"scan {cycle}"},
			{"name":"b","prompt_template":"audit {agent}"}
		]
	}`
	got := extractParallelSubtasks(body)
	if len(got) != 2 {
		t.Fatalf("got %d, want 2", len(got))
	}
	if got[0].Name != "a" || got[0].Template != "scan {cycle}" {
		t.Errorf("first subtask malformed: %+v", got[0])
	}
	if got[1].Name != "b" {
		t.Errorf("second subtask malformed: %+v", got[1])
	}
}

func TestExtractParallelSubtasks_EmptyArray(t *testing.T) {
	got := extractParallelSubtasks(`{"parallel_subtasks":[]}`)
	if len(got) != 0 {
		t.Errorf("expected empty, got %+v", got)
	}
}

func TestExtractParallelSubtasks_NoField(t *testing.T) {
	got := extractParallelSubtasks(`{"role":"scout"}`)
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestExtractBoolField(t *testing.T) {
	tests := []struct {
		body, field string
		want        bool
	}{
		{`{"parallel_eligible":true}`, "parallel_eligible", true},
		{`{"parallel_eligible":false}`, "parallel_eligible", false},
		{`{"other":1}`, "parallel_eligible", false},
		{`{"parallel_eligible":  true }`, "parallel_eligible", true},
		{`{"parallel_eligible":"true"}`, "parallel_eligible", false}, // string not bool
	}
	for _, tc := range tests {
		if got := extractBoolField(tc.body, tc.field); got != tc.want {
			t.Errorf("body=%q got %v want %v", tc.body, got, tc.want)
		}
	}
}

func TestRenderSubtaskPrompt(t *testing.T) {
	got := renderSubtaskPrompt("cycle={cycle} agent={agent} worker={worker} ws={workspace}", 5, "scout", "codebase", "/ws")
	want := "cycle=5 agent=scout worker=codebase ws=/ws"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMergePhaseFor(t *testing.T) {
	tests := []struct{ agent, want string }{
		{"scout", "scout"},
		{"auditor", "audit"},
		{"retrospective", "learn"},
		{"random", "random"},
	}
	for _, tc := range tests {
		if got := mergePhaseFor(tc.agent); got != tc.want {
			t.Errorf("agent=%s got %s want %s", tc.agent, got, tc.want)
		}
	}
}

func TestUnescapeJSONString(t *testing.T) {
	got := unescapeJSONString(`line1\nline2\ttab\"quote\\back`)
	want := "line1\nline2\ttab\"quote\\back"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCapabilityExtractArray(t *testing.T) {
	if v, ok := capabilityExtractArray(`{"x":[1,2,3]}`, "x"); !ok || v != "1,2,3" {
		t.Errorf("got %q ok=%v", v, ok)
	}
	if _, ok := capabilityExtractArray(`{}`, "x"); ok {
		t.Errorf("expected absent")
	}
	if _, ok := capabilityExtractArray(`{"x":"str"}`, "x"); ok {
		t.Errorf("string should not match")
	}
}

func TestFillDispatchParallelDefaults(t *testing.T) {
	var opts DispatchParallelOptions
	fillDispatchParallelDefaults(&opts)
	if opts.ReadProfile == nil || opts.RunFanout == nil || opts.RunAggregator == nil ||
		opts.InspectCap == nil || opts.WriteFanoutLed == nil || opts.WriteCache == nil ||
		opts.GitState == nil || opts.GenToken == nil || opts.Now == nil {
		t.Errorf("not all defaults wired: %+v", opts)
	}
	// Verify Now is sane.
	if opts.Now().IsZero() {
		t.Errorf("Now returned zero time")
	}
	// Verify GenToken produces 16 hex chars.
	tok, err := opts.GenToken()
	if err != nil {
		t.Errorf("GenToken: %v", err)
	}
	if len(tok) != ChallengeTokenBytes*2 {
		t.Errorf("token len=%d", len(tok))
	}
	_ = time.Now() // import preserved
}
