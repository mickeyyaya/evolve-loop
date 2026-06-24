package subagent

import (
	"context"
	"errors"
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

// extractFanoutToken pulls the per-worker token the parent threaded into a
// worker command (EVOLVE_FANOUT_WORKER_TOKEN=...; shell-quoted or bare).
func extractFanoutToken(cmd string) string {
	const key = "EVOLVE_FANOUT_WORKER_TOKEN="
	i := strings.Index(cmd, key)
	if i < 0 {
		return ""
	}
	rest := cmd[i+len(key):]
	if strings.HasPrefix(rest, "'") {
		rest = rest[1:]
		if j := strings.IndexByte(rest, '\''); j >= 0 {
			return rest[:j]
		}
		return rest
	}
	if j := strings.IndexByte(rest, ' '); j >= 0 {
		return rest[:j]
	}
	return rest
}

// writeFakeWorkerArtifact mirrors a real worker: it writes an artifact bearing
// the parent-dictated token from its command line, so the parent-side
// provenance verification passes.
func writeFakeWorkerArtifact(commandsFile, line string) {
	parts := strings.SplitN(line, "\t", 2)
	if len(parts) != 2 {
		return
	}
	artifact := filepath.Join(filepath.Dir(commandsFile), parts[0]+".md")
	token := extractFanoutToken(parts[1])
	_ = os.WriteFile(artifact, []byte("<!-- challenge-token: "+token+" -->\nworker "+parts[0]+" body\n"), 0o644)
}

func dispatchHappyOpts(t *testing.T, profileBody string) DispatchParallelOptions {
	t.Helper()
	clock := fixedClock(t, "2026-05-23T17:30:00Z")
	return DispatchParallelOptions{
		ReadProfile: func(string) (string, error) { return profileBody, nil },
		RunFanout: func(cfg fanoutdispatch.Config, _ io.Writer) int {
			// Materialize each worker artifact (token-bearing) so aggregator finds them.
			data, _ := os.ReadFile(cfg.CommandsFile)
			for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
				writeFakeWorkerArtifact(cfg.CommandsFile, line)
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

func TestDispatchParallel_WorkerArtifactVerifySeamInvoked(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "ws")
	_ = os.MkdirAll(ws, 0o755)
	opts := dispatchHappyOpts(t, sampleScoutProfile)
	var verifyCount int
	opts.VerifyWorkerArtifact = func(string, string) VerifyResult {
		verifyCount++
		return VerifyResult{Verdict: VerdictPASS}
	}
	res, err := DispatchParallel(context.Background(), DispatchParallelRequest{
		Agent:              "scout",
		Cycle:              5,
		WorkspacePath:      ws,
		ProfilesDir:        "/p",
		AdaptersDir:        "/a",
		ProjectRoot:        tmp,
		LedgerPath:         filepath.Join(tmp, "ledger.jsonl"),
		CachePrefixEnabled: false,
	}, opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if verifyCount != res.WorkerCount {
		t.Fatalf("VerifyWorkerArtifact calls=%d, want %d", verifyCount, res.WorkerCount)
	}
	if len(res.WorkerVerifyFailures) != 0 {
		t.Fatalf("unexpected verify failures: %v", res.WorkerVerifyFailures)
	}
}

func TestDispatchParallel_WorkerArtifactMissingSkipsAggregation(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "ws")
	_ = os.MkdirAll(ws, 0o755)
	opts := dispatchHappyOpts(t, sampleScoutProfile)
	opts.RunFanout = func(cfg fanoutdispatch.Config, _ io.Writer) int {
		data, _ := os.ReadFile(cfg.CommandsFile)
		lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
		for i, line := range lines {
			if i == len(lines)-1 {
				continue // deliberately leave the last worker's artifact missing
			}
			writeFakeWorkerArtifact(cfg.CommandsFile, line)
		}
		return 0
	}
	aggregatorCalled := false
	opts.RunAggregator = func(aggregator.Inputs, io.Writer) int {
		aggregatorCalled = true
		return 0
	}
	res, err := DispatchParallel(context.Background(), DispatchParallelRequest{
		Agent:              "scout",
		Cycle:              5,
		WorkspacePath:      ws,
		ProfilesDir:        "/p",
		AdaptersDir:        "/a",
		ProjectRoot:        tmp,
		LedgerPath:         filepath.Join(tmp, "ledger.jsonl"),
		CachePrefixEnabled: false,
	}, opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(res.WorkerVerifyFailures) != 1 {
		t.Fatalf("WorkerVerifyFailures=%v, want one missing artifact", res.WorkerVerifyFailures)
	}
	if aggregatorCalled {
		t.Fatalf("aggregator should be skipped when worker verification fails")
	}
	if res.AggregatorExit != aggregator.ExitUsageErr {
		t.Fatalf("AggregatorExit=%d, want %d", res.AggregatorExit, aggregator.ExitUsageErr)
	}
}

// TestDispatchParallel_ThreadsRecursionDepth proves the worker commands written
// for the LLM (non-test-executor) path recurse via `subagent run` AND carry an
// incremented EVOLVE_DISPATCH_DEPTH — so the cap in Run() bounds nesting.
func TestDispatchParallel_ThreadsRecursionDepth(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "ws")
	_ = os.MkdirAll(ws, 0o755)

	opts := dispatchHappyOpts(t, sampleScoutProfile)
	var commandsBody string
	inner := opts.RunFanout
	opts.RunFanout = func(cfg fanoutdispatch.Config, w io.Writer) int {
		data, _ := os.ReadFile(cfg.CommandsFile)
		commandsBody = string(data)
		return inner(cfg, w)
	}

	_, err := DispatchParallel(context.Background(), DispatchParallelRequest{
		Agent:              "scout",
		Cycle:              5,
		WorkspacePath:      ws,
		ProjectRoot:        tmp,
		LedgerPath:         filepath.Join(tmp, "ledger.jsonl"),
		CachePrefixEnabled: false,
		DispatchDepth:      1, // workers must run at depth 2
	}, opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !strings.Contains(commandsBody, "subagent run scout-worker-codebase 5 '"+ws+"'") {
		t.Errorf("worker command does not recurse via subagent run (quoted ws):\n%s", commandsBody)
	}
	if !strings.Contains(commandsBody, "EVOLVE_DISPATCH_DEPTH=2") {
		t.Errorf("worker command missing incremented recursion depth:\n%s", commandsBody)
	}
}

// TestDispatchParallel_RecursionDepthCap proves a fan-out is refused fast at the
// boundary: at parentDepth==cap the workers would run at cap+1, so DispatchParallel
// rejects before spawning doomed workers (the child-fence, not just self-depth).
func TestDispatchParallel_RecursionDepthCap(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "ws")
	_ = os.MkdirAll(ws, 0o755)
	_, err := DispatchParallel(context.Background(), DispatchParallelRequest{
		Agent:         "scout",
		Cycle:         5,
		WorkspacePath: ws,
		ProjectRoot:   tmp,
		DispatchDepth: maxDispatchDepth, // workers would be cap+1 → refuse fast
	}, dispatchHappyOpts(t, sampleScoutProfile))
	if !errors.Is(err, ErrRecursionDepthExceeded) {
		t.Fatalf("expected ErrRecursionDepthExceeded at parentDepth==cap, got %v", err)
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

// TestDispatchParallel_CachePrefixEnabledWritesPrefix covers
// dispatchparallel.go:129-139 — the cache-prefix branch invokes the
// WriteCache seam and passes the resulting path to fanout.
func TestDispatchParallel_CachePrefixEnabledWritesPrefix(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "ws")
	_ = os.MkdirAll(ws, 0o755)
	opts := dispatchHappyOpts(t, sampleScoutProfile)
	var cacheReq CachePrefixRequest
	var cacheCalled bool
	opts.WriteCache = func(req CachePrefixRequest, _ CachePrefixOptions) error {
		cacheCalled = true
		cacheReq = req
		return os.WriteFile(req.OutPath, []byte("prefix\n"), 0o644)
	}
	var fanoutCacheFile string
	innerFanout := opts.RunFanout
	opts.RunFanout = func(cfg fanoutdispatch.Config, w io.Writer) int {
		fanoutCacheFile = cfg.CachePrefixFile
		return innerFanout(cfg, w)
	}
	_, err := DispatchParallel(context.Background(), DispatchParallelRequest{
		Agent: "scout", Cycle: 4, WorkspacePath: ws, ProjectRoot: tmp,
		CachePrefixEnabled: true,
	}, opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !cacheCalled {
		t.Fatalf("WriteCache seam not invoked")
	}
	wantPath := filepath.Join(ws, "workers", "cache-prefix.md")
	if cacheReq.OutPath != wantPath {
		t.Errorf("cache OutPath=%q, want %q", cacheReq.OutPath, wantPath)
	}
	if fanoutCacheFile != wantPath {
		t.Errorf("fanout CachePrefixFile=%q, want %q", fanoutCacheFile, wantPath)
	}
}

// TestDispatchParallel_CachePrefixErrorAborts covers dispatchparallel.go:138.
func TestDispatchParallel_CachePrefixErrorAborts(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "ws")
	_ = os.MkdirAll(ws, 0o755)
	opts := dispatchHappyOpts(t, sampleScoutProfile)
	opts.WriteCache = func(CachePrefixRequest, CachePrefixOptions) error {
		return errors.New("cache write failed")
	}
	_, err := DispatchParallel(context.Background(), DispatchParallelRequest{
		Agent: "scout", Cycle: 0, WorkspacePath: ws, ProjectRoot: tmp,
		CachePrefixEnabled: true,
	}, opts)
	if err == nil || !strings.Contains(err.Error(), "cache prefix") {
		t.Errorf("got %v", err)
	}
}

// TestDispatchParallel_DefaultCLIAndAggPathWhenProfileSilent covers
// dispatchparallel.go:109-111 (cli default "claude") and 147-149 (aggregate
// path default to <workspace>/<agent>-report.md when output_artifact absent).
func TestDispatchParallel_DefaultCLIAndAggPathWhenProfileSilent(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "ws")
	_ = os.MkdirAll(ws, 0o755)
	// Profile omits "cli" and "output_artifact".
	profile := `{"role":"scout","parallel_eligible":true,"parallel_subtasks":[{"name":"codebase","prompt_template":"scan {cycle}"}]}`
	opts := dispatchHappyOpts(t, profile)
	var inspectedCLI string
	opts.InspectCap = func(_, cli string) (capability.Inspection, error) {
		inspectedCLI = cli
		return capability.Inspection{Manifest: capability.Manifest{BudgetNative: true, PermissionScoping: true}}, nil
	}
	res, err := DispatchParallel(context.Background(), DispatchParallelRequest{
		Agent: "scout", Cycle: 0, WorkspacePath: ws, ProjectRoot: tmp,
	}, opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if inspectedCLI != "claude" {
		t.Errorf("cli default=%q, want claude", inspectedCLI)
	}
	wantAgg := filepath.Join(ws, "scout-report.md")
	if res.AggregatePath != wantAgg {
		t.Errorf("aggregate path=%q, want %q (default)", res.AggregatePath, wantAgg)
	}
}

// TestDispatchParallel_GenTokenErrorAborts covers dispatchparallel.go:155-157.
func TestDispatchParallel_GenTokenErrorAborts(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "ws")
	_ = os.MkdirAll(ws, 0o755)
	opts := dispatchHappyOpts(t, sampleScoutProfile)
	opts.GenToken = func() (string, error) { return "", errors.New("rand exhausted") }
	_, err := DispatchParallel(context.Background(), DispatchParallelRequest{
		Agent: "scout", Cycle: 0, WorkspacePath: ws, ProjectRoot: tmp,
	}, opts)
	if err == nil || !strings.Contains(err.Error(), "parent token") {
		t.Errorf("got %v", err)
	}
}

// TestDispatchParallel_EmptyGitStateNormalizedToUnknown covers
// dispatchparallel.go:159-164 — empty git head/diff become "unknown" in the
// parent ledger entry.
func TestDispatchParallel_EmptyGitStateNormalizedToUnknown(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "ws")
	_ = os.MkdirAll(ws, 0o755)
	opts := dispatchHappyOpts(t, sampleScoutProfile)
	opts.GitState = func(context.Context, string) (string, string, error) { return "", "", nil }
	ledger := filepath.Join(tmp, "ledger.jsonl")
	_, err := DispatchParallel(context.Background(), DispatchParallelRequest{
		Agent: "scout", Cycle: 0, WorkspacePath: ws, ProjectRoot: tmp,
		LedgerPath: ledger,
	}, opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	body, _ := os.ReadFile(ledger)
	if !strings.Contains(string(body), `"git_head":"unknown"`) {
		t.Errorf("empty git head not normalized: %s", body)
	}
	if !strings.Contains(string(body), `"tree_state_sha":"unknown"`) {
		t.Errorf("empty tree diff not normalized: %s", body)
	}
}

// TestDispatchParallel_TestExecutorBranchBuildsBashCommand covers
// dispatchparallel.go:190-198 — when TestExecutor is set, the worker command
// shells the test executor with EVOLVE_FANOUT_* env instead of recursing into
// `evolve subagent run`.
func TestDispatchParallel_TestExecutorBranchBuildsBashCommand(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "ws")
	_ = os.MkdirAll(ws, 0o755)
	opts := dispatchHappyOpts(t, sampleScoutProfile)
	var commandsContents string
	innerFanout := opts.RunFanout
	opts.RunFanout = func(cfg fanoutdispatch.Config, w io.Writer) int {
		data, _ := os.ReadFile(cfg.CommandsFile)
		commandsContents = string(data)
		return innerFanout(cfg, w)
	}
	_, err := DispatchParallel(context.Background(), DispatchParallelRequest{
		Agent: "scout", Cycle: 2, WorkspacePath: ws, ProjectRoot: tmp,
		TestExecutor: "/path/to/exec.sh",
	}, opts)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !strings.Contains(commandsContents, "EVOLVE_FANOUT_WORKER_NAME=codebase") {
		t.Errorf("test-executor env not built: %s", commandsContents)
	}
	if !strings.Contains(commandsContents, "bash /path/to/exec.sh") {
		t.Errorf("test-executor command not shelled: %s", commandsContents)
	}
	// The non-test-executor path (evolve subagent run) must NOT be present.
	if strings.Contains(commandsContents, "subagent run scout-codebase") {
		t.Errorf("should use test executor, not recursion: %s", commandsContents)
	}
}

// TestDispatchParallel_LedgerWriteErrorPropagates covers
// dispatchparallel.go:266-268.
func TestDispatchParallel_LedgerWriteErrorPropagates(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "ws")
	_ = os.MkdirAll(ws, 0o755)
	opts := dispatchHappyOpts(t, sampleScoutProfile)
	opts.WriteFanoutLed = func(string, FanoutLedgerEntry, func() time.Time) error {
		return errors.New("ledger disk full")
	}
	_, err := DispatchParallel(context.Background(), DispatchParallelRequest{
		Agent: "scout", Cycle: 0, WorkspacePath: ws, ProjectRoot: tmp,
		LedgerPath: filepath.Join(tmp, "ledger.jsonl"),
	}, opts)
	if err == nil || !strings.Contains(err.Error(), "ledger write") {
		t.Errorf("got %v", err)
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

func TestMatchParallelEligibleField(t *testing.T) {
	tests := []struct {
		body string
		want string
	}{
		{`{"parallel_eligible":true}`, "true"},
		{`{"parallel_eligible":false}`, "false"},
		{`{"other":1}`, ""},
		{`{"parallel_eligible":  true }`, "true"},
		{`{"parallel_eligible":"true"}`, ""}, // string not bool
	}
	for _, tc := range tests {
		if got := matchField(tc.body, reFieldParallelEligible); got != tc.want {
			t.Errorf("body=%q got %q want %q", tc.body, got, tc.want)
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

// TestCapabilityExtractArray_KeyWithoutColon covers dispatchparallel.go:343 —
// the key is present but not followed by ':'.
func TestCapabilityExtractArray_KeyWithoutColon(t *testing.T) {
	if v, ok := capabilityExtractArray(`{"x" [1,2]}`, "x"); ok {
		t.Errorf("key without colon should not match, got %q", v)
	}
}

// TestCapabilityExtractArray_Unterminated covers dispatchparallel.go:362 —
// an opening bracket that is never balanced falls through to (",false").
func TestCapabilityExtractArray_Unterminated(t *testing.T) {
	if v, ok := capabilityExtractArray(`{"x":[1,2`, "x"); ok {
		t.Errorf("unterminated array should not match, got %q", v)
	}
}

// TestFirstSubmatch covers dispatchparallel.go:317-322 including the no-match
// branch (len(m) < 2 → "").
func TestFirstSubmatch(t *testing.T) {
	if got := firstSubmatch(subtaskNameRE, `{"name":"codebase"}`); got != "codebase" {
		t.Errorf("match: got %q, want codebase", got)
	}
	if got := firstSubmatch(subtaskNameRE, `{"other":"x"}`); got != "" {
		t.Errorf("no-match should be empty, got %q", got)
	}
}

// TestExtractParallelSubtasks_SubtaskWithoutTemplate exercises firstSubmatch's
// empty-return path through the public parser: a subtask object with a name
// but no prompt_template yields an empty Template (not dropped).
func TestExtractParallelSubtasks_SubtaskWithoutTemplate(t *testing.T) {
	got := extractParallelSubtasks(`{"parallel_subtasks":[{"name":"codebase"}]}`)
	if len(got) != 1 {
		t.Fatalf("got %d subtasks, want 1", len(got))
	}
	if got[0].Name != "codebase" || got[0].Template != "" {
		t.Errorf("subtask=%+v, want {codebase, empty template}", got[0])
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
