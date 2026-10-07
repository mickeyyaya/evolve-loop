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

// TestBootGate_HaltsOnWithinVersionSelfShaMismatch: a within-version ship-SHA
// mismatch must halt boot pre-scout with the operator recipe, and must not
// auto-repin — the pin is left untouched so the mismatch is not silently healed.
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
	// The message must carry the operator-unblock recipe verbatim so a human
	// can act without hunting for it.
	msg := stderr.String()
	for _, want := range []string{"make -C go build", "evolve reset-sha -operator"} {
		if !strings.Contains(msg, want) {
			t.Errorf("halt message must contain the operator recipe %q; got:\n%s", want, msg)
		}
	}
}

// TestBootGate_AcrossVersionMismatchStillAutoRepins: an across-version
// mismatch (a legitimate plugin/version bump) must stay on the existing boot
// auto-repin path unchanged — it heals and re-pins the SHA, and must not trip
// the within-version halt.
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

// TestBootGate_MatchingSHABootsIntoScout: when the on-disk binary's SHA
// matches expected_ship_sha under the same plugin version, boot must take
// zero self-SHA action and fall through to scout.
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

// TestRunLoop_HaltsPreScoutOnWithinVersionSelfShaMismatch: runLoop, given a
// within-version mismatch, must halt during boot before the readiness gate —
// no cycle, no scout, no LLM budget. The readiness-gate seam
// (runLoopPreflightFn) must never be invoked; exit is 2 with a distinct
// StopReason, and the operator recipe reaches stderr.
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
	wireOrchestratorDepsFn = func(string, string, io.Writer, routingRun) orchDeps {
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
