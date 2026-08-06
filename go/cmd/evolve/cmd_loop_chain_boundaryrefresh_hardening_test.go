package main

// cmd_loop_chain_boundaryrefresh_hardening_test.go — RED tests (cycle 1323,
// continuation of cycle 1320, inbox item auto-refresh-binary-at-boundary).
//
// Cycle 1320 landed the boundary-refresh sequence (ahead-check -> rebuild ->
// repin -> ledger -> re-exec) but its audit FAILed with 9 defects, 2 of which
// (D1 short-sha ahead-check, D2 short-sha test gap) are already fixed in this
// tree. The remaining OPEN defects are what THIS file encodes:
//
//	dd8a8d64 / dcaf44e4 (CRIT, same defect) — maybeRefreshChainBoundary passes
//	  trivialProvenance = func(string) bool { return true } into
//	  phaseintegrity.RepinShipSHA, stubbing out the ProvenanceVerified
//	  anti-tamper control and stamping Authorized="provenance" without ever
//	  verifying anything. ADR-0072 forged-verdict class.
//	d7542cf6 (MAJOR) — the repin hashes the RUNNING executable via
//	  selfsha.Running() instead of the rebuilt go/bin/evolve the rebuild just
//	  produced, so it pins the STALE hash under the forged label.
//	ddb8f717 (CRIT) — rebuild writes go/bin/evolve but the re-exec targets
//	  exec.LookPath(os.Args[0]); nothing guarantees the process that comes back
//	  is the binary that was just built.
//	de8b9e49 / df20cf48 (CRIT) — no re-exec loop breaker: any ahead-check false
//	  positive (or a rebuild that does not move the binary's build commit)
//	  becomes an unbounded rebuild -> repin -> re-exec livelock in which zero
//	  batches ever run, because the refresh check precedes chainStartDecision.
//	d9d245d4 — dead fallback: repinCommit = "boundary-refresh" on an empty
//	  running commit is unreachable AND would launder an unverifiable commit
//	  past a real provenance gate if it ever were reached.
//
// Contract the Builder implements. Three NEW package-var seams, mirroring the
// established postBuildRepinProvenanceFn / chainRebuildFn idiom, plus a
// re-shaped repin that reuses the SHARED primitive post_build_repin.go already
// uses instead of hand-rolling a second repin path:
//
//	// chainRunningCommitFn resolves the running binary's build commit.
//	// Production = version.Commit. A seam so the loop breaker's
//	// same-commit-twice behaviour is deterministic under test.
//	var chainRunningCommitFn = version.Commit
//
//	// chainBoundaryRepinProvenanceFn mirrors core.defaultPostBuildRepinProvenance:
//	// it returns the running binary's build commit AND a REAL
//	// phaseintegrity.ProvenanceVerified closure asserting that commit is an
//	// ancestor of HEAD (`git merge-base --is-ancestor <c> HEAD`). An empty
//	// commit is unverifiable and MUST return false. There is no sentinel
//	// substitute for an empty commit — an unstamped binary simply does not get
//	// to self-authorize a re-pin.
//	var chainBoundaryRepinProvenanceFn = defaultChainBoundaryRepinProvenance
//	func defaultChainBoundaryRepinProvenance(projectRoot string) (string, phaseintegrity.ProvenanceVerified)
//
//	// chainReExecTargetFn resolves the executable the boundary refresh re-execs
//	// into: the REBUILT <projectRoot>/go/bin/evolve, never os.Args[0]. An
//	// absent/non-executable target is an error and degrades to no refresh.
//	var chainReExecTargetFn = defaultChainReExecTarget
//	func defaultChainReExecTarget(projectRoot string) (string, error)
//
//	// chainBoundaryRefreshAttemptFile is the on-disk loop breaker. A refresh
//	// records the running commit that triggered it; a LATER refresh attempt
//	// carrying that SAME running commit means the previous re-exec came back on
//	// a binary that had not moved — it is refused (WARN, refreshed=false) so the
//	// chain degrades to running batches on the current binary instead of
//	// livelocking. On-disk because a re-exec destroys any in-process counter.
//	var chainBoundaryRefreshAttemptFile = "boundary-refresh-attempt.json"
//
// The repin itself becomes phaseintegrity.RepinIfDrifted(statePath,
// <projectRoot>/go/bin/evolve, commit, "", prov) — the SAME shared
// detect-drift + provenance-gate + repin path core.repinShipSHAAfterBuild uses,
// which fixes the forged-provenance and wrong-hash defects together and deletes
// the second hand-rolled repin path (never_duplicate_centralize).

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

// --- AC1 (dd8a8d64 / dcaf44e4): the provenance gate is REAL, not stubbed ---

// AC1 negative — the load-bearing anti-forgery assertion. When provenance says
// "I cannot verify this build commit", the boundary refresh MUST refuse: the
// ship pin stays untouched, nothing is ledgered as authorized, no re-exec
// happens, and the chain degrades to running on the current binary. Today the
// production path hands RepinShipSHA an always-true stub, so the pin moves and
// this test fails.
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

// AC1 positive/production-default — the shipped default provenance closure is
// a real git-ancestor check, not a constant. This is what makes AC1's seam
// meaningful in production: an arbitrary commit is rejected, an empty commit is
// rejected, and only a real ancestor of HEAD is accepted.
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

// AC1 edge (d9d245d4) — the dead sentinel fallback must be gone: an unstamped
// binary (empty build commit) must never have the literal "boundary-refresh"
// laundered through the provenance gate in its place.
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

// --- AC2 (d7542cf6): the pin is the sha of the REBUILT binary ---

// The rebuild writes <root>/go/bin/evolve; the repin must hash THAT file, not
// the running test executable. Today selfsha.Running() hashes the test binary,
// so the pin can never equal sha256(go/bin/evolve).
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

// --- AC3 (ddb8f717): re-exec targets the rebuilt binary ---

// The whole point of the refresh is to come back on the NEW binary. argv0 must
// be the <root>/go/bin/evolve the rebuild just wrote, not
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

// AC3 edge — an absent rebuilt binary means there is nothing safe to re-exec
// into: degrade to no refresh rather than exec'ing an unknown path.
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

// --- AC4 (de8b9e49 / df20cf48): the re-exec loop breaker ---

// Two refresh attempts carrying the SAME running build commit mean the
// previous re-exec came back on a binary that had not moved. The second attempt
// MUST be refused so the chain degrades to running batches on the current
// binary instead of livelocking. The marker is on disk because a real re-exec
// destroys any in-process counter.
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

// AC4 positive — the breaker is per-commit, not a permanent kill switch: once
// the running commit actually moves (the rebuild worked), a refresh is allowed
// again.
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

// AC4 reachability (production caller) — drives runLoopChain, not the helper.
// With a permanently-true ahead-check, the FIRST chain run refreshes and stops
// for the re-exec, and the process that comes back (a second runLoopChain over
// the SAME .evolve dir, still on the same build commit) must run real batches
// instead of refreshing again. Today, with no breaker, the second run refreshes
// too and zero batches ever execute — the bricked chain of df20cf48.
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
