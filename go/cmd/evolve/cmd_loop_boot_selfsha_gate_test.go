package main

// cmd_loop_boot_selfsha_gate_test.go — RED tests (cycle 639, task
// self-sha-fail-early-boot-gate; inbox 2026-07-08T03-05-30Z, weight 0.95).
//
// Problem this cycle closes: a WITHIN-VERSION ship-binary SHA mismatch
// (state.json:expected_ship_version == the current plugin version, but
// expected_ship_sha != the on-disk go/bin/evolve) is a BOOT-time-knowable,
// cycle-FATAL condition — it is exactly what the terminal ship gate calls
// SELF_SHA_TAMPERED (internal/phases/ship/verify.go:119-127, the "same version,
// different SHA" INTEGRITY-FAIL branch). Yet boot only WARNs and proceeds: 8
// consecutive cycles (625-634) each burned a full ~32-40 min scout→…→ship lane
// before dying at that terminal gate on a ship structurally doomed from boot.
//
// The fix classifies the mismatch at boot the SAME way verifySelfSHA does:
//
//   - within-version (expected_ship_version present AND == pluginVersion(root))
//     + SHA differs  → HALT pre-scout with the operator-unblock recipe
//     (`make -C go build` → `evolve reset-sha -operator` → relaunch). Do NOT
//     start scout. The auto-repin is NOT attempted — a within-version SHA change
//     is tampering/corruption by the ship-gate's own definition, not a legitimate
//     rebuild (a legit rebuild is version-bumped, or healed by the post-build
//     repin of cycle 636).
//   - across-version / legacy-unversioned mismatch → the EXISTING boot auto-repin
//     path (cycle 514, phaseintegrity.RepinIfDrifted), byte-for-byte unchanged.
//   - matching SHA → no mismatch, no halt; boot proceeds into scout.
//
// Contract the Builder implements (TDD-defined seam; extends the established
// bootRecoverFn / shipRepinProvenanceFn package-var seam idiom):
//
//	type bootRecoveryResult struct {
//	    Quarantined, Sealed, SHAMismatch, Healed bool
//	    HaltSelfSHA bool // NEW: a within-version SHA mismatch — boot must HALT pre-scout
//	}
//	// defaultBootRecovery: on a detected SHA mismatch, read expected_ship_version
//	// and compare to pluginVersion(cfg.ProjectRoot). If both non-empty AND equal
//	// (within-version), set res.HaltSelfSHA=true, print the operator-unblock recipe
//	// to stderr, and return WITHOUT attempting the auto-repin. Otherwise
//	// (across-version / legacy) keep today's detect→auto-repin behavior verbatim.
//	//
//	// runLoop, immediately after bootRecoverFn returns (BEFORE the unfinished-cycle
//	// guard and readiness gate — hence pre-scout), checks res.HaltSelfSHA and, if
//	// set, sets lr.StopReason="self_sha_boot_halt", emits, and returns 2. No scout
//	// phase, no readiness gate, no LLM budget spent.
//
// RED now (HaltSelfSHA field undefined → package main test build fails). Do NOT
// modify this file — implement the seam.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// bgWritePluginVersion writes <repo>/.claude-plugin/plugin.json:version=ver so
// pluginVersion(repo) — the SAME resolver the ship gate uses — returns ver. This
// is how a test pins "the current plugin version" deterministically without a
// link-time stamp.
func bgWritePluginVersion(t *testing.T, repo, ver string) {
	t.Helper()
	dir := filepath.Join(repo, ".claude-plugin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	brWriteJSON(t, filepath.Join(dir, "plugin.json"), map[string]any{"version": ver})
}

// bgSeedBinary writes an on-disk go/bin/evolve whose bytes hash to something
// other than any pinned expected_ship_sha (callers pin a bogus string).
func bgSeedBinary(t *testing.T, repo string, content string) {
	t.Helper()
	binPath := filepath.Join(repo, "go", "bin", "evolve")
	if err := os.MkdirAll(filepath.Dir(binPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binPath, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

// AC1 (headline, positive + negative-of-repin): a WITHIN-VERSION ship-SHA
// mismatch must HALT boot pre-scout with the operator recipe, and must NOT
// auto-repin (the pin is left untouched so the mismatch is not silently
// "healed"). This is the anti-no-op signal for the whole cycle: a "fix" that
// repins the within-version case away, or that lets boot proceed, fails here.
func TestBootGate_HaltsOnWithinVersionSelfShaMismatch(t *testing.T) {
	repo := brInitRepo(t)
	evolveDir := filepath.Join(repo, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const ver = "9.9.9-test"
	bgWritePluginVersion(t, repo, ver)
	bgSeedBinary(t, repo, "\x7fELF-on-disk-binary-within-version")
	const pin = "within-version-stale-or-tampered-sha"
	// within-version: expected_ship_version == pluginVersion(repo), SHA differs.
	brWriteJSON(t, filepath.Join(evolveDir, "state.json"), map[string]any{
		"expected_ship_sha":     pin,
		"expected_ship_version": ver,
	})

	// A provenance resolver that WOULD authorize a repin if consulted — proving
	// the within-version halt path deliberately does NOT auto-repin even when the
	// binary looks provenance-verified. If the halt path wrongly fell through to
	// the repin, res.Healed would go true and this test would catch it.
	prev := shipRepinProvenanceFn
	defer func() { shipRepinProvenanceFn = prev }()
	shipRepinProvenanceFn = func(string) (string, phaseintegrity.ProvenanceVerified) {
		return "would-verify", func(string) bool { return true }
	}

	var stderr bytes.Buffer
	res := bootRecoverFn(context.Background(),
		loopConfig{ProjectRoot: repo, EvolveDir: evolveDir}, newFakeLedger(), &stderr)

	if !res.HaltSelfSHA {
		t.Fatalf("a within-version ship-SHA mismatch must set HaltSelfSHA (boot must halt pre-scout); res=%+v stderr=%q", res, stderr.String())
	}
	if res.Healed {
		t.Errorf("a within-version mismatch must NOT be auto-repinned — it is tampering/corruption, not a legit rebuild; res=%+v", res)
	}
	if got := readExpectedShipSHA(t, evolveDir); got != pin {
		t.Errorf("expected_ship_sha must be UNTOUCHED on a within-version halt; got %q want %q", got, pin)
	}
	// The message must carry the operator-unblock recipe verbatim so a human can
	// act without hunting for it (the 8-cycle waste was partly not-knowing-what-to-do).
	msg := stderr.String()
	for _, want := range []string{"make -C go build", "evolve reset-sha -operator"} {
		if !strings.Contains(msg, want) {
			t.Errorf("halt message must contain the operator recipe %q; got:\n%s", want, msg)
		}
	}
}

// AC2 (regression twin): an ACROSS-VERSION mismatch (expected_ship_version !=
// the current plugin version — a legitimate plugin/version bump) must stay on the
// EXISTING boot auto-repin path unchanged: it heals (res.Healed) and re-pins the
// SHA, and it must NOT trip the new within-version halt.
func TestBootGate_AcrossVersionMismatchStillAutoRepins(t *testing.T) {
	repo := brInitRepo(t)
	evolveDir := filepath.Join(repo, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	bgWritePluginVersion(t, repo, "9.9.9-test")
	bgSeedBinary(t, repo, "\x7fELF-legitimately-rebuilt-new-version")
	// across-version: the pinned version differs from the current plugin version.
	brWriteJSON(t, filepath.Join(evolveDir, "state.json"), map[string]any{
		"expected_ship_sha":     "stale-pin-from-the-old-version",
		"expected_ship_version": "1.0.0-previous",
	})

	// Provenance VERIFIED (legit rebuild) — the existing repin path fires.
	prev := shipRepinProvenanceFn
	defer func() { shipRepinProvenanceFn = prev }()
	shipRepinProvenanceFn = func(string) (string, phaseintegrity.ProvenanceVerified) {
		return "verified-build-commit", func(string) bool { return true }
	}

	var stderr bytes.Buffer
	res := bootRecoverFn(context.Background(),
		loopConfig{ProjectRoot: repo, EvolveDir: evolveDir}, newFakeLedger(), &stderr)

	if res.HaltSelfSHA {
		t.Errorf("an across-version mismatch must NOT halt boot — it is a legit version bump; res=%+v stderr=%q", res, stderr.String())
	}
	if !res.Healed {
		t.Fatalf("an across-version mismatch with verified provenance must auto-repin (existing behavior unchanged); res=%+v stderr=%q", res, stderr.String())
	}
	// The pin now equals the on-disk binary sha: the existing repin actually fired.
	newExpected := readExpectedShipSHA(t, evolveDir)
	binPath := filepath.Join(repo, "go", "bin", "evolve")
	mismatch, _, err := core.ShipSHAMismatch(binPath, newExpected)
	if err != nil {
		t.Fatal(err)
	}
	if mismatch {
		t.Errorf("after the across-version auto-repin, expected_ship_sha (%q) must match the on-disk binary", newExpected)
	}
}

// AC3 (negative / matching SHA): when the on-disk binary's SHA MATCHES
// expected_ship_sha under the same plugin version, there is no mismatch — boot
// must take ZERO self-SHA action (no halt, no flag, no heal) and fall through to
// scout. This guards against a gate that halts spuriously on a healthy tree.
func TestBootGate_MatchingSHABootsIntoScout(t *testing.T) {
	repo := brInitRepo(t)
	evolveDir := filepath.Join(repo, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const ver = "9.9.9-test"
	bgWritePluginVersion(t, repo, ver)
	const binContent = "\x7fELF-healthy-matching-binary"
	bgSeedBinary(t, repo, binContent)
	// Pin expected_ship_sha to the ACTUAL on-disk sha — a healthy, matched tree.
	binPath := filepath.Join(repo, "go", "bin", "evolve")
	_, actual, err := core.ShipSHAMismatch(binPath, "") // "" != actual, returns the real sha
	if err != nil {
		t.Fatal(err)
	}
	brWriteJSON(t, filepath.Join(evolveDir, "state.json"), map[string]any{
		"expected_ship_sha":     actual,
		"expected_ship_version": ver,
	})

	var stderr bytes.Buffer
	res := bootRecoverFn(context.Background(),
		loopConfig{ProjectRoot: repo, EvolveDir: evolveDir}, newFakeLedger(), &stderr)

	if res.HaltSelfSHA {
		t.Errorf("a matching SHA must NOT halt boot; res=%+v stderr=%q", res, stderr.String())
	}
	if res.SHAMismatch || res.Healed {
		t.Errorf("a matching SHA must trigger zero self-SHA action; res=%+v", res)
	}
}

// AC1 (headline, integration — "no scout phase runs"): runLoop, given a
// within-version mismatch, must HALT during boot BEFORE the readiness gate — so
// no cycle, no scout, no LLM budget. Proof of "pre-scout": the readiness-gate
// seam (runLoopPreflightFn), which runs strictly AFTER the boot self-heal and
// strictly BEFORE any cycle dispatch, is NEVER invoked. The exit is 2 with the
// distinct StopReason, and the operator recipe reaches stderr.
func TestRunLoop_HaltsPreScoutOnWithinVersionSelfShaMismatch(t *testing.T) {
	repo := brInitRepo(t)
	evolveDir := filepath.Join(repo, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const ver = "9.9.9-test"
	bgWritePluginVersion(t, repo, ver)
	bgSeedBinary(t, repo, "\x7fELF-within-version-mismatch")
	brWriteJSON(t, filepath.Join(evolveDir, "state.json"), map[string]any{
		"expected_ship_sha":     "within-version-mismatch-sha",
		"expected_ship_version": ver,
	})

	prevDeps := wireOrchestratorDepsFn
	defer func() { wireOrchestratorDepsFn = prevDeps }()
	wireOrchestratorDepsFn = func(string, string) orchDeps {
		return orchDeps{Storage: &fixtures.FakeStorage{}, Ledger: newFakeLedger()}
	}

	// The readiness gate is the sentinel: if it is ever reached, boot did NOT
	// halt pre-scout. It also force-halts as a backstop so no real cycle runs even
	// if the contract regresses.
	preflightCalled := false
	prevPf := runLoopPreflightFn
	defer func() { runLoopPreflightFn = prevPf }()
	runLoopPreflightFn = func(loopConfig, io.Writer) looppreflight.Result {
		preflightCalled = true
		return forcedHalt()
	}

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", repo,
		"--evolve-dir", evolveDir,
		"--goal-text", "anything",
		"--cycles", "1",
		"--force-fresh",
	}, nil, &stdout, &stderr)

	if rc != 2 {
		t.Fatalf("rc=%d want 2 (self-SHA boot halt); stderr=%q", rc, stderr.String())
	}
	if preflightCalled {
		t.Error("boot must HALT before the readiness gate (pre-scout) — runLoopPreflightFn was reached, so a within-version mismatch did NOT halt early")
	}
	var out struct {
		StopReason string `json:"stop_reason"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal loop result: %v\nstdout=%q", err, stdout.String())
	}
	if out.StopReason != "self_sha_boot_halt" {
		t.Errorf("StopReason=%q want %q", out.StopReason, "self_sha_boot_halt")
	}
	msg := stderr.String()
	for _, want := range []string{"make -C go build", "evolve reset-sha -operator"} {
		if !strings.Contains(msg, want) {
			t.Errorf("halt stderr must contain the operator recipe %q; got:\n%s", want, msg)
		}
	}
}
