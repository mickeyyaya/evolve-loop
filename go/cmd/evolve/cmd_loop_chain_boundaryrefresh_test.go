package main

// cmd_loop_chain_boundaryrefresh_test.go — RED tests (cycle 1314, inbox item
// auto-refresh-binary-at-boundary, task boundary-binary-refresh).
//
// Defect: runLoopChain (cmd_loop_chain.go) relaunches runLoopBatchFn at every
// boundary but never checks whether the RUNNING binary (version.Commit()) has
// fallen behind HEAD — fixes that land on main mid-chain (e.g. the sentinel
// tail-anchor fix at cycle-1301 64f8620e) sit inert until an operator manually
// rebuilds + `evolve reset-sha -operator` + relaunches. Cycles 1302-1309 kept
// running the old parser on a stale binary.
//
// Contract the Builder implements (TDD-defined seams; mirrors the established
// bootRecoverFn / shipRepinProvenanceFn package-var seam idiom from
// cmd_loop_boot_recovery.go):
//
//	// chainBoundaryAheadFn reports whether HEAD carries commits beyond
//	// runningCommit — the "is my binary stale" check. Reuses the EXACT
//	// ancestor-check idiom runResetSHA already uses (`git merge-base
//	// --is-ancestor <commit> HEAD`), just inverted: ahead=true means
//	// runningCommit is a STRICT ancestor of HEAD (HEAD has moved past it).
//	// An empty runningCommit (unstamped dev binary — nothing to compare) is a
//	// no-op: ahead=false, err=nil, no git subprocess. Any git/network/repo
//	// failure returns err!=nil — the caller MUST treat that as "skip this
//	// boundary's refresh", never halt the chain (AC4).
//	var chainBoundaryAheadFn = defaultChainBoundaryAhead
//	func defaultChainBoundaryAhead(projectRoot, runningCommit string) (ahead bool, err error)
//
//	// chainRebuildFn runs the sanctioned rebuild recipe (`make -C go build`,
//	// runtime-reference.md:170) so the on-disk binary catches up to HEAD.
//	var chainRebuildFn = defaultChainRebuild
//	func defaultChainRebuild(projectRoot string) error
//
//	// chainReExecArgvFn resolves the argv to re-exec — deliberately just
//	// os.Args, so runLoopChain's SIGNATURE (and every prior call site / frozen
//	// test) stays byte-identical; only a new package var is added.
//	var chainReExecArgvFn = func() []string { return os.Args }
//
//	// chainReExecFn is the seam over syscall.Exec so tests can assert the
//	// re-exec was invoked with the right argv without replacing the test
//	// process. Returns an error only if the exec syscall itself fails to
//	// launch (e.g. argv0 not executable) — in production a SUCCESSFUL
//	// syscall.Exec never returns at all.
//	var chainReExecFn = defaultChainReExec
//	func defaultChainReExec(argv0 string, argv, envv []string) error
//
//	// maybeRefreshChainBoundary runs the ahead-check -> rebuild -> repin ->
//	// audit-log -> re-exec sequence for boundary `batch`. It is called ONLY
//	// from runLoopChain between chainStartDecision deciding to continue and
//	// runLoopBatchFn — the loop body is single-threaded per boundary, so this
//	// call-site placement is what satisfies "refuse entirely while a batch is
//	// mid-flight" (AC2); no separate lock is needed. Every failure at every
//	// stage (ahead-check error, rebuild error, repin error, re-exec error)
//	// degrades to refreshed=false, logged to stderr, and the CURRENT binary
//	// keeps running the chain — never halts (AC4). The repin reuses
//	// phaseintegrity.RepinShipSHA UNCHANGED (protected surface,
//	// go/internal/phaseintegrity/ — this cycle does not touch it): the
//	// provenance closure simply re-asserts the freshly-rebuilt commit is HEAD,
//	// which is trivially true, so RepinShipSHA stamps its own
//	// Authorized="provenance" as today. The DISTINGUISHABLE "boundary-refresh"
//	// authorization class the inbox item asks for is a SEPARATE, additive audit
//	// record — chainBoundaryRefreshLogFile (a JSONL file under evolveDir,
//	// .evolve/boundary-refresh-log.jsonl) — so a ledger read can tell an
//	// automatic boundary repin apart from a manual `evolve reset-sha`
//	// (RepinShipSHA's own two-value Authorized enum is unchanged; the THIRD
//	// class lives one layer up, in the chain-owned log).
//	var chainBoundaryRefreshLogFile = "boundary-refresh-log.jsonl"
//	func maybeRefreshChainBoundary(cfg loopConfig, batch int, stderr io.Writer) (refreshed bool)
//
// Wiring: runLoopChain calls maybeRefreshChainBoundary(cfg, n+1, stderr)
// immediately after chainStartDecision's continue branch and before the
// existing `width := loadFleetConfig(...)` line; refreshed==true breaks the
// loop with res.StopReason = "chain_boundary_refresh_reexec", exit 0 (the new
// process, once re-exec'd, resumes the chain from scratch under the same
// args). runLoopChain's own SIGNATURE is UNCHANGED — every existing frozen
// call site (cmd_loop.go:143, cmd_loop_chain_test.go and siblings) keeps
// compiling untouched.
//
// RED now: chainBoundaryAheadFn / chainRebuildFn / chainReExecArgvFn /
// chainReExecFn / maybeRefreshChainBoundary are all undefined -> this
// package's test build fails. Do NOT modify this file — implement the seams
// in cmd_loop_chain.go.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// --- git fixture helpers (brf* prefix avoids collision with other main tests) ---

func brfGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = brfFilteredEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v (dir=%s): %v\n%s", args, dir, err, out)
	}
	return string(out)
}

func brfFilteredEnv() []string {
	var out []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "EVOLVE_") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// brfInitRepo creates a one-commit git repo and returns (repoDir, commitA).
func brfInitRepo(t *testing.T) (dir, commitA string) {
	t.Helper()
	dir = t.TempDir()
	brfGit(t, dir, "init", "-q")
	brfGit(t, dir, "config", "user.email", "ci@example.com")
	brfGit(t, dir, "config", "user.name", "ci")
	brfGit(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	brfGit(t, dir, "-C", dir, "add", "a.txt")
	brfGit(t, dir, "-C", dir, "commit", "-q", "-m", "commit A")
	commitA = strings.TrimSpace(brfGit(t, dir, "-C", dir, "rev-parse", "HEAD"))
	return dir, commitA
}

// brfAdvance commits one more file on top and returns the new HEAD.
func brfAdvance(t *testing.T, dir string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	brfGit(t, dir, "-C", dir, "add", "b.txt")
	brfGit(t, dir, "-C", dir, "commit", "-q", "-m", "commit B")
	return strings.TrimSpace(brfGit(t, dir, "-C", dir, "rev-parse", "HEAD"))
}

func brfWriteJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// --- AC1: ahead-check detection, reusing the ancestor-check idiom ---

// AC1 positive: HEAD has moved past the running binary's build commit.
func TestDefaultChainBoundaryAhead_DetectsRunningCommitBehindHead(t *testing.T) {
	dir, commitA := brfInitRepo(t)
	brfAdvance(t, dir)
	ahead, err := defaultChainBoundaryAhead(dir, commitA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ahead {
		t.Fatal("expected ahead=true — HEAD carries a commit the running binary was not built from")
	}
}

// AC1 negative: the running binary IS the tip — nothing to refresh.
func TestDefaultChainBoundaryAhead_NoLagWhenRunningCommitIsHead(t *testing.T) {
	dir, commitA := brfInitRepo(t)
	ahead, err := defaultChainBoundaryAhead(dir, commitA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ahead {
		t.Fatal("running commit == HEAD must NOT report ahead")
	}
}

// AC1 edge: an unstamped dev binary (empty build commit) has nothing
// verifiable to compare — must be a quiet no-op, not an error.
func TestDefaultChainBoundaryAhead_EmptyRunningCommitIsNoOp(t *testing.T) {
	dir, _ := brfInitRepo(t)
	ahead, err := defaultChainBoundaryAhead(dir, "")
	if err != nil {
		t.Fatalf("empty running commit must be a no-op, not an error: %v", err)
	}
	if ahead {
		t.Fatal("empty running commit must never report ahead=true")
	}
}

// AC4: a git failure (not a repo at all) must return an error the caller
// treats as "skip this boundary" — never panic, never claim ahead=true on
// bad data.
func TestDefaultChainBoundaryAhead_GitFailureDegradesToSkip(t *testing.T) {
	notARepo := t.TempDir()
	_, err := defaultChainBoundaryAhead(notARepo, "deadbeef")
	if err == nil {
		t.Fatal("expected an error against a non-git directory")
	}
}

// --- maybeRefreshChainBoundary: the orchestrated sequence ---

// AC6c: no lag detected -> the whole rebuild/repin/re-exec sequence is a
// provably free no-op — none of the seams fire.
func TestMaybeRefreshChainBoundary_NoLagIsNoOpFree(t *testing.T) {
	restore := brfStubSeams(t, false, nil, nil)
	defer restore()

	rebuildCalled, reexecCalled := false, false
	prevRebuild, prevReExec := chainRebuildFn, chainReExecFn
	defer func() { chainRebuildFn, chainReExecFn = prevRebuild, prevReExec }()
	chainRebuildFn = func(string) error { rebuildCalled = true; return nil }
	chainReExecFn = func(string, []string, []string) error { reexecCalled = true; return nil }

	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	refreshed := maybeRefreshChainBoundary(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, 1, &stderr)

	if refreshed {
		t.Error("no lag detected must not report refreshed=true")
	}
	if rebuildCalled || reexecCalled {
		t.Errorf("no lag detected must call NEITHER rebuild NOR re-exec (rebuild=%v reexec=%v)", rebuildCalled, reexecCalled)
	}
}

// AC3 + AC6b: lag detected -> rebuild, repin (ledgered under a distinguishable
// "boundary-refresh" class), and re-exec, in that order.
func TestMaybeRefreshChainBoundary_LagTriggersRebuildRepinReExecAndLedger(t *testing.T) {
	// Cycle-1323 fixture update (contract unchanged, setup completed): the
	// re-pin now hashes the REBUILT <root>/go/bin/evolve and the provenance gate
	// is real, so the fixture must supply both. Every assertion below is the
	// cycle-1320 assertion, verbatim.
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")

	restore := brfStubSeams(t, true, nil, nil)
	defer restore()

	prevCommit, prevProv := chainRunningCommitFn, chainBoundaryRepinProvenanceFn
	defer func() { chainRunningCommitFn, chainBoundaryRepinProvenanceFn = prevCommit, prevProv }()
	chainRunningCommitFn = func() string { return "cafebabe1234" }
	chainBoundaryRepinProvenanceFn = func(string) (string, phaseintegrity.ProvenanceVerified) {
		return "cafebabe1234", func(c string) bool { return c == "cafebabe1234" }
	}

	var order []string
	prevRebuild, prevReExec := chainRebuildFn, chainReExecFn
	defer func() { chainRebuildFn, chainReExecFn = prevRebuild, prevReExec }()
	chainRebuildFn = func(string) error { order = append(order, "rebuild"); return nil }
	var reexecArgv []string
	chainReExecFn = func(argv0 string, argv, envv []string) error {
		order = append(order, "reexec")
		reexecArgv = argv
		return nil
	}

	var stderr bytes.Buffer
	refreshed := maybeRefreshChainBoundary(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, 7, &stderr)

	if !refreshed {
		t.Fatalf("lag detected must report refreshed=true; stderr=%s", stderr.String())
	}
	if len(order) != 2 || order[0] != "rebuild" || order[1] != "reexec" {
		t.Fatalf("expected rebuild-then-reexec ordering, got %v", order)
	}
	if len(reexecArgv) == 0 {
		t.Error("re-exec must be invoked with a non-empty argv (the original chain invocation)")
	}

	// The state.json pin moved (RepinShipSHA's own write path — unchanged,
	// protected surface not touched this cycle).
	raw, err := os.ReadFile(filepath.Join(evolveDir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "STALE_PIN") {
		t.Errorf("expected_ship_sha must be re-pinned on a successful boundary refresh: %s", raw)
	}

	// AC3's "auditable... (NOT silent)" requirement: a DISTINGUISHABLE
	// boundary-refresh record, separate from state.json's own two-value
	// Authorized enum, so a ledger read can tell this apart from a manual
	// `evolve reset-sha`.
	logRaw, err := os.ReadFile(filepath.Join(evolveDir, chainBoundaryRefreshLogFile))
	if err != nil {
		t.Fatalf("boundary-refresh must be ledgered (missing %s): %v", chainBoundaryRefreshLogFile, err)
	}
	if !strings.Contains(string(logRaw), "boundary-refresh") {
		t.Errorf("boundary-refresh ledger entry must carry a distinguishable authorization class: %s", logRaw)
	}
	if !strings.Contains(string(logRaw), `"batch":7`) && !strings.Contains(string(logRaw), `"batch": 7`) {
		t.Errorf("boundary-refresh ledger entry must record which boundary triggered it: %s", logRaw)
	}
}

// AC4: a rebuild failure must degrade to refreshed=false — repin and re-exec
// are never reached, and the chain keeps running the CURRENT (un-rebuilt)
// binary rather than halting.
func TestMaybeRefreshChainBoundary_RebuildFailureDegradesToNoRefresh(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	brfWriteJSON(t, filepath.Join(evolveDir, "state.json"),
		map[string]any{"expected_ship_sha": "STALE_PIN"})

	restore := brfStubSeams(t, true, nil, nil)
	defer restore()

	reexecCalled := false
	prevRebuild, prevReExec := chainRebuildFn, chainReExecFn
	defer func() { chainRebuildFn, chainReExecFn = prevRebuild, prevReExec }()
	chainRebuildFn = func(string) error { return errors.New("build failed: syntax error") }
	chainReExecFn = func(string, []string, []string) error { reexecCalled = true; return nil }

	var stderr bytes.Buffer
	refreshed := maybeRefreshChainBoundary(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, 1, &stderr)

	if refreshed {
		t.Error("a rebuild failure must never report refreshed=true")
	}
	if reexecCalled {
		t.Error("a rebuild failure must never reach re-exec")
	}
	raw, _ := os.ReadFile(filepath.Join(evolveDir, "state.json"))
	if !strings.Contains(string(raw), "STALE_PIN") {
		t.Errorf("a rebuild failure must leave the pin UNTOUCHED: %s", raw)
	}
	if stderr.Len() == 0 {
		t.Error("a rebuild failure must be logged (auditable degrade), not swallowed silently")
	}
}

// AC4: an ahead-check error (git/network failure) must degrade to
// refreshed=false without ever touching rebuild/repin/re-exec.
func TestMaybeRefreshChainBoundary_AheadCheckErrorDegradesToNoRefresh(t *testing.T) {
	restore := brfStubSeams(t, false, errors.New("git fetch: network unreachable"), nil)
	defer restore()

	rebuildCalled := false
	prevRebuild := chainRebuildFn
	defer func() { chainRebuildFn = prevRebuild }()
	chainRebuildFn = func(string) error { rebuildCalled = true; return nil }

	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	refreshed := maybeRefreshChainBoundary(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, 1, &stderr)

	if refreshed {
		t.Error("an ahead-check error must never report refreshed=true")
	}
	if rebuildCalled {
		t.Error("an ahead-check error must short-circuit before rebuild")
	}
}

// brfStubSeams installs a chainBoundaryAheadFn stub returning (ahead, err) for
// every call, and returns a restore func. aheadErr/unused params reserved for
// future callers that need per-call variation; nil means constant per-call.
func brfStubSeams(t *testing.T, ahead bool, aheadErr error, _ any) func() {
	t.Helper()
	prev := chainBoundaryAheadFn
	chainBoundaryAheadFn = func(string, string) (bool, error) { return ahead, aheadErr }
	return func() { chainBoundaryAheadFn = prev }
}

// --- AC2 + AC6a: boundary refresh is checked ONLY between chainStartDecision
// and runLoopBatchFn, at every boundary, never mid-batch ---

// AC2/AC6a: over a 3-batch chain, the ahead-check must fire exactly once per
// boundary, strictly BEFORE that boundary's runLoopBatchFn call — proving the
// refresh can never interleave with (or interrupt) an in-flight batch, which
// is single-threaded by construction at this call site.
func TestRunLoopChain_BoundaryRefreshCheckedBeforeEveryBatchNeverMidBatch(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Seed >=3 pending inbox items so chain_inbox_empty (an n>0-gated CONTINUE
	// condition, cmd_loop_chain.go chainStartDecision) never fires before the
	// 3rd boundary's [ahead-check, batch] pair is recorded — an empty inbox
	// would stop the chain after batch 1, making the calls>=3 branch in the
	// runLoopBatchFn stub below unreachable (cycle-1314/1315 fixture bug).
	inboxDir := filepath.Join(evolveDir, "inbox")
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		brfWriteJSON(t, filepath.Join(inboxDir, fmt.Sprintf("item-%d.json", i)),
			map[string]any{"id": fmt.Sprintf("item-%d", i)})
	}

	var order []string
	prevAhead := chainBoundaryAheadFn
	defer func() { chainBoundaryAheadFn = prevAhead }()
	chainBoundaryAheadFn = func(string, string) (bool, error) {
		order = append(order, "ahead-check")
		return false, nil // never trigger a real refresh — isolates the ORDERING assertion
	}

	prevBatch := runLoopBatchFn
	defer func() { runLoopBatchFn = prevBatch }()
	calls := 0
	runLoopBatchFn = func(loopConfig, io.Reader, io.Writer, io.Writer) int {
		calls++
		order = append(order, "batch")
		if calls >= 3 {
			return 5 // quota defer — stop the chain
		}
		return 0
	}

	cfg := loopConfig{ProjectRoot: root, EvolveDir: evolveDir}
	var stdout, stderr bytes.Buffer
	_ = runLoopChain(cfg, policy.ChainConfig{Enabled: true, MaxBatches: 10}, nil, &stdout, &stderr)

	if len(order) < 6 {
		t.Fatalf("expected an ahead-check immediately before each of >=3 batches, got sequence %v", order)
	}
	for i := 0; i+1 < len(order); i += 2 {
		if order[i] != "ahead-check" || order[i+1] != "batch" {
			t.Fatalf("expected strict [ahead-check, batch] pairs at every boundary, got %v", order)
		}
	}
}

// AC6b end-to-end: when the ahead-check trips at a given boundary, the chain
// stops (re-exec is terminal in production) BEFORE that boundary's
// runLoopBatchFn runs, and the stop reason names the refresh.
func TestRunLoopChain_BoundaryRefreshStopsChainBeforeThatBoundarysBatch(t *testing.T) {
	// Cycle-1323 fixture update (see the sibling test): a rebuilt binary to
	// re-exec into and a real provenance seam. Assertions are unchanged.
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")

	prevCommit, prevProv := chainRunningCommitFn, chainBoundaryRepinProvenanceFn
	defer func() { chainRunningCommitFn, chainBoundaryRepinProvenanceFn = prevCommit, prevProv }()
	chainRunningCommitFn = func() string { return "cafebabe1234" }
	chainBoundaryRepinProvenanceFn = func(string) (string, phaseintegrity.ProvenanceVerified) {
		return "cafebabe1234", func(c string) bool { return c == "cafebabe1234" }
	}

	callN := 0
	prevAhead := chainBoundaryAheadFn
	defer func() { chainBoundaryAheadFn = prevAhead }()
	chainBoundaryAheadFn = func(string, string) (bool, error) {
		callN++
		return callN == 2, nil // trip on the SECOND boundary only
	}
	prevRebuild, prevReExec := chainRebuildFn, chainReExecFn
	defer func() { chainRebuildFn, chainReExecFn = prevRebuild, prevReExec }()
	chainRebuildFn = func(string) error { return nil }
	reexecCalled := false
	chainReExecFn = func(string, []string, []string) error { reexecCalled = true; return nil }

	batchCalls := 0
	prevBatch := runLoopBatchFn
	defer func() { runLoopBatchFn = prevBatch }()
	runLoopBatchFn = func(loopConfig, io.Reader, io.Writer, io.Writer) int {
		batchCalls++
		return 0
	}

	cfg := loopConfig{ProjectRoot: root, EvolveDir: evolveDir}
	var stdout, stderr bytes.Buffer
	_ = runLoopChain(cfg, policy.ChainConfig{Enabled: true, MaxBatches: 10}, nil, &stdout, &stderr)

	if !reexecCalled {
		t.Fatalf("expected the second boundary's ahead-check trip to trigger a re-exec; stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	if batchCalls != 1 {
		t.Errorf("the boundary that trips the refresh must run ZERO batches of its own (re-exec is terminal); got %d batch calls before stop", batchCalls)
	}
	if !strings.Contains(stdout.String(), "chain_boundary_refresh") {
		t.Errorf("chain summary must name the boundary-refresh stop reason: %s", stdout.String())
	}
}
