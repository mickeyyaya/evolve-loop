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

func bgWritePluginVersion(t *testing.T, repo, ver string) {
	t.Helper()
	dir := filepath.Join(repo, ".claude-plugin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	brWriteJSON(t, filepath.Join(dir, "plugin.json"), map[string]any{"version": ver})
}

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
	brWriteJSON(t, filepath.Join(evolveDir, "state.json"), map[string]any{
		"expected_ship_sha":     pin,
		"expected_ship_version": ver,
	})

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
	msg := stderr.String()
	for _, want := range []string{"make -C go build", "evolve reset-sha -operator"} {
		if !strings.Contains(msg, want) {
			t.Errorf("halt message must contain the operator recipe %q; got:\n%s", want, msg)
		}
	}
}

func TestBootGate_AcrossVersionMismatchStillAutoRepins(t *testing.T) {
	repo := brInitRepo(t)
	evolveDir := filepath.Join(repo, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	bgWritePluginVersion(t, repo, "9.9.9-test")
	bgSeedBinary(t, repo, "\x7fELF-legitimately-rebuilt-new-version")
	brWriteJSON(t, filepath.Join(evolveDir, "state.json"), map[string]any{
		"expected_ship_sha":     "stale-pin-from-the-old-version",
		"expected_ship_version": "1.0.0-previous",
	})

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

func TestBootGate_MatchingSHABootsIntoScout(t *testing.T) {
	t.Parallel()
	repo := brInitRepo(t)
	evolveDir := filepath.Join(repo, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const ver = "9.9.9-test"
	bgWritePluginVersion(t, repo, ver)
	const binContent = "\x7fELF-healthy-matching-binary"
	bgSeedBinary(t, repo, binContent)
	binPath := filepath.Join(repo, "go", "bin", "evolve")
	_, actual, err := core.ShipSHAMismatch(binPath, "")
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
