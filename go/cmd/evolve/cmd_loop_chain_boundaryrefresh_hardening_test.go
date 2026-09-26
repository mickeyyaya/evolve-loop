package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// brhProject builds a temp project root with .evolve/state.json pinned to
// `pin` and a go/bin/evolve carrying `binContent`, and returns
// (root, evolveDir, sha256(binContent)).
func brhProject(t *testing.T, pin, binContent string) (root, evolveDir, binSHA string) {
	t.Helper()
	root = t.TempDir()
	evolveDir = filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	brfWriteJSON(t, filepath.Join(evolveDir, "state.json"),
		map[string]any{"expected_ship_sha": pin})

	binDir := filepath.Join(root, "go", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "evolve"), []byte(binContent), 0o755); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(binContent))
	return root, evolveDir, hex.EncodeToString(sum[:])
}

// brhReadPin returns .evolve/state.json:expected_ship_sha.
func brhReadPin(t *testing.T, evolveDir string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(evolveDir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var st map[string]any
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatalf("state.json is not parseable JSON: %v\n%s", err, raw)
	}
	pin, _ := st["expected_ship_sha"].(string)
	return pin
}

// TestMaybeRefreshChainBoundary_UnverifiedProvenanceRefusesRepinAndReExec is
// the load-bearing anti-forgery assertion: when provenance says "I cannot
// verify this build commit", the boundary refresh must refuse — the ship pin
// stays untouched, nothing is ledgered as authorized, no re-exec happens, and
// the chain degrades to running on the current binary.
func TestMaybeRefreshChainBoundary_UnverifiedProvenanceRefusesRepinAndReExec(t *testing.T) {
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")

	restore := brfStubSeams(t, true, nil, nil)
	defer restore()

	prevRebuild, prevReExec := chainRebuildFn, chainReExecFn
	prevCommit, prevProv := chainRunningCommitFn, chainBoundaryRepinProvenanceFn
	defer func() {
		chainRebuildFn, chainReExecFn = prevRebuild, prevReExec
		chainRunningCommitFn, chainBoundaryRepinProvenanceFn = prevCommit, prevProv
	}()

	chainRunningCommitFn = func() string { return "cafebabe1234" }
	chainRebuildFn = func(string) error { return nil }
	reexecCalled := false
	chainReExecFn = func(string, []string, []string) error { reexecCalled = true; return nil }
	// The binary's build commit is NOT verifiable against HEAD (tampered /
	// stripped / built from uncommitted source).
	chainBoundaryRepinProvenanceFn = func(string) (string, phaseintegrity.ProvenanceVerified) {
		return "cafebabe1234", func(string) bool { return false }
	}

	var stderr bytes.Buffer
	refreshed := maybeRefreshChainBoundary(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, 1, &stderr)

	if refreshed {
		t.Error("an unverified build commit must never report refreshed=true")
	}
	if reexecCalled {
		t.Error("an unverified build commit must never reach re-exec")
	}
	if got := brhReadPin(t, evolveDir); got != "STALE_PIN" {
		t.Errorf("forged provenance: pin moved to %q despite an UNVERIFIED build commit — the anti-tamper control was bypassed", got)
	}
	if _, err := os.Stat(filepath.Join(evolveDir, chainBoundaryRefreshLogFile)); err == nil {
		t.Error("a refused refresh must not write a boundary-refresh authorization record")
	}
	if !strings.Contains(stderr.String(), "boundary-refresh") {
		t.Errorf("a refused refresh must say so on stderr (fail loudly): %s", stderr.String())
	}
}

// TestDefaultChainBoundaryRepinProvenance_RejectsNonAncestorAndEmptyCommits:
// the shipped default provenance closure is a real git-ancestor check, not a
// constant — an arbitrary commit is rejected, an empty commit is rejected,
// and only a real ancestor of HEAD is accepted.
func TestDefaultChainBoundaryRepinProvenance_RejectsNonAncestorAndEmptyCommits(t *testing.T) {
	dir, commitA := brfInitRepo(t)
	brfAdvance(t, dir)

	_, prov := defaultChainBoundaryRepinProvenance(dir)
	if prov == nil {
		t.Fatal("the default boundary-refresh provenance must supply a verification closure")
	}
	if prov("") {
		t.Error("an empty build commit is unverifiable and must NEVER be provenance-verified")
	}
	if prov("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef") {
		t.Error("a commit that is not in this repo must NEVER be provenance-verified — that is the forged-provenance class")
	}
	if prov("boundary-refresh") {
		t.Error("the dead 'boundary-refresh' sentinel must NEVER be accepted as a build commit")
	}
	if !prov(commitA) {
		t.Errorf("a real ancestor of HEAD (%s) must be provenance-verified", commitA)
	}
}

// TestMaybeRefreshChainBoundary_NeverSubstitutesSentinelForEmptyCommit: an
// unstamped binary (empty build commit) must never have the literal
// "boundary-refresh" laundered through the provenance gate in its place.
func TestMaybeRefreshChainBoundary_NeverSubstitutesSentinelForEmptyCommit(t *testing.T) {
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")

	restore := brfStubSeams(t, true, nil, nil)
	defer restore()

	prevRebuild, prevReExec := chainRebuildFn, chainReExecFn
	prevCommit, prevProv := chainRunningCommitFn, chainBoundaryRepinProvenanceFn
	defer func() {
		chainRebuildFn, chainReExecFn = prevRebuild, prevReExec
		chainRunningCommitFn, chainBoundaryRepinProvenanceFn = prevCommit, prevProv
	}()

	chainRunningCommitFn = func() string { return "" }
	chainRebuildFn = func(string) error { return nil }
	chainReExecFn = func(string, []string, []string) error { return nil }

	var asked []string
	chainBoundaryRepinProvenanceFn = func(string) (string, phaseintegrity.ProvenanceVerified) {
		return "", func(c string) bool { asked = append(asked, c); return true }
	}

	var stderr bytes.Buffer
	maybeRefreshChainBoundary(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, 1, &stderr)

	for _, c := range asked {
		if c == "boundary-refresh" {
			t.Fatalf("the dead sentinel 'boundary-refresh' was substituted for an empty build commit and sent to the provenance gate: %v", asked)
		}
	}
	if got := brhReadPin(t, evolveDir); got != "STALE_PIN" {
		t.Errorf("an unstamped binary (empty build commit) must not be able to move the ship pin; pin=%q", got)
	}
}

// TestMaybeRefreshChainBoundary_PinsShaOfRebuiltBinaryNotRunningExecutable:
// the rebuild writes <root>/go/bin/evolve, and the repin must hash that
// file, not the running executable.
func TestMaybeRefreshChainBoundary_PinsShaOfRebuiltBinaryNotRunningExecutable(t *testing.T) {
	root, evolveDir, wantSHA := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")

	restore := brfStubSeams(t, true, nil, nil)
	defer restore()

	prevRebuild, prevReExec := chainRebuildFn, chainReExecFn
	prevCommit, prevProv := chainRunningCommitFn, chainBoundaryRepinProvenanceFn
	defer func() {
		chainRebuildFn, chainReExecFn = prevRebuild, prevReExec
		chainRunningCommitFn, chainBoundaryRepinProvenanceFn = prevCommit, prevProv
	}()

	chainRunningCommitFn = func() string { return "cafebabe1234" }
	chainRebuildFn = func(string) error { return nil }
	chainReExecFn = func(string, []string, []string) error { return nil }
	chainBoundaryRepinProvenanceFn = func(string) (string, phaseintegrity.ProvenanceVerified) {
		return "cafebabe1234", func(c string) bool { return c == "cafebabe1234" }
	}

	var stderr bytes.Buffer
	if !maybeRefreshChainBoundary(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, 1, &stderr) {
		t.Fatalf("a verified refresh must report refreshed=true; stderr=%s", stderr.String())
	}

	got := brhReadPin(t, evolveDir)
	if got != wantSHA {
		t.Errorf("expected_ship_sha must be sha256(<root>/go/bin/evolve) = %s, got %s — the repin hashed the wrong binary", wantSHA, got)
	}
}

// TestMaybeRefreshChainBoundary_ReExecTargetsRebuiltBinaryNotArgv0: the whole
// point of the refresh is to come back on the new binary, so argv0 must be
// the <root>/go/bin/evolve the rebuild just wrote, not
// exec.LookPath(os.Args[0]) (the running, stale image).
func TestMaybeRefreshChainBoundary_ReExecTargetsRebuiltBinaryNotArgv0(t *testing.T) {
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")

	restore := brfStubSeams(t, true, nil, nil)
	defer restore()

	prevRebuild, prevReExec, prevArgv := chainRebuildFn, chainReExecFn, chainReExecArgvFn
	prevCommit, prevProv := chainRunningCommitFn, chainBoundaryRepinProvenanceFn
	defer func() {
		chainRebuildFn, chainReExecFn, chainReExecArgvFn = prevRebuild, prevReExec, prevArgv
		chainRunningCommitFn, chainBoundaryRepinProvenanceFn = prevCommit, prevProv
	}()

	chainRunningCommitFn = func() string { return "cafebabe1234" }
	chainRebuildFn = func(string) error { return nil }
	chainReExecArgvFn = func() []string { return []string{"evolve", "loop", "--chain"} }
	chainBoundaryRepinProvenanceFn = func(string) (string, phaseintegrity.ProvenanceVerified) {
		return "cafebabe1234", func(string) bool { return true }
	}

	var gotArgv0 string
	var gotArgv []string
	chainReExecFn = func(argv0 string, argv, _ []string) error {
		gotArgv0, gotArgv = argv0, argv
		return nil
	}

	var stderr bytes.Buffer
	if !maybeRefreshChainBoundary(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, 1, &stderr) {
		t.Fatalf("a verified refresh must report refreshed=true; stderr=%s", stderr.String())
	}

	wantArgv0 := filepath.Join(root, "go", "bin", "evolve")
	if gotArgv0 != wantArgv0 {
		t.Errorf("re-exec must target the REBUILT binary %q, got %q — the refresh can land back on the stale image", wantArgv0, gotArgv0)
	}
	if len(gotArgv) == 0 || gotArgv[len(gotArgv)-1] != "--chain" {
		t.Errorf("the original chain argv must be preserved across the re-exec, got %v", gotArgv)
	}
}

// TestMaybeRefreshChainBoundary_MissingRebuiltBinaryDegradesToNoRefresh: an
// absent rebuilt binary means there is nothing safe to re-exec into —
// degrade to no refresh rather than exec'ing an unknown path.
func TestMaybeRefreshChainBoundary_MissingRebuiltBinaryDegradesToNoRefresh(t *testing.T) {
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")
	if err := os.Remove(filepath.Join(root, "go", "bin", "evolve")); err != nil {
		t.Fatal(err)
	}

	restore := brfStubSeams(t, true, nil, nil)
	defer restore()

	prevRebuild, prevReExec := chainRebuildFn, chainReExecFn
	prevCommit, prevProv := chainRunningCommitFn, chainBoundaryRepinProvenanceFn
	defer func() {
		chainRebuildFn, chainReExecFn = prevRebuild, prevReExec
		chainRunningCommitFn, chainBoundaryRepinProvenanceFn = prevCommit, prevProv
	}()

	chainRunningCommitFn = func() string { return "cafebabe1234" }
	chainRebuildFn = func(string) error { return nil } // "succeeds" but produces nothing
	reexecCalled := false
	chainReExecFn = func(string, []string, []string) error { reexecCalled = true; return nil }
	chainBoundaryRepinProvenanceFn = func(string) (string, phaseintegrity.ProvenanceVerified) {
		return "cafebabe1234", func(string) bool { return true }
	}

	var stderr bytes.Buffer
	refreshed := maybeRefreshChainBoundary(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, 1, &stderr)

	if refreshed || reexecCalled {
		t.Errorf("an absent rebuilt binary must degrade to no refresh (refreshed=%v reexec=%v)", refreshed, reexecCalled)
	}
	if got := brhReadPin(t, evolveDir); got != "STALE_PIN" {
		t.Errorf("an absent rebuilt binary must leave the pin untouched, got %q", got)
	}
}

// TestMaybeRefreshChainBoundary_SecondAttemptSameCommitIsRefusedLoopBreaker:
// two refresh attempts carrying the same running build commit mean the
// previous re-exec came back on a binary that had not moved, so the second
// attempt must be refused — the marker persists on disk because a real
// re-exec destroys any in-process counter.
func TestMaybeRefreshChainBoundary_SecondAttemptSameCommitIsRefusedLoopBreaker(t *testing.T) {
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")

	restore := brfStubSeams(t, true, nil, nil)
	defer restore()

	prevRebuild, prevReExec := chainRebuildFn, chainReExecFn
	prevCommit, prevProv := chainRunningCommitFn, chainBoundaryRepinProvenanceFn
	defer func() {
		chainRebuildFn, chainReExecFn = prevRebuild, prevReExec
		chainRunningCommitFn, chainBoundaryRepinProvenanceFn = prevCommit, prevProv
	}()

	rebuilds, reexecs := 0, 0
	chainRunningCommitFn = func() string { return "cafebabe1234" } // never moves
	chainRebuildFn = func(string) error { rebuilds++; return nil }
	chainReExecFn = func(string, []string, []string) error { reexecs++; return nil }
	chainBoundaryRepinProvenanceFn = func(string) (string, phaseintegrity.ProvenanceVerified) {
		return "cafebabe1234", func(string) bool { return true }
	}

	cfg := loopConfig{ProjectRoot: root, EvolveDir: evolveDir}
	var stderr bytes.Buffer

	if !maybeRefreshChainBoundary(cfg, 1, &stderr) {
		t.Fatalf("the FIRST refresh for a new build commit must proceed; stderr=%s", stderr.String())
	}
	// Simulates the process that came back from the re-exec still reporting the
	// same build commit — the livelock signature.
	if maybeRefreshChainBoundary(cfg, 2, &stderr) {
		t.Error("a SECOND refresh for the same running build commit must be refused — that is an unbounded re-exec livelock")
	}
	if reexecs != 1 {
		t.Errorf("the loop breaker must cap re-execs for one unchanged build commit at 1, got %d", reexecs)
	}
	if rebuilds > 1 {
		t.Errorf("a refused refresh must not rebuild again, got %d rebuilds", rebuilds)
	}
	if _, err := os.Stat(filepath.Join(evolveDir, chainBoundaryRefreshAttemptFile)); err != nil {
		t.Errorf("the loop breaker must persist its marker to %s (a re-exec destroys in-process state): %v", chainBoundaryRefreshAttemptFile, err)
	}
	if !strings.Contains(stderr.String(), "boundary-refresh") {
		t.Errorf("a refused refresh must be logged (fail loudly): %s", stderr.String())
	}
}

// TestMaybeRefreshChainBoundary_LoopBreakerRearmsWhenCommitMoves: the
// breaker is per-commit, not a permanent kill switch — once the running
// commit actually moves, a refresh is allowed again.
func TestMaybeRefreshChainBoundary_LoopBreakerRearmsWhenCommitMoves(t *testing.T) {
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")

	restore := brfStubSeams(t, true, nil, nil)
	defer restore()

	prevRebuild, prevReExec := chainRebuildFn, chainReExecFn
	prevCommit, prevProv := chainRunningCommitFn, chainBoundaryRepinProvenanceFn
	defer func() {
		chainRebuildFn, chainReExecFn = prevRebuild, prevReExec
		chainRunningCommitFn, chainBoundaryRepinProvenanceFn = prevCommit, prevProv
	}()

	commit := "cafebabe1234"
	chainRunningCommitFn = func() string { return commit }
	chainRebuildFn = func(string) error { return nil }
	chainReExecFn = func(string, []string, []string) error { return nil }
	chainBoundaryRepinProvenanceFn = func(string) (string, phaseintegrity.ProvenanceVerified) {
		return commit, func(string) bool { return true }
	}

	cfg := loopConfig{ProjectRoot: root, EvolveDir: evolveDir}
	var stderr bytes.Buffer

	if !maybeRefreshChainBoundary(cfg, 1, &stderr) {
		t.Fatalf("first refresh must proceed; stderr=%s", stderr.String())
	}
	// The re-exec worked: the process is now running a binary built from a NEW
	// commit, and HEAD has since advanced again.
	commit = "0ddba11beef0"
	if err := os.WriteFile(filepath.Join(root, "go", "bin", "evolve"), []byte("REBUILT-AGAIN"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !maybeRefreshChainBoundary(cfg, 2, &stderr) {
		t.Errorf("the loop breaker must re-arm once the running build commit actually moves; stderr=%s", stderr.String())
	}
}

// TestRunLoopChain_LoopBreakerLetsBatchesRunAfterAFruitlessReExec drives
// runLoopChain, not the helper: with a permanently-true ahead-check, the
// first chain run refreshes and stops for the re-exec, and the process that
// comes back (a second runLoopChain over the same .evolve dir, still on the
// same build commit) must run real batches instead of refreshing again.
func TestRunLoopChain_LoopBreakerLetsBatchesRunAfterAFruitlessReExec(t *testing.T) {
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")
	inboxDir := filepath.Join(evolveDir, "inbox")
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		brfWriteJSON(t, filepath.Join(inboxDir, "item-"+string(rune('a'+i))+".json"),
			map[string]any{"id": "item", "title": "t", "weight": 0.5})
	}

	restore := brfStubSeams(t, true, nil, nil)
	defer restore()

	prevRebuild, prevReExec, prevBatch := chainRebuildFn, chainReExecFn, runLoopBatchFn
	prevCommit, prevProv := chainRunningCommitFn, chainBoundaryRepinProvenanceFn
	defer func() {
		chainRebuildFn, chainReExecFn, runLoopBatchFn = prevRebuild, prevReExec, prevBatch
		chainRunningCommitFn, chainBoundaryRepinProvenanceFn = prevCommit, prevProv
	}()

	chainRunningCommitFn = func() string { return "cafebabe1234" } // the rebuild never moves it
	chainRebuildFn = func(string) error { return nil }
	chainReExecFn = func(string, []string, []string) error { return nil }
	chainBoundaryRepinProvenanceFn = func(string) (string, phaseintegrity.ProvenanceVerified) {
		return "cafebabe1234", func(string) bool { return true }
	}

	batches := 0
	runLoopBatchFn = func(loopConfig, io.Reader, io.Writer, io.Writer) int { batches++; return 0 }

	cfg := loopConfig{ProjectRoot: root, EvolveDir: evolveDir}
	cc := policy.ChainConfig{MaxBatches: 2}

	var out1, err1 bytes.Buffer
	runLoopChain(cfg, cc, strings.NewReader(""), &out1, &err1)
	// The process that came back from the re-exec.
	var out2, err2 bytes.Buffer
	runLoopChain(cfg, cc, strings.NewReader(""), &out2, &err2)

	if batches == 0 {
		t.Fatalf("a fruitless re-exec must not brick the chain — zero batches ran across two chain runs\nrun1=%s\nrun2=%s", err1.String(), err2.String())
	}
}
